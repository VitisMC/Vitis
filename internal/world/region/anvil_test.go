package region

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.0.0.mca")

	r, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != headerSize {
		t.Fatalf("expected file size %d, got %d", headerSize, info.Size())
	}
}

func TestWriteAndReadChunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.0.0.mca")

	r, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	testData := []byte("hello chunk nbt data for testing purposes")
	if err := r.WriteChunkNBT(0, 0, testData); err != nil {
		t.Fatalf("WriteChunkNBT: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r2, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r2.Close()

	if !r2.HasChunk(0, 0) {
		t.Fatal("expected chunk at (0,0)")
	}
	if r2.HasChunk(1, 0) {
		t.Fatal("unexpected chunk at (1,0)")
	}

	got, err := r2.ReadChunkNBT(0, 0)
	if err != nil {
		t.Fatalf("ReadChunkNBT: %v", err)
	}
	if string(got) != string(testData) {
		t.Fatalf("data mismatch: got %q, want %q", got, testData)
	}
}

func TestWriteMultipleChunks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.0.0.mca")

	r, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for x := 0; x < 4; x++ {
		for z := 0; z < 4; z++ {
			data := make([]byte, 1024)
			data[0] = byte(x)
			data[1] = byte(z)
			if err := r.WriteChunkNBT(x, z, data); err != nil {
				t.Fatalf("WriteChunkNBT(%d,%d): %v", x, z, err)
			}
		}
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r2, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r2.Close()

	for x := 0; x < 4; x++ {
		for z := 0; z < 4; z++ {
			if !r2.HasChunk(x, z) {
				t.Fatalf("missing chunk at (%d,%d)", x, z)
			}
			got, err := r2.ReadChunkNBT(x, z)
			if err != nil {
				t.Fatalf("ReadChunkNBT(%d,%d): %v", x, z, err)
			}
			if got[0] != byte(x) || got[1] != byte(z) {
				t.Fatalf("data mismatch at (%d,%d): got [%d,%d]", x, z, got[0], got[1])
			}
		}
	}
}

func TestReadNonExistentChunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.0.0.mca")

	r, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer r.Close()

	_, err = r.ReadChunk(5, 5)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOverwriteChunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.0.0.mca")

	r, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	data1 := []byte("first version of chunk data")
	if err := r.WriteChunkNBT(3, 7, data1); err != nil {
		t.Fatalf("WriteChunkNBT: %v", err)
	}

	data2 := []byte("second version overwrite")
	if err := r.WriteChunkNBT(3, 7, data2); err != nil {
		t.Fatalf("WriteChunkNBT overwrite: %v", err)
	}

	got, err := r.ReadChunkNBT(3, 7)
	if err != nil {
		t.Fatalf("ReadChunkNBT: %v", err)
	}
	if string(got) != string(data2) {
		t.Fatalf("expected overwritten data, got %q", got)
	}
	r.Close()
}

func TestChunkToRegion(t *testing.T) {
	tests := []struct {
		cx, cz   int
		rx, rz   int
	}{
		{0, 0, 0, 0},
		{31, 31, 0, 0},
		{32, 32, 1, 1},
		{-1, -1, -1, -1},
		{-32, -32, -1, -1},
		{-33, -33, -2, -2},
	}
	for _, tt := range tests {
		rx, rz := ChunkToRegion(tt.cx, tt.cz)
		if rx != tt.rx || rz != tt.rz {
			t.Errorf("ChunkToRegion(%d,%d) = (%d,%d), want (%d,%d)", tt.cx, tt.cz, rx, rz, tt.rx, tt.rz)
		}
	}
}

func TestChunkInRegion(t *testing.T) {
	tests := []struct {
		cx, cz int
		x, z   int
	}{
		{0, 0, 0, 0},
		{31, 31, 31, 31},
		{32, 32, 0, 0},
		{33, 33, 1, 1},
	}
	for _, tt := range tests {
		x, z := ChunkInRegion(tt.cx, tt.cz)
		if x != tt.x || z != tt.z {
			t.Errorf("ChunkInRegion(%d,%d) = (%d,%d), want (%d,%d)", tt.cx, tt.cz, x, z, tt.x, tt.z)
		}
	}
}

func TestRegionPath(t *testing.T) {
	got := RegionPath("/world/region", 1, -2)
	want := "/world/region/r.1.-2.mca"
	if got != want {
		t.Fatalf("RegionPath = %q, want %q", got, want)
	}
}
