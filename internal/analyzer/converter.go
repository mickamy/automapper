package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ConverterInfo holds information about a discovered converter registration.
type ConverterInfo struct {
	// SourceType is the source type (A in RegisterTo[A, B])
	SourceType string
	// TargetType is the target type (B in RegisterTo[A, B])
	TargetType string
	// FuncName is the qualified function name being registered
	FuncName string
	// FuncPkg is the package path where the function is defined
	FuncPkg string
	// HasError indicates if the converter returns an error
	HasError bool
	// Name is the converter name for named converters (empty for unnamed)
	Name string
	// Direction indicates "to" or "from"
	Direction string
}

// DiscoverConverters finds all automapper.Register* calls in a package.
func DiscoverConverters(pkg *packages.Package) ([]ConverterInfo, error) {
	var converters []ConverterInfo

	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			info, ok := parseRegisterCall(call, pkg)
			if ok {
				converters = append(converters, info)
			}

			return true
		})
	}

	return converters, nil
}

// parseRegisterCall parses a potential automapper.Register* call.
func parseRegisterCall(call *ast.CallExpr, pkg *packages.Package) (ConverterInfo, bool) {
	// Could be an indexed expression for generics: automapper.RegisterTo[A, B]
	idx, ok := call.Fun.(*ast.IndexListExpr)
	if !ok {
		return ConverterInfo{}, false
	}
	sel, ok := idx.X.(*ast.SelectorExpr)
	if !ok {
		return ConverterInfo{}, false
	}

	return parseRegisterCallWithTypes(call, idx, sel, pkg)
}

// parseRegisterCallWithTypes parses a Register call with explicit type parameters.
func parseRegisterCallWithTypes(call *ast.CallExpr, idx *ast.IndexListExpr, sel *ast.SelectorExpr, pkg *packages.Package) (ConverterInfo, bool) {
	// Check it's automapper package
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ConverterInfo{}, false
	}

	// Check if this identifier refers to the automapper package
	if !isAutomapperImport(ident.Name, pkg) {
		return ConverterInfo{}, false
	}

	funcName := sel.Sel.Name
	info := ConverterInfo{}

	// Parse function name to determine direction and error handling
	switch funcName {
	case "RegisterTo":
		info.Direction = "to"
		info.HasError = false
	case "RegisterFrom":
		info.Direction = "from"
		info.HasError = false
	case "RegisterToE":
		info.Direction = "to"
		info.HasError = true
	case "RegisterFromE":
		info.Direction = "from"
		info.HasError = true
	case "RegisterToNamed":
		info.Direction = "to"
		info.HasError = false
		info.Name = extractFirstStringArg(call)
	case "RegisterFromNamed":
		info.Direction = "from"
		info.HasError = false
		info.Name = extractFirstStringArg(call)
	case "RegisterToNamedE":
		info.Direction = "to"
		info.HasError = true
		info.Name = extractFirstStringArg(call)
	case "RegisterFromNamedE":
		info.Direction = "from"
		info.HasError = true
		info.Name = extractFirstStringArg(call)
	default:
		return ConverterInfo{}, false
	}

	// Extract type parameters
	if len(idx.Indices) != 2 {
		return ConverterInfo{}, false
	}

	// Prefer using type checker info to resolve type parameters, as it
	// correctly handles import aliases (e.g., cmodel "pkg/model").
	// Fall back to AST-based string extraction if type info is unavailable.
	info.SourceType = resolveTypeExpr(idx.Indices[0], pkg)
	info.TargetType = resolveTypeExpr(idx.Indices[1], pkg)

	// Extract the function being registered
	funcArgIdx := 0
	if info.Name != "" {
		funcArgIdx = 1 // Named converters have name as first arg
	}

	if len(call.Args) > funcArgIdx {
		info.FuncName, info.FuncPkg = extractFuncInfo(call.Args[funcArgIdx], pkg)
	}

	return info, true
}

// isAutomapperImport checks if an identifier refers to the automapper package.
func isAutomapperImport(name string, pkg *packages.Package) bool {
	for path, imp := range pkg.Imports {
		if strings.HasSuffix(path, "automapper") && imp.Name == name {
			return true
		}
		// Check if import is aliased
		if imp.Name == name && strings.HasSuffix(path, "automapper") {
			return true
		}
	}
	// Also check for direct import name
	for path := range pkg.Imports {
		if strings.HasSuffix(path, "automapper") {
			// Default import name is the last path component
			parts := strings.Split(path, "/")
			if parts[len(parts)-1] == name {
				return true
			}
		}
	}

	return false
}

