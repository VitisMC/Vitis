package persistence

import "testing"

// TestChunkBatchAcrossRegions verifies result alignment and negative-coordinate persistence.
func TestChunkBatchAcrossRegions(t *testing.T) {
	dir := t.TempDir()
	store := NewChunkStore(dir)
	chunks := []ChunkWrite{{X: -33, Z: -1, NBT: []byte("negative")}, {X: 0, Z: 0, NBT: []byte("origin")}, {X: -32, Z: 31, NBT: []byte("border")}, {X: 1, Z: 0, NBT: []byte("same region")}}
	results := store.WriteChunkBatch(chunks)
	if len(results) != len(chunks) {
		t.Fatal("result count differs")
	}
	for _, err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := NewChunkStore(dir)
	defer reopened.Close()
	for _, chunk := range chunks {
		data, err := reopened.ReadChunkNBT(chunk.X, chunk.Z)
		if err != nil || string(data) != string(chunk.NBT) {
			t.Fatalf("chunk %d,%d: %q %v", chunk.X, chunk.Z, data, err)
		}
	}
}
