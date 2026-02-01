package resolver

import (
	"fmt"
	"strings"
)

// Error represents a resolution error.
type Error struct {
	Field   string
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("field %s: %s", e.Field, e.Message)
}

// Errors is a collection of resolution errors.
type Errors []Error

func (e Errors) Error() string {
	if len(e) == 0 {
		return ""
	}

	msgs := make([]string, 0, len(e))
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}

	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any errors.
func (e Errors) HasErrors() bool {
	return len(e) > 0
}

// Add adds an error to the collection.
func (e *Errors) Add(field, message string) {
	*e = append(*e, Error{Field: field, Message: message})
}
