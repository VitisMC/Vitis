package region

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMissingChunkQueriesDoNotWaitForDiskSync keeps generation and tick scheduling independent of IO.
func TestMissingChunkQueriesDoNotWaitForDiskSync(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "r.0.0.mca"))
	if err != nil {
		t.Fatal(err)
	}
	file := &batchTestFile{File: f}
	r, err := createFrom(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	barrier, release := make(chan struct{}), make(chan struct{})
	file.onSync = func(index int) {
		if index == 1 {
			close(barrier)
			<-release
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- r.WriteChunksNBT([]ChunkData{{X: 0, Z: 0, Data: []byte("a")}, {X: 1, Z: 0, Data: []byte("b")}})
	}()
	<-barrier
	queries := make(chan error, 1)
	go func() {
		if r.HasChunk(0, 0) {
			queries <- errors.New("published before sync")
			return
		}
		_, err := r.ReadChunk(0, 0)
		queries <- err
	}()
	var queryErr error
	select {
	case queryErr = <-queries:
	case <-time.After(time.Second):
		t.Error("missing chunk lookup blocked behind disk sync")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if queryErr != ErrNotFound {
		t.Fatalf("query error=%v, want not found", queryErr)
	}
	if !r.HasChunk(0, 0) || !r.HasChunk(1, 0) {
		t.Fatal("durable locations were not published")
	}
}

// batchTestFile injects durability and header failures without changing payload IO.
type batchTestFile struct {
	*os.File
	syncs, failSync, headerWrites, failHeader int
	onSync                                    func(int)
}

// Sync observes each barrier and optionally rejects it.
func (f *batchTestFile) Sync() error {
	f.syncs++
	if f.onSync != nil {
		f.onSync(f.syncs)
	}
	if f.syncs == f.failSync {
		return errors.New("injected sync failure")
	}
	return f.File.Sync()
}

// WriteAt rejects a selected header write before it reaches the file.
func (f *batchTestFile) WriteAt(data []byte, offset int64) (int, error) {
	f.headerWrites++
	if f.headerWrites == f.failHeader {
		return 0, errors.New("injected header failure")
	}
	return f.File.WriteAt(data, offset)
}

// TestRegionBatchDurability checks two barriers, delayed publication and persisted content.
func TestRegionBatchDurability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.0.0.mca")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	file := &batchTestFile{File: f}
	r, err := createFrom(file)
	if err != nil {
		t.Fatal(err)
	}
	chunks := []ChunkData{{X: 0, Z: 0, Data: []byte("first")}, {X: 31, Z: 31, Data: []byte("last")}, {X: 1, Z: 2, Data: []byte("middle")}}
	file.onSync = func(barrier int) {
		for _, chunk := range chunks {
			if r.offsets[chunkIndex(chunk.X, chunk.Z)] != 0 {
				t.Error("locations published before final sync returned")
			}
		}
		var header [headerSize]byte
		if _, err := file.File.ReadAt(header[:], 0); err != nil {
			t.Fatal(err)
		}
		for _, chunk := range chunks {
			offset := binary.BigEndian.Uint32(header[chunkIndex(chunk.X, chunk.Z)*4:])
			if (barrier == 1 && offset != 0) || (barrier == 2 && offset == 0) {
				t.Errorf("header order at barrier %d", barrier)
			}
		}
	}
	if err := r.WriteChunksNBT(chunks); err != nil {
		t.Fatal(err)
	}
	if file.syncs != 2 {
		t.Fatalf("syncs=%d, want 2", file.syncs)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, chunk := range chunks {
		got, err := reopened.ReadChunkNBT(chunk.X, chunk.Z)
		if err != nil || string(got) != string(chunk.Data) {
			t.Fatalf("chunk %d,%d: %q %v", chunk.X, chunk.Z, got, err)
		}
	}
}

// TestRegionBatchFailurePreservesPreviousPayload protects copy-on-write reservations and retry.
func TestRegionBatchFailurePreservesPreviousPayload(t *testing.T) {
	for _, scenario := range []struct {
		failure string
		count   int
	}{{"payload_sync", 1}, {"header", 1}, {"header_sync", 1}, {"payload_sync", 2}, {"header", 2}, {"header_sync", 2}} {
		t.Run(fmt.Sprintf("%s/count=%d", scenario.failure, scenario.count), func(t *testing.T) {
			f, err := os.Create(filepath.Join(t.TempDir(), "r.0.0.mca"))
			if err != nil {
				t.Fatal(err)
			}
			file := &batchTestFile{File: f}
			r, err := createFrom(file)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err := r.WriteChunkNBT(0, 0, []byte("old")); err != nil {
				t.Fatal(err)
			}
			oldSector, oldCount := sectorLoc(r.offsets[0])
			file.syncs, file.headerWrites = 0, 0
			switch scenario.failure {
			case "payload_sync":
				file.failSync = 1
			case "header_sync":
				file.failSync = 2
			case "header":
				file.failHeader = 2
				if scenario.count > 1 {
					file.failHeader = 3
				}
			}
			chunks := []ChunkData{{X: 0, Z: 0, Data: []byte("new")}, {X: 1, Z: 0, Data: []byte("second")}}
			chunks = chunks[:scenario.count]
			if err := r.WriteChunksNBT(chunks); err == nil {
				t.Fatal("failure was ignored")
			}
			got, err := r.ReadChunkNBT(0, 0)
			if err != nil || string(got) != "old" || r.HasChunk(1, 0) {
				t.Fatal("failed batch published locations or lost old data")
			}
			for sector := int32(0); sector < oldCount; sector++ {
				if !r.sectors[oldSector+sector] {
					t.Fatal("old sectors released before durable commit")
				}
			}
			file.failSync, file.failHeader = 0, 0
			if err := r.WriteChunksNBT(chunks); err != nil {
				t.Fatal(err)
			}
			for _, chunk := range chunks {
				got, err := r.ReadChunkNBT(chunk.X, chunk.Z)
				if err != nil || string(got) != string(chunk.Data) {
					t.Fatal("retry did not preserve batch content")
				}
			}
		})
	}
}

// TestRegionBatchRejectsDuplicateCoordinates verifies validation before either durability barrier.
func TestRegionBatchRejectsDuplicateCoordinates(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "r.0.0.mca"))
	if err != nil {
		t.Fatal(err)
	}
	file := &batchTestFile{File: f}
	r, err := createFrom(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.WriteChunksNBT([]ChunkData{{X: 0, Z: 0, Data: []byte("a")}, {X: 0, Z: 0, Data: []byte("b")}}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if file.syncs != 0 || r.HasChunk(0, 0) {
		t.Fatal("invalid batch changed the region")
	}
}
