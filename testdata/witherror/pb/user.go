// Package pb contains protobuf-like types.
package pb

// UserPB represents a protobuf user message.
type UserPB struct {
	Id        int64  //nolint:revive // protobuf naming convention
	Name      string
	Email     string
	BirthDate string // Date as RFC3339 string
}
