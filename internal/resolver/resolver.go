// Package resolver determines field mappings between struct types.
package resolver

import (
	"go/types"
	"strings"

	"github.com/mickamy/automapper/internal/analyzer"
	"github.com/mickamy/automapper/internal/registry"
)

// FieldMapping describes how to map a source field to a target field.
type FieldMapping struct {
	// SourceField is the source field name
	SourceField string
	// TargetField is the target field name
	TargetField string
	// SourceType is the source field type
	SourceType types.Type
	// TargetType is the target field type
	TargetType types.Type
	// Kind indicates the type of mapping
	Kind MappingKind
	// Converter is the converter to use (for KindConverter)
	Converter *registry.Converter
	// NestedMapper is the generated mapper function name (for KindNested)
	NestedMapper string
	// HasError indicates if this mapping may produce an error
	HasError bool
}

// MappingKind indicates the type of field mapping.
type MappingKind int

const (
	// KindDirect is a direct copy (same name, same type).
	KindDirect MappingKind = iota
	// KindConverter uses a registered converter.
	KindConverter
	// KindNested uses a generated nested mapper.
	KindNested
	// KindSlice maps slice elements.
	KindSlice
	// KindIgnore ignores the field (map:"-" or target-only field).
	KindIgnore
)

// Mapping holds the complete mapping between two struct types.
type Mapping struct {
	Source   *analyzer.StructInfo
	Target   *analyzer.StructInfo
	Fields   []FieldMapping
	HasError bool // true if any field mapping may produce an error
}

// Resolver resolves field mappings between struct types.
type Resolver struct {
	registry *registry.Registry
}

// New creates a new Resolver.
func New(reg *registry.Registry) *Resolver {
	return &Resolver{
		registry: reg,
	}
}

// Resolve determines the field mappings from source to target.
func (r *Resolver) Resolve(source, target *analyzer.StructInfo) (*Mapping, Errors) {
	mapping := &Mapping{
		Source: source,
		Target: target,
	}
	var errs Errors

	// Build a map of source fields by name
	sourceFields := make(map[string]analyzer.FieldInfo)
	for _, f := range source.Fields {
		sourceFields[f.Name] = f
	}

	// For each target field, find a mapping
	for _, targetField := range target.Fields {
		if !targetField.Exported {
			continue
		}

		fm, err := r.resolveField(targetField, sourceFields)
		if err != nil {
			errs.Add(targetField.Name, err.Message)

			continue
		}

		if fm.Kind == KindIgnore {
			continue
		}

		mapping.Fields = append(mapping.Fields, fm)
		if fm.HasError {
			mapping.HasError = true
		}
	}

	return mapping, errs
}

