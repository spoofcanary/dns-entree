package migrate

import (
	"errors"
	"regexp"
	"testing"
)

var uuidV7Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewMigrationID(t *testing.T) {
	id := NewMigrationID()
	if len(id) != 36 {
		t.Fatalf("want 36-char UUID, got %d: %q", len(id), id)
	}
	if !uuidV7Re.MatchString(id) {
		t.Fatalf("not a UUIDv7: %q", id)
	}

	// Monotonic: 100 consecutive calls must be non-decreasing.
	prev := NewMigrationID()
	for i := 0; i < 100; i++ {
		next := NewMigrationID()
		if next < prev {
			t.Fatalf("UUIDv7 not time-ordered: %q < %q", next, prev)
		}
		prev = next
	}
}

func TestStoreErrors(t *testing.T) {
	errs := []error{ErrNotFound, ErrVersionMismatch, ErrExpired}
	for i, a := range errs {
		for j, b := range errs {
			if i == j {
				if !errors.Is(a, b) {
					t.Fatalf("errors.Is(%v, %v) = false", a, b)
				}
			} else {
				if errors.Is(a, b) {
					t.Fatalf("errors.Is(%v, %v) = true, want distinct", a, b)
				}
			}
		}
	}

	// ListFilter zero value is valid and empty.
	var f ListFilter
	if f.TenantID != "" || f.Status != "" || f.Limit != 0 || f.Offset != 0 {
		t.Fatalf("ListFilter zero value not empty: %+v", f)
	}
}
