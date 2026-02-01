package amgen_test

import (
	"testing"

	"github.com/mickamy/automapper/testdata/simple/amgen"
	"github.com/mickamy/automapper/testdata/simple/model"
	"github.com/mickamy/automapper/testdata/simple/pb"
)

func TestToPbUserPB(t *testing.T) {
	user := model.User{
		ID:     123,
		Name:   "John Doe",
		Email:  "john@example.com",
		Active: true,
		Tags:   []string{"admin", "user"},
		Address: &model.Address{
			Street:  "123 Main St",
			City:    "New York",
			Country: "USA",
		},
	}

	result := amgen.ToPbUserPB(user)

	if result.Id != user.ID {
		t.Errorf("Id: got %d, want %d", result.Id, user.ID)
	}
	if result.Name != user.Name {
		t.Errorf("Name: got %s, want %s", result.Name, user.Name)
	}
	if result.Email != user.Email {
		t.Errorf("Email: got %s, want %s", result.Email, user.Email)
	}
	if result.Active != user.Active {
		t.Errorf("Active: got %v, want %v", result.Active, user.Active)
	}
	if len(result.Tags) != len(user.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(result.Tags), len(user.Tags))
	}
	if result.Address == nil {
		t.Fatal("Address: got nil")
	}
	if result.Address.Street != user.Address.Street {
		t.Errorf("Address.Street: got %s, want %s", result.Address.Street, user.Address.Street)
	}
}

func TestToPbUserPBPtr(t *testing.T) {
	user := &model.User{
		ID:   456,
		Name: "Jane Doe",
	}

	result := amgen.ToPbUserPBPtr(user)

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Id != user.ID {
		t.Errorf("Id: got %d, want %d", result.Id, user.ID)
	}
}

func TestToPbUserPBPtr_Nil(t *testing.T) {
	result := amgen.ToPbUserPBPtr(nil)

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestToPbAddressPBPtr_Nil(t *testing.T) {
	result := amgen.ToPbAddressPBPtr(nil)

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFromPbUserPB(t *testing.T) {
	pbUser := pb.UserPB{
		Id:     789,
		Name:   "Bob Smith",
		Email:  "bob@example.com",
		Active: true,
		Tags:   []string{"viewer"},
		Address: &pb.AddressPB{
			Street:  "456 Oak Ave",
			City:    "Boston",
			Country: "USA",
		},
	}

	result := amgen.FromPbUserPB(pbUser)

	if result.ID != pbUser.Id {
		t.Errorf("ID: got %d, want %d", result.ID, pbUser.Id)
	}
	if result.Name != pbUser.Name {
		t.Errorf("Name: got %s, want %s", result.Name, pbUser.Name)
	}
	if result.Email != pbUser.Email {
		t.Errorf("Email: got %s, want %s", result.Email, pbUser.Email)
	}
	if result.Address == nil {
		t.Fatal("Address: got nil")
	}
	if result.Address.City != pbUser.Address.City {
		t.Errorf("Address.City: got %s, want %s", result.Address.City, pbUser.Address.City)
	}
}

func TestFromPbUserPBPtr_Nil(t *testing.T) {
	result := amgen.FromPbUserPBPtr(nil)

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestRoundTrip(t *testing.T) {
	original := model.User{
		ID:     999,
		Name:   "Round Trip",
		Email:  "rt@example.com",
		Active: true,
		Tags:   []string{"test"},
		Address: &model.Address{
			Street:  "1 Circle Dr",
			City:    "Loop City",
			Country: "Cycle",
		},
	}

	// Convert to PB and back
	pb := amgen.ToPbUserPB(original)
	result := amgen.FromPbUserPB(pb)

	if result.ID != original.ID {
		t.Errorf("ID: got %d, want %d", result.ID, original.ID)
	}
	if result.Name != original.Name {
		t.Errorf("Name: got %s, want %s", result.Name, original.Name)
	}
	if result.Address.Street != original.Address.Street {
		t.Errorf("Address.Street: got %s, want %s", result.Address.Street, original.Address.Street)
	}
}

// Ensure types are used
var (
	_ pb.UserPB
)
