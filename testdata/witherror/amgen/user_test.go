package amgen_test

import (
	"testing"
	"time"

	"github.com/mickamy/automapper/testdata/witherror/amgen"
	"github.com/mickamy/automapper/testdata/witherror/model"
	"github.com/mickamy/automapper/testdata/witherror/pb"
)

func TestToPbUserPB(t *testing.T) {
	birthDate := time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC)
	user := model.User{
		ID:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		BirthDate: birthDate,
	}

	result := amgen.ToPbUserPB(user)

	if result.Id != 1 {
		t.Errorf("expected Id=1, got %d", result.Id)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name=Alice, got %s", result.Name)
	}
	if result.Email != "alice@example.com" {
		t.Errorf("expected Email=alice@example.com, got %s", result.Email)
	}
	if result.BirthDate != "1990-05-15T00:00:00Z" {
		t.Errorf("expected BirthDate=1990-05-15T00:00:00Z, got %s", result.BirthDate)
	}
}

func TestFromPbUserPB_Success(t *testing.T) {
	pbUser := pb.UserPB{
		Id:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		BirthDate: "1990-05-15T00:00:00Z",
	}

	result, err := amgen.FromPbUserPB(pbUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != 1 {
		t.Errorf("expected ID=1, got %d", result.ID)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name=Alice, got %s", result.Name)
	}
	expectedDate := time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC)
	if !result.BirthDate.Equal(expectedDate) {
		t.Errorf("expected BirthDate=%v, got %v", expectedDate, result.BirthDate)
	}
}

func TestFromPbUserPB_Error(t *testing.T) {
	pbUser := pb.UserPB{
		Id:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		BirthDate: "invalid-date",
	}

	_, err := amgen.FromPbUserPB(pbUser)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check error contains field name
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	t.Logf("error: %v", err)
}

func TestFromPbUserPBPtr_NilInput(t *testing.T) {
	result, err := amgen.FromPbUserPBPtr(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestFromPbUserPBPtr_Error(t *testing.T) {
	pbUser := &pb.UserPB{
		Id:        1,
		Name:      "Alice",
		BirthDate: "not-a-valid-date",
	}

	result, err := amgen.FromPbUserPBPtr(pbUser)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
	t.Logf("error: %v", err)
}
