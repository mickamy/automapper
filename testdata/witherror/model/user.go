// Package model contains domain types.
package model

import "time"

// User represents a domain user with birthdate.
type User struct {
	ID        int64
	Name      string
	Email     string
	BirthDate time.Time // Converts to/from string with potential parse error
}
