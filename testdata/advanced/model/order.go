// Package model contains domain types.
package model

import "time"

// Order represents a domain order.
type Order struct {
	ID          int64 `map:"OrderId"`
	CustomerID  int64 `map:"-"` // Ignored
	ProductName string
	Quantity    int32
	CreatedAt   time.Time
	Status      OrderStatus
	Items       []OrderItem
}

// OrderStatus represents the status of an order.
type OrderStatus int

const (
	OrderStatusPending OrderStatus = iota
	OrderStatusProcessing
	OrderStatusShipped
	OrderStatusDelivered
)

// OrderItem represents a line item in an order.
type OrderItem struct {
	SKU      string
	Name     string
	Quantity int32
	Price    float64
}

// Product represents a product with formatted price.
type Product struct {
	ID    int64
	Name  string
	Price string `map:",conv=priceToInt"` // Use named converter: string -> int64
}
