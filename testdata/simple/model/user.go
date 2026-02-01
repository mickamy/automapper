// Package model contains domain types.
package model

import "time"

// User represents a domain user.
type User struct {
	ID        int64
	Name      string
	Email     string
	BirthDate time.Time
	Active    bool
	Tags      []string
	Address   *Address
}

// Address represents a mailing address.
type Address struct {
	Street  string
	City    string
	Country string
}
