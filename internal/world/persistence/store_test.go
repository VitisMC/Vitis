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
