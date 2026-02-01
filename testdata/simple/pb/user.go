// Package pb contains protobuf-like types.
package pb

// UserPB represents a protobuf user message.
type UserPB struct {
	Id      int64
	Name    string
	Email   string
	Active  bool
	Tags    []string
	Address *AddressPB
}

// AddressPB represents a protobuf address message.
type AddressPB struct {
	Street  string
	City    string
	Country string
}
