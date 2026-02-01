package analyzer

import (
	"go/types"
)

// StructInfo holds information about a struct type.
type StructInfo struct {
	// Name is the type name (e.g., "User")
	Name string
	// PkgPath is the import path (e.g., "myproject/model")
	PkgPath string
	// PkgName is the package name (e.g., "model")
	PkgName string
	// Fields contains information about each exported field
	Fields []FieldInfo
}

// FieldInfo holds information about a struct field.
type FieldInfo struct {
	// Name is the field name
	Name string
	// Type is the field's Go type
	Type types.Type
	// TypeStr is the string representation of the type
	TypeStr string
	// Tag is the raw struct tag (including backticks)
	Tag string
	// Exported indicates if the field is exported
	Exported bool
}

// QualifiedName returns the fully qualified type name (e.g., "model.User").
func (s *StructInfo) QualifiedName() string {
	if s.PkgName == "" {
		return s.Name
	}

	return s.PkgName + "." + s.Name
}

// IsPointer returns true if the type is a pointer type.
func IsPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)

	return ok
}

// IsSlice returns true if the type is a slice type.
func IsSlice(t types.Type) bool {
	_, ok := t.(*types.Slice)

	return ok
}

// IsStruct returns true if the type is a struct (or pointer to struct).
func IsStruct(t types.Type) bool {
	t = Dereference(t)
	if named, ok := t.(*types.Named); ok {
		_, isStruct := named.Underlying().(*types.Struct)

		return isStruct
	}
	_, isStruct := t.(*types.Struct)

	return isStruct
}

// Dereference removes pointer indirection from a type.
func Dereference(t types.Type) types.Type {
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = ptr.Elem()
	}
}

// SliceElem returns the element type of a slice, or nil if not a slice.
func SliceElem(t types.Type) types.Type {
	if slice, ok := t.(*types.Slice); ok {
		return slice.Elem()
	}

	return nil
}

// TypeName returns the base type name for a type, stripping pointers and slices.
func TypeName(t types.Type) string {
	t = Dereference(t)
	if slice, ok := t.(*types.Slice); ok {
		t = Dereference(slice.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}

	return t.String()
}

// TypePkgPath returns the package path for a named type, or empty string.
func TypePkgPath(t types.Type) string {
	t = Dereference(t)
	if slice, ok := t.(*types.Slice); ok {
		t = Dereference(slice.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		if pkg := named.Obj().Pkg(); pkg != nil {
			return pkg.Path()
		}
	}

	return ""
}
