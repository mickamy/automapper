// Package pb contains protobuf-like types.
package pb

// OrderPB represents a protobuf order message.
type OrderPB struct {
	OrderId     int64 //nolint:revive // protobuf naming convention
	ProductName string
	Quantity    int32
	CreatedAt   int64 // Unix timestamp
	Status      int32
	Items       []*OrderItemPB
}

// OrderItemPB represents a protobuf order item.
type OrderItemPB struct {
	Sku      string
	Name     string
	Quantity int32
	Price    float64
}

// ProductPB represents a protobuf product message.
type ProductPB struct {
	Id    int64 //nolint:revive // protobuf naming convention
	Name  string
	Price int64 // Price in cents
}