// extractFirstStringArg extracts the first string literal argument from a call.
func extractFirstStringArg(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return ""
	}
	// Remove quotes
	return strings.Trim(lit.Value, `"`)
}

// resolveTypeExpr resolves a type expression to a fully qualified type string
// using the package's type checker info. Falls back to AST-based extraction.
func resolveTypeExpr(expr ast.Expr, pkg *packages.Package) string {
	if pkg.TypesInfo != nil {
		if tv, ok := pkg.TypesInfo.Types[expr]; ok {
			return QualifiedTypeName(tv.Type)
		}
	}

	return exprToTypeString(expr)
}

// exprToTypeString converts an AST expression to a type string.
func exprToTypeString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	case *ast.StarExpr:
		return "*" + exprToTypeString(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToTypeString(e.Elt)
		}
	}

	return ""
}

// extractFuncInfo extracts function name and package from an expression.
func extractFuncInfo(expr ast.Expr, pkg *packages.Package) (name, pkgPath string) {
	switch e := expr.(type) {
	case *ast.Ident:
		// Function in same package
		return e.Name, pkg.PkgPath
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			// Find the import path for this package name
			for path, imp := range pkg.Imports {
				if imp.Name == x.Name {
					return e.Sel.Name, path
				}
			}
			// Check for default import name
			for path := range pkg.Imports {
				parts := strings.Split(path, "/")
				if parts[len(parts)-1] == x.Name {
					return e.Sel.Name, path
				}
			}

			return x.Name + "." + e.Sel.Name, ""
		}
	}

	return "", ""
}

// TypeKeyFromTypes creates a type key string from types.Type for registry lookup.
func TypeKeyFromTypes(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		return p.Path()
	})
}

// NormalizeTypeKey normalizes a type key for consistent comparison.
func NormalizeTypeKey(typeStr string, pkg *packages.Package) string {
	// Handle simple type names by qualifying them
	if !strings.Contains(typeStr, ".") && !strings.HasPrefix(typeStr, "*") && !strings.HasPrefix(typeStr, "[]") {
		// Check if it's a builtin
		if isBuiltinType(typeStr) {
			return typeStr
		}
		// Qualify with current package
		return pkg.PkgPath + "." + typeStr
	}

	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		inner := strings.TrimPrefix(typeStr, "*")

		return "*" + NormalizeTypeKey(inner, pkg)
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		inner := strings.TrimPrefix(typeStr, "[]")

		return "[]" + NormalizeTypeKey(inner, pkg)
	}

	// Type is already qualified (pkg.Type)
	// Need to resolve package alias to full path
	parts := strings.SplitN(typeStr, ".", 2)
	if len(parts) != 2 {
		return typeStr
	}

	pkgAlias := parts[0]
	typeName := parts[1]

	// Find full import path
	for path, imp := range pkg.Imports {
		if imp.Name == pkgAlias {
			return path + "." + typeName
		}
	}

	// Check for default import name
	for path := range pkg.Imports {
		pathParts := strings.Split(path, "/")
		if pathParts[len(pathParts)-1] == pkgAlias {
			return path + "." + typeName
		}
	}

	return typeStr
}

func isBuiltinType(s string) bool {
	builtins := map[string]bool{
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true, "int": true,
		"int8": true, "int16": true, "int32": true, "int64": true,
		"rune": true, "string": true, "uint": true, "uint8": true,
		"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	}

	return builtins[s]
}

// QualifiedTypeName returns a fully qualified type name for a types.Type.
func QualifiedTypeName(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == nil {
			return ""
		}

		return p.Path()
	})
}

// ShortTypeName returns a short type name suitable for function naming.
func ShortTypeName(t types.Type) string {
	t = Dereference(t)
	if slice, ok := t.(*types.Slice); ok {
		return TypeName(slice.Elem()) + "s"
	}
	if named, ok := t.(*types.Named); ok {
		if pkg := named.Obj().Pkg(); pkg != nil {
			return pkg.Name() + named.Obj().Name()
		}

		return named.Obj().Name()
	}

	return fmt.Sprintf("%T", t)
}
