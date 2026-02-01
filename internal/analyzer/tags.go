package analyzer

import (
	"reflect"
	"strings"
)

// MapTag represents a parsed map:"..." struct tag.
type MapTag struct {
	// Ignore indicates map:"-" was specified
	Ignore bool
	// TargetName is the target field name if specified (e.g., map:"TargetName")
	TargetName string
	// Converter is the named converter to use (e.g., map:",conv=name")
	Converter string
}

// ParseMapTag parses the map:"..." tag from a raw struct tag string.
// The raw tag includes backticks, e.g., `json:"name" map:"TargetName,conv=foo"`.
func ParseMapTag(rawTag string) MapTag {
	if rawTag == "" {
		return MapTag{}
	}

	// Remove backticks
	tag := strings.Trim(rawTag, "`")

	// Use reflect.StructTag to parse
	st := reflect.StructTag(tag)
	value, ok := st.Lookup("map")
	if !ok {
		return MapTag{}
	}

	return parseMapTagValue(value)
}

// parseMapTagValue parses the value part of a map tag.
// Examples:
//   - "-" -> Ignore
//   - "TargetName" -> TargetName
//   - "TargetName,conv=foo" -> TargetName + Converter
//   - ",conv=foo" -> Converter only
func parseMapTagValue(value string) MapTag {
	if value == "-" {
		return MapTag{Ignore: true}
	}

	tag := MapTag{}
	parts := strings.Split(value, ",")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if i == 0 && !strings.Contains(part, "=") {
			// First part without = is the target name
			tag.TargetName = part

			continue
		}

		// Parse key=value options
		if strings.HasPrefix(part, "conv=") {
			tag.Converter = strings.TrimPrefix(part, "conv=")
		}
	}

	return tag
}
