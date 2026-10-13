package region

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

const maxBatchChunks = 64

// ChunkData carries one exclusively owned payload at region-relative coordinates.
type ChunkData struct {
	X, Z int
	Data []byte
}

// WriteChunksNBT commits a bounded batch with durable payloads before publishing locations.
// Errors acknowledge none of the batch; previously stored payloads remain reserved and readable.
func (r *Region) WriteChunksNBT(chunks []ChunkData) error {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) > maxBatchChunks {
		return ErrTooLarge
	}
	if len(chunks) == 1 {
		return r.WriteChunkNBT(chunks[0].X, chunks[0].Z, chunks[0].Data)
	}
	compressed := make([]ChunkData, len(chunks))
	state := compressionPool.Get().(*compressionBuffer)
	defer func() {
		state.writer.Reset(io.Discard)
		if state.buffer.Cap() <= 1<<20 {
			compressionPool.Put(state)
		}
	}()
	for index, chunk := range chunks {
		if err := validateChunkCoord(chunk.X, chunk.Z); err != nil {
			return err
		}
		if len(chunk.Data) > maxChunkNBTSize {
			return ErrTooLarge
		}
		state.buffer.Reset()
		state.buffer.WriteByte(CompressionZlib)
		state.writer.Reset(&state.buffer)
		if _, err := state.writer.Write(chunk.Data); err != nil {
			return err
		}
		if err := state.writer.Close(); err != nil {
			return err
		}
		compressed[index] = ChunkData{X: chunk.X, Z: chunk.Z, Data: bytes.Clone(state.buffer.Bytes())}
	}
	return r.writeBatch(compressed)
}

// batchLocation retains copy-on-write reservations until both durability barriers succeed.
type batchLocation struct {
	index, sector, count, oldSector, oldCount int32
}

// writeBatch shares two durability barriers without exposing uncommitted in-memory locations.
func (r *Region) writeBatch(chunks []ChunkData) error {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) > maxBatchChunks {
		return ErrTooLarge
	}
	var locations [maxBatchChunks]batchLocation
	var seen [chunksPerRegion]bool
	for index, chunk := range chunks {
		if err := validateChunkCoord(chunk.X, chunk.Z); err != nil {
			return err
		}
		count := int32((len(chunk.Data) + 4 + sectorSize - 1) / sectorSize)
		if count <= 0 || count >= 256 {
			return ErrTooLarge
		}
		slot := chunkIndex(chunk.X, chunk.Z)
		if seen[slot] {
			return fmt.Errorf("region: duplicate chunk (%d,%d) in batch", chunk.X, chunk.Z)
		}
		seen[slot] = true
		locations[index].index, locations[index].count = int32(slot), count
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	reserved, headersStarted := 0, false
	defer func() {
		if !headersStarted {
			for _, location := range locations[:reserved] {
				for sector := int32(0); sector < location.count; sector++ {
					r.sectors[location.sector+sector] = false
				}
			}
		}
	}()
	for index, chunk := range chunks {
		location := &locations[index]
		location.oldSector, location.oldCount = sectorLoc(r.offsets[location.index])
		location.sector = r.findSpace(location.count)
		for sector := int32(0); sector < location.count; sector++ {
			r.sectors[location.sector+sector] = true
		}
		reserved++
		if _, err := r.f.Seek(int64(location.sector)*sectorSize, io.SeekStart); err != nil {
			return err
		}
		if err := binary.Write(r.f, binary.BigEndian, int32(len(chunk.Data))); err != nil {
			return err
		}
		if err := writeFull(r.f, chunk.Data); err != nil {
			return err
		}
		if padding := int(location.count)*sectorSize - len(chunk.Data) - 4; padding > 0 {
			if err := writeFull(r.f, make([]byte, padding)); err != nil {
				return err
			}
		}
	}
	if err := r.sync(); err != nil {
		return err
	}
	timestamp := int32(time.Now().Unix())
	// After publication starts, retain both old and new sectors on failure: a header may have reached disk.
	headersStarted = true
	for index := range chunks {
		location := &locations[index]
		var entry [4]byte
		binary.BigEndian.PutUint32(entry[:], uint32(timestamp))
		if err := r.writeAt(entry[:], sectorSize+int64(location.index)*4); err != nil {
			return err
		}
		binary.BigEndian.PutUint32(entry[:], uint32((location.sector<<8)|location.count))
		if err := r.writeAt(entry[:], int64(location.index)*4); err != nil {
			return err
		}
	}
	if err := r.sync(); err != nil {
		return err
	}
	for index := range chunks {
		location := &locations[index]
		r.offsets[location.index] = (location.sector << 8) | location.count
		r.timestamps[location.index] = timestamp
		r.published[location.index].Store(r.offsets[location.index])
		for sector := int32(0); sector < location.oldCount; sector++ {
			r.sectors[location.oldSector+sector] = false
		}
	}
	return nil
}
