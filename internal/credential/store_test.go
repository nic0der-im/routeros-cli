package credential

import (
	"errors"
	"testing"
)

func TestMemoryStore_SetThenGet(t *testing.T) {
	store := NewMemoryStore()

	if err := store.Set("router1", "secret123"); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	password, err := store.Get("router1")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	if password != "secret123" {
		t.Errorf("expected password %q, got %q", "secret123", password)
	}
}

func TestMemoryStore_GetNonExistent(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Get("unknown")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_DeleteThenGet(t *testing.T) {
	store := NewMemoryStore()

	if err := store.Set("router1", "secret123"); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	if err := store.Delete("router1"); err != nil {
		t.Fatalf("Delete returned unexpected error: %v", err)
	}

	_, err := store.Get("router1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after Delete, got %v", err)
	}
}

func TestMemoryStore_DeleteNonExistent(t *testing.T) {
	store := NewMemoryStore()

	err := store.Delete("unknown")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_SetOverwrites(t *testing.T) {
	store := NewMemoryStore()

	if err := store.Set("router1", "old-password"); err != nil {
		t.Fatalf("Set returned unexpected error: %v", err)
	}

	if err := store.Set("router1", "new-password"); err != nil {
		t.Fatalf("Set (overwrite) returned unexpected error: %v", err)
	}

	password, err := store.Get("router1")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}
	if password != "new-password" {
		t.Errorf("expected password %q after overwrite, got %q", "new-password", password)
	}
}

// Verify both implementations satisfy the Store interface at compile time.
var (
	_ Store = (*KeyringStore)(nil)
	_ Store = (*MemoryStore)(nil)
)
