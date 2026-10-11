package persistence

import (
	"path/filepath"
	"testing"
)

func TestChunkStoreWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	worldDir := filepath.Join(dir, "testworld")

	cs := NewChunkStore(worldDir)
	defer cs.Close()

	data := []byte("test chunk nbt data for persistence")
	if err := cs.WriteChunkNBT(0, 0, data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !cs.HasChunk(0, 0) {
		t.Fatal("expected chunk to exist")
	}

	got, err := cs.ReadChunkNBT(0, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("data mismatch: got %q", got)
	}
}

func TestChunkStoreMultipleRegions(t *testing.T) {
	dir := t.TempDir()
	cs := NewChunkStore(filepath.Join(dir, "world"))
	defer cs.Close()

	if err := cs.WriteChunkNBT(0, 0, []byte("chunk_0_0")); err != nil {
		t.Fatalf("Write 0,0: %v", err)
	}
	if err := cs.WriteChunkNBT(32, 0, []byte("chunk_32_0")); err != nil {
		t.Fatalf("Write 32,0: %v", err)
	}

	got1, _ := cs.ReadChunkNBT(0, 0)
	got2, _ := cs.ReadChunkNBT(32, 0)
	if string(got1) != "chunk_0_0" || string(got2) != "chunk_32_0" {
		t.Fatalf("data mismatch: %q, %q", got1, got2)
	}
}

func TestChunkStoreNotFound(t *testing.T) {
	dir := t.TempDir()
	cs := NewChunkStore(filepath.Join(dir, "world"))
	defer cs.Close()

	if cs.HasChunk(5, 5) {
		t.Fatal("expected not found")
	}
}

func TestChunkStoreClose(t *testing.T) {
	dir := t.TempDir()
	cs := NewChunkStore(filepath.Join(dir, "world"))
	cs.WriteChunkNBT(0, 0, []byte("data"))
	if err := cs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestChunkStoreSeparatesChunkEntityAndPOIRegions(t *testing.T) {
	worldDir := filepath.Join(t.TempDir(), "world")
	store := NewChunkStore(worldDir)
	defer store.Close()

	if err := store.WriteChunkNBT(-1, 33, []byte("chunk")); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if err := store.WriteEntityNBT(-1, 33, []byte("entities")); err != nil {
		t.Fatalf("write entities: %v", err)
	}
	if err := store.WritePOINBT(-1, 33, []byte("poi")); err != nil {
		t.Fatalf("write poi: %v", err)
	}

	readCases := []struct {
		name string
		read func(int, int) ([]byte, error)
		want string
	}{
		{name: "chunk", read: store.ReadChunkNBT, want: "chunk"},
		{name: "entities", read: store.ReadEntityNBT, want: "entities"},
		{name: "poi", read: store.ReadPOINBT, want: "poi"},
	}
	for _, testCase := range readCases {
		got, err := testCase.read(-1, 33)
		if err != nil {
			t.Fatalf("read %s: %v", testCase.name, err)
		}
		if string(got) != testCase.want {
			t.Fatalf("read %s = %q, want %q", testCase.name, got, testCase.want)
		}
	}

	for _, directory := range []string{"region", "entities", "poi"} {
		matches, err := filepath.Glob(filepath.Join(worldDir, directory, "*.mca"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("%s region files = %v, err=%v", directory, matches, err)
		}
	}
}
