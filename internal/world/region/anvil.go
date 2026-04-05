package region

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	sectorSize      = 4096
	headerSize      = 2 * sectorSize
	chunksPerAxis   = 32
	chunksPerRegion = chunksPerAxis * chunksPerAxis

	CompressionGzip = 1
	CompressionZlib = 2
	CompressionNone = 3
)

var (
	ErrNotFound = errors.New("region: chunk not found")
	ErrTooLarge = errors.New("region: chunk data too large")
	ErrBadData  = errors.New("region: corrupt chunk data")
)

// Region reads and writes Anvil .mca region files.
// Not goroutine-safe — callers must synchronize externally.
type Region struct {
	f          io.ReadWriteSeeker
	offsets    [chunksPerRegion]int32
	timestamps [chunksPerRegion]int32
	sectors    map[int32]bool
	mu         sync.Mutex
}

// Open opens an existing .mca region file.
func Open(path string) (*Region, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	r, err := Load(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return r, nil
}

// Load reads the region header from an io.ReadWriteSeeker.
func Load(f io.ReadWriteSeeker) (*Region, error) {
	r := &Region{
		f:       f,
		sectors: make(map[int32]bool),
	}

	if err := binary.Read(f, binary.BigEndian, &r.offsets); err != nil {
		return nil, fmt.Errorf("region: read offsets: %w", err)
	}
	r.sectors[0] = true

	if err := binary.Read(f, binary.BigEndian, &r.timestamps); err != nil {
		return nil, fmt.Errorf("region: read timestamps: %w", err)
	}
	r.sectors[1] = true

	for _, loc := range r.offsets {
		if sec, num := sectorLoc(loc); sec != 0 {
			for i := int32(0); i < num; i++ {
				r.sectors[sec+i] = true
			}
		}
	}
	return r, nil
}

// Create creates a new .mca region file with an empty header.
func Create(path string) (*Region, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0644)
	if err != nil {
		return nil, err
	}
	return createFrom(f)
}

func createFrom(f io.ReadWriteSeeker) (*Region, error) {
	r := &Region{
		f:       f,
		sectors: make(map[int32]bool),
	}
	if err := binary.Write(f, binary.BigEndian, &r.offsets); err != nil {
		r.Close()
		return nil, err
	}
	r.sectors[0] = true

	if err := binary.Write(f, binary.BigEndian, &r.timestamps); err != nil {
		r.Close()
		return nil, err
	}
	r.sectors[1] = true

	return r, nil
}

