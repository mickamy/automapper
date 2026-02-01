// Package analyzer provides AST analysis for struct types and converter registrations.
package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Analyzer loads and analyzes Go packages.
type Analyzer struct {
	pkgs map[string]*packages.Package
}

// New creates a new Analyzer.
func New() *Analyzer {
	return &Analyzer{
		pkgs: make(map[string]*packages.Package),
	}
}

// LoadPackage loads a package by its import path or directory.
func (a *Analyzer) LoadPackage(pattern string) (*packages.Package, error) {
	if pkg, ok := a.pkgs[pattern]; ok {
		return pkg, nil
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps,
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("load package %s: %w", pattern, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found for pattern %s", pattern)
	}

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("package %s has errors: %v", pattern, pkg.Errors)
	}

	a.pkgs[pattern] = pkg

	return pkg, nil
}

// FindStruct finds a struct type by name in a package.
func (a *Analyzer) FindStruct(pkg *packages.Package, name string) (*StructInfo, error) {
	obj := pkg.Types.Scope().Lookup(name)
	if obj == nil {
		return nil, fmt.Errorf("type %s not found in package %s", name, pkg.PkgPath)
	}

	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("%s is not a named type", name)
	}

	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("%s is not a struct type", name)
	}

	return a.extractStructInfo(pkg, name, structType)
}

// extractStructInfo extracts field information from a struct type.
func (a *Analyzer) extractStructInfo(pkg *packages.Package, name string, structType *types.Struct) (*StructInfo, error) {
	info := &StructInfo{
		Name:    name,
		PkgPath: pkg.PkgPath,
		PkgName: pkg.Name,
		Fields:  make([]FieldInfo, 0, structType.NumFields()),
	}

	// Get AST for tag extraction
	var astStruct *ast.StructType
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if ts.Name.Name == name {
				if st, ok := ts.Type.(*ast.StructType); ok {
					astStruct = st

					return false
				}
			}

			return true
		})
		if astStruct != nil {
			break
		}
	}

	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if !field.Exported() {
			continue
		}

		fieldInfo := FieldInfo{
			Name:     field.Name(),
			Type:     field.Type(),
			TypeStr:  types.TypeString(field.Type(), nil),
			Exported: field.Exported(),
		}

		// Extract tag from AST
		if astStruct != nil && i < len(astStruct.Fields.List) {
			// Need to find the correct AST field by name
			for _, astField := range astStruct.Fields.List {
				for _, ident := range astField.Names {
					if ident.Name == field.Name() && astField.Tag != nil {
						fieldInfo.Tag = astField.Tag.Value
					}
				}
			}
		}

		info.Fields = append(info.Fields, fieldInfo)
	}

	return info, nil
}

// ResolveType resolves a type string like "userpb.User" to its full information.
func (a *Analyzer) ResolveType(typeStr string, basePkg *packages.Package) (*StructInfo, error) {
	// Check if it's a qualified name (pkg.Type)
	pkgName, typeName := splitTypeName(typeStr)

	if pkgName == "" {
		// Type is in the base package
		return a.FindStruct(basePkg, typeName)
	}

	// Find the import for this package name
	var importPath string
	for path, imp := range basePkg.Imports {
		if imp.Name == pkgName {
			importPath = path

			break
		}
	}

	if importPath == "" {
		// Try loading by package name directly (for local packages)
		return nil, fmt.Errorf("import %s not found in package %s", pkgName, basePkg.PkgPath)
	}

	pkg, err := a.LoadPackage(importPath)
	if err != nil {
		return nil, fmt.Errorf("load import %s: %w", importPath, err)
	}

	return a.FindStruct(pkg, typeName)
}

// splitTypeName splits "pkg.Type" into ("pkg", "Type") or ("", "Type") for unqualified types.
func splitTypeName(s string) (pkg, name string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[:i], s[i+1:]
		}
	}

	return "", s
}
