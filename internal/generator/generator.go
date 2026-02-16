// Package generator produces Go source code for struct mappers.
package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"go/types"
	"path"
	"sort"
	"strings"

	"golang.org/x/tools/imports"

	"github.com/mickamy/automapper/internal/analyzer"
	"github.com/mickamy/automapper/internal/registry"
	"github.com/mickamy/automapper/internal/resolver"
)

// ImportEntry represents a single import with an optional alias.
type ImportEntry struct {
	Path  string
	Alias string // empty if no alias needed
}

// Generator produces mapper code.
type Generator struct {
	registry      *registry.Registry
	outputPkg     string
	outputPath    string
	qualifiers    map[string]string // pkgPath -> unique qualifier
	declaredNames map[string]string // pkgPath -> declared package name
}

// New creates a new Generator.
func New(reg *registry.Registry, outputPkg, outputPath string) *Generator {
	return &Generator{
		registry:   reg,
		outputPkg:  outputPkg,
		outputPath: outputPath,
	}
}

// FileData holds data for generating a complete file.
type FileData struct {
	PackageName string
	Imports     []ImportEntry
	Functions   []FunctionData
}

// FunctionData holds data for generating a single function.
type FunctionData struct {
	Name            string
	SourceType      string
	TargetType      string
	SourceQualified string
	TargetQualified string
	IsPointer       bool
	ReturnsPointer  bool
	HasError        bool
	Fields          []FieldData
}

// FieldData holds data for a single field mapping.
type FieldData struct {
	SourceField   string
	TargetField   string
	Kind          string // "direct", "converter", "nested", "slice"
	ConverterCall string
	NestedCall    string
	SliceCall     string
	HasError      bool
	VarName       string
	NilOrZero     string // "nil" for pointer returns, "Type{}" for value returns
}

// GenerateFile generates a complete mapper file for the given mappings.
func (g *Generator) GenerateFile(mappings []*resolver.Mapping, direction string) ([]byte, error) {
	data := FileData{
		PackageName: g.outputPkg,
	}

	// Build qualifier map before generating functions so all references
	// use consistent, collision-free package qualifiers.
	g.qualifiers, g.declaredNames = g.buildQualifiers(mappings)

	importSet := make(map[string]bool)

	for _, m := range mappings {
		funcs, fileImports := g.generateFunctions(m, direction)
		data.Functions = append(data.Functions, funcs...)
		for _, imp := range fileImports {
			importSet[imp] = true
		}
	}

	// Convert import set to sorted ImportEntry slice
	for imp := range importSet {
		entry := ImportEntry{Path: imp}
		if alias, ok := g.qualifiers[imp]; ok {
			declared := g.declaredNames[imp]
			if alias != declared {
				entry.Alias = alias
			}
		}
		data.Imports = append(data.Imports, entry)
	}
	sort.Slice(data.Imports, func(i, j int) bool {
		return data.Imports[i].Path < data.Imports[j].Path
	})

	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "file", data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Format with goimports
	formatted, err := imports.Process(g.outputPath, buf.Bytes(), nil)
	if err != nil {
		// Try basic formatting if imports fails
		formatted, err = format.Source(buf.Bytes())
		if err != nil {
			// Return raw bytes for debugging
			return buf.Bytes(), fmt.Errorf("format source: %w\n\nRaw output:\n%s", err, buf.String())
		}
	}

	return formatted, nil
}

// generateFunctions generates function data for a mapping.
func (g *Generator) generateFunctions(m *resolver.Mapping, direction string) ([]FunctionData, []string) {
	funcs := make([]FunctionData, 0, 2)
	allImports := make([]string, 0, 2)

	// Generate function name
	baseName := g.functionName(m.Source, m.Target, direction)

	// Generate pointer version
	ptrFunc, ptrImports := g.generateFunction(m, baseName+"Ptr", true, true, direction)
	funcs = append(funcs, ptrFunc)
	allImports = append(allImports, ptrImports...)

	// Generate value version
	valFunc, valImports := g.generateFunction(m, baseName, false, false, direction)
	funcs = append(funcs, valFunc)
	allImports = append(allImports, valImports...)

	return funcs, allImports
}

