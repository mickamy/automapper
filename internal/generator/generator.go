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

// Generator produces mapper code.
type Generator struct {
	registry   *registry.Registry
	outputPkg  string
	outputPath string
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
	Imports     []string
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

	importSet := make(map[string]bool)

	for _, m := range mappings {
		funcs, fileImports := g.generateFunctions(m, direction)
		data.Functions = append(data.Functions, funcs...)
		for _, imp := range fileImports {
			importSet[imp] = true
		}
	}

	// Convert import set to sorted slice
	for imp := range importSet {
		data.Imports = append(data.Imports, imp)
	}
	sort.Strings(data.Imports)

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
	var funcs []FunctionData
	var allImports []string

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

	return info.PkgName + "." + info.Name
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
	pkgName := path.Base(conv.PkgPath)

	return pkgName + "." + conv.FuncName
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

		return pkg.Name() + "." + v.Obj().Name()
	case *types.Basic:
		return v.Name()
	default:
		return types.TypeString(t, func(p *types.Package) string {
			if p == nil {
				return ""
			}
			*imports = append(*imports, p.Path())

			return p.Name()
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
