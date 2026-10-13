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
	"sync/atomic"
)

const (
	sectorSize      = 4096
	headerSize      = 2 * sectorSize
	chunksPerAxis   = 32
	chunksPerRegion = chunksPerAxis * chunksPerAxis
	maxChunkNBTSize = 16 << 20

	CompressionGzip = 1
	CompressionZlib = 2
	CompressionNone = 3
)

var (
	ErrNotFound = errors.New("region: chunk not found")
	ErrTooLarge = errors.New("region: chunk data too large")
	ErrBadData  = errors.New("region: corrupt chunk data")
	ErrBadCoord = errors.New("region: chunk coordinate out of range")
)

// compressionBuffer owns a reusable writer and its exclusively borrowed output buffer.
type compressionBuffer struct {
	buffer bytes.Buffer
	writer *zlib.Writer
}

var compressionPool = sync.Pool{New: func() any {
	state := &compressionBuffer{}
	state.writer = zlib.NewWriter(&state.buffer)
	return state
}}

// Region reads and writes Anvil .mca region files.
type Region struct {
	f          io.ReadWriteSeeker
	offsets    [chunksPerRegion]int32
	published  [chunksPerRegion]atomic.Int32
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
	for index, offset := range r.offsets {
		r.published[index].Store(offset)
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if closer, ok := r.f.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// ReadChunk reads the raw compressed chunk data for the chunk at relative coordinates (x, z).
// x and z must be in the range [0, 31].
// Returns the raw bytes (compression_type_byte + compressed_nbt) for the caller to decompress.
func (r *Region) ReadChunk(x, z int) ([]byte, error) {
	if err := validateChunkCoord(x, z); err != nil {
		return nil, err
	}
	if r.published[chunkIndex(x, z)].Load() == 0 {
		return nil, ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readChunkLocked(x, z)
}

func (r *Region) readChunkLocked(x, z int) ([]byte, error) {
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
	if length > int32(num)*sectorSize-4 {
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
		if len(compressed) > maxChunkNBTSize {
			return nil, ErrTooLarge
		}
		return bytes.Clone(compressed), nil
	default:
		return nil, fmt.Errorf("region: unknown compression type %d", compression)
	}
	if err != nil {
		return nil, fmt.Errorf("region: decompress: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(io.LimitReader(reader, maxChunkNBTSize+1))
	if err != nil {
		return nil, fmt.Errorf("region: read decompressed: %w", err)
	}
	if len(decompressed) > maxChunkNBTSize {
		return nil, ErrTooLarge
	}
	return decompressed, nil
}

// WriteChunk writes raw chunk data (compression_type_byte + compressed_nbt) at relative coordinates.
func (r *Region) WriteChunk(x, z int, data []byte) error {
	return r.writeBatch([]ChunkData{{X: x, Z: z, Data: data}})
}

// WriteChunkNBT compresses and writes NBT data at relative coordinates using zlib.
func (r *Region) WriteChunkNBT(x, z int, nbtData []byte) error {
	if len(nbtData) > maxChunkNBTSize {
		return ErrTooLarge
	}
	state := compressionPool.Get().(*compressionBuffer)
	state.buffer.Reset()
	state.buffer.WriteByte(CompressionZlib)
	w := state.writer
	w.Reset(&state.buffer)
	defer func() {
		w.Reset(io.Discard)
		if state.buffer.Cap() <= 1<<20 {
			compressionPool.Put(state)
		}
	}()
	if _, err := w.Write(nbtData); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return r.WriteChunk(x, z, state.buffer.Bytes())
}

// HasChunk returns true if the chunk at relative coordinates exists.
func (r *Region) HasChunk(x, z int) bool {
	if validateChunkCoord(x, z) != nil {
		return false
	}
	return r.published[chunkIndex(x, z)].Load() != 0
}

func validateChunkCoord(x, z int) error {
	if x < 0 || x >= chunksPerAxis || z < 0 || z >= chunksPerAxis {
		return ErrBadCoord
	}
	return nil
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

	binary.BigEndian.PutUint32(buf[:], uint32(r.timestamps[idx]))
	if err := r.writeAt(buf[:], sectorSize+int64(idx)*4); err != nil {
		return err
	}

	// The location entry is the commit point. The payload and timestamp are
	// durable before readers can discover the new sectors.
	binary.BigEndian.PutUint32(buf[:], uint32(r.offsets[idx]))
	if err := r.writeAt(buf[:], int64(idx)*4); err != nil {
		return err
	}
	return nil
}

func (r *Region) writeAt(p []byte, off int64) error {
	if f, ok := r.f.(io.WriterAt); ok {
		n, err := f.WriteAt(p, off)
		if err == nil && n != len(p) {
			return io.ErrShortWrite
		}
		return err
	}
	if _, err := r.f.Seek(off, io.SeekStart); err != nil {
		return err
	}
	return writeFull(r.f, p)
}

func (r *Region) sync() error {
	if syncer, ok := r.f.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}
	return nil
}

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
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