// generateFunction generates a single function.
func (g *Generator) generateFunction(m *resolver.Mapping, name string, isPointer, returnsPointer bool, direction string) (FunctionData, []string) {
	var imports []string

	sourceQual := g.qualifiedType(m.Source, &imports)
	targetQual := g.qualifiedType(m.Target, &imports)

	fd := FunctionData{
		Name:            name,
		SourceType:      m.Source.Name,
		TargetType:      m.Target.Name,
		SourceQualified: sourceQual,
		TargetQualified: targetQual,
		IsPointer:       isPointer,
		ReturnsPointer:  returnsPointer,
		HasError:        m.HasError,
	}

	if m.HasError {
		imports = append(imports, "fmt")
	}

	varIdx := 0
	for _, fm := range m.Fields {
		field := g.generateField(fm, &imports, &varIdx, m.Target, returnsPointer, direction)
		fd.Fields = append(fd.Fields, field)
	}

	return fd, imports
}

// generateField generates field data for a field mapping.
func (g *Generator) generateField(fm resolver.FieldMapping, imports *[]string, varIdx *int, target *analyzer.StructInfo, returnsPointer bool, direction string) FieldData {
	fd := FieldData{
		SourceField: fm.SourceField,
		TargetField: fm.TargetField,
		HasError:    fm.HasError,
	}

	switch fm.Kind {
	case resolver.KindDirect:
		fd.Kind = "direct"

	case resolver.KindConverter:
		fd.Kind = "converter"
		fd.ConverterCall = g.converterCall(fm.Converter, imports)
		if fm.HasError {
			fd.VarName = fmt.Sprintf("conv%d", *varIdx)
			*varIdx++
			fd.NilOrZero = g.nilOrZero(target, returnsPointer)
		}

	case resolver.KindNested:
		fd.Kind = "nested"
		fd.NestedCall = g.nestedCall(fm, imports, direction)

	case resolver.KindSlice:
		fd.Kind = "slice"
		fd.SliceCall = g.sliceCall(fm, imports, direction)
	}

	return fd
}

// qualifiedType returns the qualified type name and adds imports.
func (g *Generator) qualifiedType(info *analyzer.StructInfo, imports *[]string) string {
	if info.PkgPath == "" || info.PkgPath == g.outputPkg {
		return info.Name
	}

	*imports = append(*imports, info.PkgPath)

	qualifier := info.PkgName
	if q, ok := g.qualifiers[info.PkgPath]; ok {
		qualifier = q
	}

	return qualifier + "." + info.Name
}

// functionName generates a function name based on direction.
func (g *Generator) functionName(source, target *analyzer.StructInfo, direction string) string {
	if direction == "to" {
		// ToTargetType (e.g., ToUserPB)
		return "To" + g.shortTypeName(target)
	}
	// FromTargetType (e.g., FromUserPB)
	return "From" + g.shortTypeName(source)
}

// shortTypeName returns a short name for a struct type.
func (g *Generator) shortTypeName(info *analyzer.StructInfo) string {
	// If same package, just use type name
	if info.PkgPath == "" || path.Base(info.PkgPath) == g.outputPkg {
		return info.Name
	}
	// Otherwise include package name in CamelCase
	pkgName := path.Base(info.PkgPath)

	return strings.ToUpper(pkgName[:1]) + pkgName[1:] + info.Name
}

// converterCall generates a converter function call.
func (g *Generator) converterCall(conv *registry.Converter, imports *[]string) string {
	if conv.PkgPath == "" || conv.PkgPath == g.outputPkg {
		return conv.FuncName
	}
	*imports = append(*imports, conv.PkgPath)

	qualifier := path.Base(conv.PkgPath)
	if q, ok := g.qualifiers[conv.PkgPath]; ok {
		qualifier = q
	}

	return qualifier + "." + conv.FuncName
}