// Close closes the underlying file if it implements io.Closer.
func (r *Region) Close() error {
	if closer, ok := r.f.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// ReadChunk reads the raw compressed chunk data for the chunk at relative coordinates (x, z).
// x and z must be in the range [0, 31].
// Returns the raw bytes (compression_type_byte + compressed_nbt) for the caller to decompress.
func (r *Region) ReadChunk(x, z int) ([]byte, error) {
	idx := chunkIndex(x, z)
	sec, num := sectorLoc(r.offsets[idx])
	if sec == 0 {
		return nil, ErrNotFound
	}

	if _, err := r.f.Seek(int64(sec)*sectorSize, io.SeekStart); err != nil {
		return nil, err
	}

	var length int32
	if err := binary.Read(r.f, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length <= 0 {
		return nil, ErrNotFound
	}
	if length > int32(num)*sectorSize {
		return nil, ErrBadData
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r.f, data); err != nil {
		return nil, err
	}
	return data, nil
}

// ReadChunkNBT reads and decompresses the chunk NBT data at relative coordinates (x, z).
func (r *Region) ReadChunkNBT(x, z int) ([]byte, error) {
	raw, err := r.ReadChunk(x, z)
	if err != nil {
		return nil, err
	}
	if len(raw) < 1 {
		return nil, ErrBadData
	}

	compression := raw[0]
	compressed := raw[1:]

	var reader io.ReadCloser
	switch compression {
	case CompressionGzip:
		reader, err = gzip.NewReader(bytes.NewReader(compressed))
	case CompressionZlib:
		reader, err = zlib.NewReader(bytes.NewReader(compressed))
	case CompressionNone:
		return compressed, nil
	default:
		return nil, fmt.Errorf("region: unknown compression type %d", compression)
	}
	if err != nil {
		return nil, fmt.Errorf("region: decompress: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("region: read decompressed: %w", err)
	}
	return decompressed, nil
}

// WriteChunk writes raw chunk data (compression_type_byte + compressed_nbt) at relative coordinates.
func (r *Region) WriteChunk(x, z int, data []byte) error {
	need := int32((len(data) + 4 + sectorSize - 1) / sectorSize)
	if need >= 256 {
		return ErrTooLarge
	}

	idx := chunkIndex(x, z)
	n, now := sectorLoc(r.offsets[idx])

	if n != 0 && now == need {
		// overwrite in place
	} else {
		for i := int32(0); i < now; i++ {
			r.sectors[n+i] = false
		}

		n = r.findSpace(need)
		for i := int32(0); i < need; i++ {
			r.sectors[n+i] = true
		}

		r.offsets[idx] = (n << 8) | (need & 0xFF)
		if err := r.writeHeader(x, z); err != nil {
			return err
		}
	}

	if _, err := r.f.Seek(int64(n)*sectorSize, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(r.f, binary.BigEndian, int32(len(data))); err != nil {
		return err
	}
	if _, err := r.f.Write(data); err != nil {
		return err
	}

	padding := int(need)*sectorSize - len(data) - 4
	if padding > 0 {
		if _, err := r.f.Write(make([]byte, padding)); err != nil {
			return err
		}
	}

	return nil
}

// WriteChunkNBT compresses and writes NBT data at relative coordinates using zlib.
func (r *Region) WriteChunkNBT(x, z int, nbtData []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(CompressionZlib)
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(nbtData); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return r.WriteChunk(x, z, buf.Bytes())
}

// HasChunk returns true if the chunk at relative coordinates exists.
func (r *Region) HasChunk(x, z int) bool {
	return r.offsets[chunkIndex(x, z)] != 0
}

func sectorLoc(offset int32) (sec, num int32) {
	return (offset >> 8) & 0xFFFFFF, offset & 0xFF
}

func chunkIndex(x, z int) int {
	return (z&31)*chunksPerAxis + (x & 31)
}

func (r *Region) findSpace(need int32) int32 {
	n := int32(0)
	for i := int32(0); i < need; i++ {
		if r.sectors[n+i] {
			n += i + 1
			i = -1
		}
	}
	return n
}

func (r *Region) writeHeader(x, z int) error {
	idx := chunkIndex(x, z)
	var buf [4]byte

	binary.BigEndian.PutUint32(buf[:], uint32(r.offsets[idx]))
	if err := r.writeAt(buf[:], int64(idx)*4); err != nil {
		return err
	}

	binary.BigEndian.PutUint32(buf[:], uint32(r.timestamps[idx]))
	if err := r.writeAt(buf[:], sectorSize+int64(idx)*4); err != nil {
		return err
	}
	return nil
}

func (r *Region) writeAt(p []byte, off int64) error {
	if f, ok := r.f.(io.WriterAt); ok {
		_, err := f.WriteAt(p, off)
		return err
	}
	if _, err := r.f.Seek(off, io.SeekStart); err != nil {
		return err
	}
	_, err := r.f.Write(p)
	return err
}

// RegionPath returns the .mca filename for the given region coordinates.
func RegionPath(dir string, rx, rz int) string {
	return fmt.Sprintf("%s/r.%d.%d.mca", dir, rx, rz)
}

// ChunkToRegion converts chunk coordinates to region coordinates.
func ChunkToRegion(cx, cz int) (rx, rz int) {
	return cx >> 5, cz >> 5
}

// ChunkInRegion returns the chunk's local coordinates within its region.
func ChunkInRegion(cx, cz int) (x, z int) {
	return cx & 31, cz & 31
}