// resolveField resolves a single target field.
func (r *Resolver) resolveField(targetField analyzer.FieldInfo, sourceFields map[string]analyzer.FieldInfo) (FieldMapping, *Error) {
	// Check for map tag on target field
	tag := analyzer.ParseMapTag(targetField.Tag)

	// Rule 1: map:"-" -> ignore
	if tag.Ignore {
		return FieldMapping{Kind: KindIgnore}, nil
	}

	// Determine source field name
	sourceName := targetField.Name
	if tag.TargetName != "" {
		// When processing target fields, TargetName in tag means source field name
		sourceName = tag.TargetName
	}

	// Check source fields for tags that might rename to this target
	for _, sf := range sourceFields {
		sfTag := analyzer.ParseMapTag(sf.Tag)
		if sfTag.TargetName == targetField.Name {
			sourceName = sf.Name
			if sfTag.Converter != "" {
				tag.Converter = sfTag.Converter
			}

			break
		}
	}

	sourceField, hasSource := sourceFields[sourceName]

	// Try case-insensitive match if exact match not found
	if !hasSource {
		sourceField, hasSource = findFieldCaseInsensitive(sourceFields, sourceName)
	}

	// Rule 6: Target field has no source -> ignore (zero value)
	if !hasSource {
		return FieldMapping{Kind: KindIgnore}, nil
	}

	// Check for map tag on source field
	sourceTag := analyzer.ParseMapTag(sourceField.Tag)
	if sourceTag.Ignore {
		return FieldMapping{Kind: KindIgnore}, nil
	}

	// Rule 3: Named converter (source tag takes precedence for forward mapping)
	if sourceTag.Converter != "" || tag.Converter != "" {
		convName := sourceTag.Converter
		if convName == "" {
			convName = tag.Converter
		}
		conv, ok := r.registry.LookupNamed(convName)
		if !ok {
			return FieldMapping{}, &Error{Message: "named converter '" + convName + "' not found"}
		}

		return FieldMapping{
			SourceField: sourceField.Name,
			TargetField: targetField.Name,
			SourceType:  sourceField.Type,
			TargetType:  targetField.Type,
			Kind:        KindConverter,
			Converter:   &conv,
			HasError:    conv.HasError,
		}, nil
	}

	// Rule 4: Same name, same type -> direct copy
	if typesEqual(sourceField.Type, targetField.Type) {
		return FieldMapping{
			SourceField: sourceField.Name,
			TargetField: targetField.Name,
			SourceType:  sourceField.Type,
			TargetType:  targetField.Type,
			Kind:        KindDirect,
		}, nil
	}

	// Rule 5: Same name, different type -> look for converter
	sourceKey := analyzer.QualifiedTypeName(sourceField.Type)
	targetKey := analyzer.QualifiedTypeName(targetField.Type)

	conv, ok := r.registry.Lookup(sourceKey, targetKey)
	if ok {
		return FieldMapping{
			SourceField: sourceField.Name,
			TargetField: targetField.Name,
			SourceType:  sourceField.Type,
			TargetType:  targetField.Type,
			Kind:        KindConverter,
			Converter:   &conv,
			HasError:    conv.HasError,
		}, nil
	}

	// Check for slice of structs
	if analyzer.IsSlice(sourceField.Type) && analyzer.IsSlice(targetField.Type) {
		sourceElem := analyzer.SliceElem(sourceField.Type)
		targetElem := analyzer.SliceElem(targetField.Type)

		if analyzer.IsStruct(sourceElem) && analyzer.IsStruct(targetElem) {
			// Will use generated mapper for slice elements
			sourceElemKey := analyzer.QualifiedTypeName(sourceElem)
			targetElemKey := analyzer.QualifiedTypeName(targetElem)

			// Check for registered converter for element type
			elemConv, hasElemConv := r.registry.Lookup(sourceElemKey, targetElemKey)
			if hasElemConv {
				return FieldMapping{
					SourceField: sourceField.Name,
					TargetField: targetField.Name,
					SourceType:  sourceField.Type,
					TargetType:  targetField.Type,
					Kind:        KindSlice,
					Converter:   &elemConv,
					HasError:    elemConv.HasError,
				}, nil
			}

			// Check for generated mapper
			if fn, ok := r.registry.LookupGenerated(sourceElemKey, targetElemKey); ok {
				return FieldMapping{
					SourceField:  sourceField.Name,
					TargetField:  targetField.Name,
					SourceType:   sourceField.Type,
					TargetType:   targetField.Type,
					Kind:         KindSlice,
					NestedMapper: fn,
				}, nil
			}

			// Will need to generate a mapper for this
			return FieldMapping{
				SourceField: sourceField.Name,
				TargetField: targetField.Name,
				SourceType:  sourceField.Type,
				TargetType:  targetField.Type,
				Kind:        KindSlice,
			}, nil
		}
	}

	// Check for nested struct
	if analyzer.IsStruct(sourceField.Type) && analyzer.IsStruct(targetField.Type) {
		sourceDeref := analyzer.Dereference(sourceField.Type)
		targetDeref := analyzer.Dereference(targetField.Type)

		sourceElemKey := analyzer.QualifiedTypeName(sourceDeref)
		targetElemKey := analyzer.QualifiedTypeName(targetDeref)

		// Check for registered converter
		nestedConv, hasNestedConv := r.registry.Lookup(sourceElemKey, targetElemKey)
		if hasNestedConv {
			return FieldMapping{
				SourceField: sourceField.Name,
				TargetField: targetField.Name,
				SourceType:  sourceField.Type,
				TargetType:  targetField.Type,
				Kind:        KindConverter,
				Converter:   &nestedConv,
				HasError:    nestedConv.HasError,
			}, nil
		}

		// Check for generated mapper
		if fn, ok := r.registry.LookupGenerated(sourceElemKey, targetElemKey); ok {
			return FieldMapping{
				SourceField:  sourceField.Name,
				TargetField:  targetField.Name,
				SourceType:   sourceField.Type,
				TargetType:   targetField.Type,
				Kind:         KindNested,
				NestedMapper: fn,
			}, nil
		}

		// Will need to generate a mapper
		return FieldMapping{
			SourceField: sourceField.Name,
			TargetField: targetField.Name,
			SourceType:  sourceField.Type,
			TargetType:  targetField.Type,
			Kind:        KindNested,
		}, nil
	}

	// Rule 8: Cannot resolve
	return FieldMapping{}, &Error{Message: "cannot map " + sourceField.TypeStr + " to " + targetField.TypeStr}
}

// typesEqual compares two types for equality.
func typesEqual(a, b types.Type) bool {
	return types.Identical(a, b)
}

// findFieldCaseInsensitive finds a field by name with case-insensitive matching.
// This handles common naming conventions like ID <-> Id.
func findFieldCaseInsensitive(fields map[string]analyzer.FieldInfo, name string) (analyzer.FieldInfo, bool) {
	nameLower := strings.ToLower(name)
	for fieldName, field := range fields {
		if strings.ToLower(fieldName) == nameLower {
			return field, true
		}
	}

	return analyzer.FieldInfo{}, false
}