// nestedCall generates a nested mapper call.
func (g *Generator) nestedCall(fm resolver.FieldMapping, _ *[]string, direction string) string {
	if fm.NestedMapper != "" {
		return fm.NestedMapper + "(src." + fm.SourceField + ")"
	}

	// Generate inline or use ToXxx/FromXxx convention
	sourceIsPtr := analyzer.IsPointer(fm.SourceType)
	targetIsPtr := analyzer.IsPointer(fm.TargetType)

	targetName := analyzer.TypeName(fm.TargetType)
	targetPkg := analyzer.TypePkgPath(fm.TargetType)

	// For "from" direction, we need to reference the source type name from "from" perspective
	// which is actually the target type in the mapping
	var funcName string
	if direction == "to" {
		funcName = "To" + targetName
		if targetPkg != "" && targetPkg != g.outputPkg {
			pkgName := path.Base(targetPkg)
			funcName = "To" + strings.ToUpper(pkgName[:1]) + pkgName[1:] + targetName
		}
	} else {
		// "from" direction - use the source type package for naming
		sourceName := analyzer.TypeName(fm.SourceType)
		sourcePkg := analyzer.TypePkgPath(fm.SourceType)
		funcName = "From" + sourceName
		if sourcePkg != "" && sourcePkg != g.outputPkg {
			pkgName := path.Base(sourcePkg)
			funcName = "From" + strings.ToUpper(pkgName[:1]) + pkgName[1:] + sourceName
		}
	}

	switch {
	case sourceIsPtr && targetIsPtr:
		return funcName + "Ptr(src." + fm.SourceField + ")"
	case sourceIsPtr:
		return funcName + "(*src." + fm.SourceField + ")"
	case targetIsPtr:
		return funcName + "Ptr(&src." + fm.SourceField + ")"
	default:
		return funcName + "(src." + fm.SourceField + ")"
	}
}

// sliceCall generates a slice mapping call.
func (g *Generator) sliceCall(fm resolver.FieldMapping, imports *[]string, direction string) string {
	sourceElem := analyzer.SliceElem(fm.SourceType)
	targetElem := analyzer.SliceElem(fm.TargetType)

	sourceIsPtr := analyzer.IsPointer(sourceElem)
	targetIsPtr := analyzer.IsPointer(targetElem)

	targetName := analyzer.TypeName(targetElem)
	targetPkg := analyzer.TypePkgPath(targetElem)

	var funcName string
	switch {
	case fm.Converter != nil:
		funcName = g.converterCall(fm.Converter, imports)
	case fm.NestedMapper != "":
		funcName = fm.NestedMapper
	default:
		if direction == "to" {
			funcName = "To" + targetName
			if targetPkg != "" && targetPkg != g.outputPkg {
				pkgName := path.Base(targetPkg)
				funcName = "To" + strings.ToUpper(pkgName[:1]) + pkgName[1:] + targetName
			}
		} else {
			// "from" direction
			sourceName := analyzer.TypeName(sourceElem)
			sourcePkg := analyzer.TypePkgPath(sourceElem)
			funcName = "From" + sourceName
			if sourcePkg != "" && sourcePkg != g.outputPkg {
				pkgName := path.Base(sourcePkg)
				funcName = "From" + strings.ToUpper(pkgName[:1]) + pkgName[1:] + sourceName
			}
		}
		if sourceIsPtr && targetIsPtr {
			funcName += "Ptr"
		}
	}

	// Generate slice mapping inline
	targetTypeStr := g.typeString(targetElem, imports)

	var elemConvert string
	switch {
	case sourceIsPtr && !targetIsPtr:
		elemConvert = funcName + "(*v)"
	case !sourceIsPtr && targetIsPtr:
		elemConvert = funcName + "Ptr(&v)"
	default:
		elemConvert = funcName + "(v)"
	}

	return fmt.Sprintf("func() []%s {\n\t\tif src.%s == nil {\n\t\t\treturn nil\n\t\t}\n\t\tresult := make([]%s, len(src.%s))\n\t\tfor i, v := range src.%s {\n\t\t\tresult[i] = %s\n\t\t}\n\t\treturn result\n\t}()",
		targetTypeStr, fm.SourceField, targetTypeStr, fm.SourceField, fm.SourceField, elemConvert)
}

// typeString returns a Go type string with short package qualifiers.
func (g *Generator) typeString(t interface{}, imports *[]string) string {
	switch v := t.(type) {
	case *analyzer.StructInfo:
		return g.qualifiedType(v, imports)
	case types.Type:
		return g.typesTypeString(v, imports)
	default:
		// Fallback
		return fmt.Sprintf("%v", t)
	}
}

