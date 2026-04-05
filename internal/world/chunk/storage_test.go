package chunk

import "testing"

func TestStorageSetGetDeleteLen(t *testing.T) {
	storage := NewStorage(0)
	if storage.Len() != 0 {
		t.Fatalf("expected empty storage")
	}

	chunkA := New(1, 2)
	storage.Set(chunkA)
	if storage.Len() != 1 {
		t.Fatalf("expected storage len 1, got %d", storage.Len())
	}

	got, ok := storage.Get(1, 2)
	if !ok || got != chunkA {
		t.Fatalf("expected to retrieve inserted chunk")
	}

	storage.Delete(1, 2)
	if storage.Len() != 0 {
		t.Fatalf("expected empty storage after delete, got %d", storage.Len())
	}
	if _, exists := storage.Get(1, 2); exists {
		t.Fatalf("expected chunk removed")
	}
}

func TestStorageOverwriteSameKey(t *testing.T) {
	storage := NewStorage(4)
	chunkA := New(10, 20)
	chunkB := New(10, 20)
	chunkB.Touch(99)

	storage.Set(chunkA)
	storage.Set(chunkB)

	got, ok := storage.Get(10, 20)
	if !ok {
		t.Fatalf("expected chunk for key")
	}
	if got != chunkB {
		t.Fatalf("expected latest chunk instance")
	}
	if got.LastAccessTick() != 99 {
		t.Fatalf("expected overwritten chunk metadata to persist")
	}
	if storage.Len() != 1 {
		t.Fatalf("expected len to remain 1, got %d", storage.Len())
	}
}
