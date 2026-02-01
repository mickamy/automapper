// Package registry manages converter registrations for code generation.
package registry

import (
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mickamy/automapper/internal/analyzer"
)

// TypePair represents a source-target type pair.
type TypePair struct {
	Source string // Fully qualified source type
	Target string // Fully qualified target type
}

// Converter holds information about a registered converter.
type Converter struct {
	// FuncName is the function name (e.g., "ToDate")
	FuncName string
	// PkgPath is the package where the function is defined
	PkgPath string
	// HasError indicates if the converter returns an error
	HasError bool
}

// QualifiedName returns the fully qualified function name.
func (c Converter) QualifiedName(currentPkg string) string {
	if c.PkgPath == "" || c.PkgPath == currentPkg {
		return c.FuncName
	}
	// Need to import and use qualified name
	return c.PkgPath + "." + c.FuncName
}

// Registry manages type converters.
type Registry struct {
	// converters maps type pairs to their converters
	converters map[TypePair]Converter
	// namedConverters maps names to their converters
	namedConverters map[string]Converter
	// generatedMappers tracks which type pairs have generated mappers
	generatedMappers map[TypePair]string // TypePair -> function name
}

// New creates a new Registry.
func New() *Registry {
	return &Registry{
		converters:       make(map[TypePair]Converter),
		namedConverters:  make(map[string]Converter),
		generatedMappers: make(map[TypePair]string),
	}
}

// Register adds a converter to the registry.
func (r *Registry) Register(source, target string, conv Converter) {
	pair := TypePair{Source: source, Target: target}
	r.converters[pair] = conv
}

// RegisterNamed adds a named converter to the registry.
func (r *Registry) RegisterNamed(name string, conv Converter) {
	r.namedConverters[name] = conv
}

// Lookup finds a converter for a type pair.
func (r *Registry) Lookup(source, target string) (Converter, bool) {
	pair := TypePair{Source: source, Target: target}
	conv, ok := r.converters[pair]

	return conv, ok
}

// LookupNamed finds a named converter.
func (r *Registry) LookupNamed(name string) (Converter, bool) {
	conv, ok := r.namedConverters[name]

	return conv, ok
}

// MarkGenerated marks a type pair as having a generated mapper.
func (r *Registry) MarkGenerated(source, target, funcName string) {
	pair := TypePair{Source: source, Target: target}
	r.generatedMappers[pair] = funcName
}

// LookupGenerated finds a generated mapper for a type pair.
func (r *Registry) LookupGenerated(source, target string) (string, bool) {
	pair := TypePair{Source: source, Target: target}
	fn, ok := r.generatedMappers[pair]

	return fn, ok
}

// LoadFromConverterInfos populates the registry from discovered converters.
func (r *Registry) LoadFromConverterInfos(infos []analyzer.ConverterInfo, pkg *packages.Package) {
	for _, info := range infos {
		conv := Converter{
			FuncName: info.FuncName,
			PkgPath:  info.FuncPkg,
			HasError: info.HasError,
		}

		// Determine source and target based on direction
		source := info.SourceType
		target := info.TargetType

		if info.Direction == "from" {
			// RegisterFrom[A, B](fn func(B) A) means B -> A
			source = info.TargetType
			target = info.SourceType
		}

		// Normalize type names to fully qualified form
		source = normalizeTypeKey(source, pkg)
		target = normalizeTypeKey(target, pkg)

		if info.Name != "" {
			r.RegisterNamed(info.Name, conv)
		} else {
			r.Register(source, target, conv)
		}
	}
}

// normalizeTypeKey normalizes a type key to fully qualified form.
func normalizeTypeKey(typeStr string, pkg *packages.Package) string {
	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		inner := strings.TrimPrefix(typeStr, "*")

		return "*" + normalizeTypeKey(inner, pkg)
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		inner := strings.TrimPrefix(typeStr, "[]")

		return "[]" + normalizeTypeKey(inner, pkg)
	}

	// Check if it's a builtin type
	if isBuiltinType(typeStr) {
		return typeStr
	}

	// Check if already qualified (contains a dot)
	if strings.Contains(typeStr, ".") {
		// Resolve package alias to full path
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

	// Unqualified type - qualify with current package
	return pkg.PkgPath + "." + typeStr
}

// isBuiltinType checks if a type is a Go builtin type.
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

// All returns all registered type pairs.
func (r *Registry) All() map[TypePair]Converter {
	return r.converters
}

// AllNamed returns all named converters.
func (r *Registry) AllNamed() map[string]Converter {
	return r.namedConverters
}