// typesTypeString converts a types.Type to a string with short package qualifiers.
func (g *Generator) typesTypeString(t types.Type, imports *[]string) string {
	switch v := t.(type) {
	case *types.Pointer:
		return "*" + g.typesTypeString(v.Elem(), imports)
	case *types.Slice:
		return "[]" + g.typesTypeString(v.Elem(), imports)
	case *types.Named:
		pkg := v.Obj().Pkg()
		if pkg == nil {
			return v.Obj().Name()
		}
		pkgPath := pkg.Path()
		if pkgPath == g.outputPkg || pkgPath == "" {
			return v.Obj().Name()
		}
		*imports = append(*imports, pkgPath)

		qualifier := pkg.Name()
		if q, ok := g.qualifiers[pkgPath]; ok {
			qualifier = q
		}

		return qualifier + "." + v.Obj().Name()
	case *types.Basic:
		return v.Name()
	default:
		return types.TypeString(t, func(p *types.Package) string {
			if p == nil {
				return ""
			}
			*imports = append(*imports, p.Path())

			qualifier := p.Name()
			if q, ok := g.qualifiers[p.Path()]; ok {
				qualifier = q
			}

			return qualifier
		})
	}
}

// nilOrZero returns "nil" for pointer returns or "Type{}" for value returns.
func (g *Generator) nilOrZero(info *analyzer.StructInfo, returnsPointer bool) string {
	if returnsPointer {
		return "nil"
	}

	return g.qualifiedType(info, &[]string{}) + "{}"
}

// buildQualifiers scans all mappings to collect referenced packages,
// detects name collisions, and assigns unique aliases.
// Returns (qualifiers, declaredNames) where qualifiers maps pkgPath to the
// qualifier to use in generated code, and declaredNames maps pkgPath to the
// package's declared name (from its package statement).
func (g *Generator) buildQualifiers(mappings []*resolver.Mapping) (map[string]string, map[string]string) {
	// pkgs maps pkgPath -> declared package name
	pkgs := make(map[string]string)

	for _, m := range mappings {
		g.registerPkg(pkgs, m.Source.PkgPath, m.Source.PkgName)
		g.registerPkg(pkgs, m.Target.PkgPath, m.Target.PkgName)
		for _, fm := range m.Fields {
			g.collectTypePkgs(fm.SourceType, pkgs)
			g.collectTypePkgs(fm.TargetType, pkgs)
			if fm.Converter != nil && fm.Converter.PkgPath != "" && fm.Converter.PkgPath != g.outputPkg {
				if _, exists := pkgs[fm.Converter.PkgPath]; !exists {
					pkgs[fm.Converter.PkgPath] = path.Base(fm.Converter.PkgPath)
				}
			}
		}
	}

	// Group by declared name to find collisions.
	// nameGroups maps declaredName -> []pkgPath
	nameGroups := make(map[string][]string)
	for pkgPath, name := range pkgs {
		nameGroups[name] = append(nameGroups[name], pkgPath)
	}

	// Build qualifier map. Only assign aliases for colliding names.
	qualifiers := make(map[string]string, len(pkgs))
	for name, paths := range nameGroups {
		if len(paths) == 1 {
			qualifiers[paths[0]] = name
			continue
		}
		// Collision: use parentDir + declaredName as alias.
		aliasCount := make(map[string]int)
		for _, pkgPath := range paths {
			parent := path.Base(path.Dir(pkgPath))
			alias := parent + name
			aliasCount[alias]++
		}
		// Assign aliases, appending numeric suffix if aliases still collide.
		usedAliases := make(map[string]int)
		for _, pkgPath := range paths {
			parent := path.Base(path.Dir(pkgPath))
			alias := parent + name
			if aliasCount[alias] > 1 {
				idx := usedAliases[alias]
				usedAliases[alias]++
				if idx > 0 {
					alias = fmt.Sprintf("%s%d", alias, idx)
				}
			}
			qualifiers[pkgPath] = alias
		}
	}

	return qualifiers, pkgs
}

// registerPkg adds a package to the map if it's external.
func (g *Generator) registerPkg(pkgs map[string]string, pkgPath, pkgName string) {
	if pkgPath == "" || pkgPath == g.outputPkg {
		return
	}
	if _, exists := pkgs[pkgPath]; !exists {
		pkgs[pkgPath] = pkgName
	}
}

// collectTypePkgs walks a types.Type tree to collect referenced packages.
func (g *Generator) collectTypePkgs(t types.Type, pkgs map[string]string) {
	if t == nil {
		return
	}
	switch v := t.(type) {
	case *types.Pointer:
		g.collectTypePkgs(v.Elem(), pkgs)
	case *types.Slice:
		g.collectTypePkgs(v.Elem(), pkgs)
	case *types.Named:
		pkg := v.Obj().Pkg()
		if pkg != nil {
			g.registerPkg(pkgs, pkg.Path(), pkg.Name())
		}
	}
}

