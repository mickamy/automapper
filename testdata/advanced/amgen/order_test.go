package amgen_test

import (
	"testing"
	"time"

	"github.com/mickamy/automapper/testdata/advanced/amgen"
	"github.com/mickamy/automapper/testdata/advanced/model"
	"github.com/mickamy/automapper/testdata/advanced/pb"
)

func TestToPbOrderPB(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	order := model.Order{
		ID:          123,
		CustomerID:  456, // Should be ignored
		ProductName: "Widget",
		Quantity:    5,
		CreatedAt:   createdAt,
		Status:      model.OrderStatusProcessing,
		Items: []model.OrderItem{
			{SKU: "SKU-001", Name: "Item 1", Quantity: 2, Price: 19.99},
			{SKU: "SKU-002", Name: "Item 2", Quantity: 3, Price: 29.99},
		},
	}

	result := amgen.ToPbOrderPB(order)

	if result.OrderId != order.ID {
		t.Errorf("OrderId: got %d, want %d", result.OrderId, order.ID)
	}
	if result.ProductName != order.ProductName {
		t.Errorf("ProductName: got %s, want %s", result.ProductName, order.ProductName)
	}
	if result.Quantity != order.Quantity {
		t.Errorf("Quantity: got %d, want %d", result.Quantity, order.Quantity)
	}
	if result.CreatedAt != createdAt.Unix() {
		t.Errorf("CreatedAt: got %d, want %d", result.CreatedAt, createdAt.Unix())
	}
	if result.Status != int32(model.OrderStatusProcessing) {
		t.Errorf("Status: got %d, want %d", result.Status, int32(model.OrderStatusProcessing))
	}
	if len(result.Items) != 2 {
		t.Fatalf("Items length: got %d, want 2", len(result.Items))
	}
	if result.Items[0].Sku != "SKU-001" {
		t.Errorf("Items[0].Sku: got %s, want SKU-001", result.Items[0].Sku)
	}
	if result.Items[1].Price != 29.99 {
		t.Errorf("Items[1].Price: got %f, want 29.99", result.Items[1].Price)
	}
}

func TestFromPbOrderPB(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	pbOrder := pb.OrderPB{
		OrderId:     789,
		ProductName: "Gadget",
		Quantity:    10,
		CreatedAt:   createdAt.Unix(),
		Status:      int32(model.OrderStatusShipped),
		Items: []*pb.OrderItemPB{
			{Sku: "SKU-100", Name: "PB Item", Quantity: 5, Price: 49.99},
		},
	}

	result := amgen.FromPbOrderPB(pbOrder)

	if result.ID != pbOrder.OrderId {
		t.Errorf("ID: got %d, want %d", result.ID, pbOrder.OrderId)
	}
	if result.CreatedAt.Unix() != createdAt.Unix() {
		t.Errorf("CreatedAt: got %v, want %v", result.CreatedAt, createdAt)
	}
	if result.Status != model.OrderStatusShipped {
		t.Errorf("Status: got %d, want %d", result.Status, model.OrderStatusShipped)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Items length: got %d, want 1", len(result.Items))
	}
	if result.Items[0].SKU != "SKU-100" {
		t.Errorf("Items[0].SKU: got %s, want SKU-100", result.Items[0].SKU)
	}
}

func TestToPbProductPB_NamedConverter(t *testing.T) {
	t.Parallel()

	product := model.Product{
		ID:    100,
		Name:  "Test Product",
		Price: "$19.99", // string format
	}

	result := amgen.ToPbProductPB(product)

	if result.Id != product.ID {
		t.Errorf("Id: got %d, want %d", result.Id, product.ID)
	}
	if result.Name != product.Name {
		t.Errorf("Name: got %s, want %s", result.Name, product.Name)
	}
	// $19.99 -> 1999 cents
	if result.Price != 1999 {
		t.Errorf("Price: got %d, want 1999", result.Price)
	}
}

func TestRoundTripOrder(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	original := model.Order{
		ID:          999,
		ProductName: "Round Trip Product",
		Quantity:    7,
		CreatedAt:   createdAt,
		Status:      model.OrderStatusDelivered,
		Items: []model.OrderItem{
			{SKU: "RT-001", Name: "RT Item", Quantity: 1, Price: 99.99},
		},
	}

	// Convert to PB and back
	pbOrder := amgen.ToPbOrderPB(original)
	result := amgen.FromPbOrderPB(pbOrder)

	if result.ID != original.ID {
		t.Errorf("ID: got %d, want %d", result.ID, original.ID)
	}
	if result.ProductName != original.ProductName {
		t.Errorf("ProductName: got %s, want %s", result.ProductName, original.ProductName)
	}
	// Note: time precision is lost to seconds
	if result.CreatedAt.Unix() != original.CreatedAt.Unix() {
		t.Errorf("CreatedAt: got %v, want %v", result.CreatedAt, original.CreatedAt)
	}
	if result.Status != original.Status {
		t.Errorf("Status: got %d, want %d", result.Status, original.Status)
	}
	if len(result.Items) != len(original.Items) {
		t.Fatalf("Items length: got %d, want %d", len(result.Items), len(original.Items))
	}
}
