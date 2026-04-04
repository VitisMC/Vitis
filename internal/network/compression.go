package network

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"sync"
)

var (
	ErrDecompressLength = fmt.Errorf("decompressed length mismatch")
	ErrDecompressFailed = fmt.Errorf("zlib decompression failed")
)

const maxDecompressedSize = 8 << 20

var zlibWriterPool = sync.Pool{
	New: func() any {
		w, _ := zlib.NewWriterLevel(io.Discard, zlib.DefaultCompression)
		return w
	},
}

var zlibReaderPool = sync.Pool{}

// CompressFrame applies Minecraft protocol compression framing to packet bytes.
// If len(packetBytes) >= threshold, the data is zlib-compressed and the frame is:
//
//	[VarInt(compressed_len + VarInt(uncompressed_len))] [VarInt(uncompressed_len)] [zlib(packetBytes)]
//
// If below threshold, the frame is:
//
//	[VarInt(1 + len(packetBytes))] [VarInt(0)] [packetBytes]
func CompressFrame(packetBytes []byte, threshold int) ([]byte, error) {
	if len(packetBytes) < threshold {
		dataLenSize := varIntSize(0)
		totalPayload := dataLenSize + len(packetBytes)
		outerLenSize := varIntSize(int32(totalPayload))

		out := make([]byte, outerLenSize+totalPayload)
		offset := writeVarInt(out, int32(totalPayload))
		offset += writeVarInt(out[offset:], 0)
		copy(out[offset:], packetBytes)
		return out, nil
	}

	var compressed bytes.Buffer
	compressed.Grow(len(packetBytes))

	w := zlibWriterPool.Get().(*zlib.Writer)
	w.Reset(&compressed)
	if _, err := w.Write(packetBytes); err != nil {
		w.Close()
		zlibWriterPool.Put(w)
		return nil, fmt.Errorf("compress frame: %w", err)
	}
	if err := w.Close(); err != nil {
		zlibWriterPool.Put(w)
		return nil, fmt.Errorf("compress frame close: %w", err)
	}
	zlibWriterPool.Put(w)

	compressedData := compressed.Bytes()
	uncompressedLen := int32(len(packetBytes))
	dataLenSize := varIntSize(uncompressedLen)
	totalPayload := dataLenSize + len(compressedData)
	outerLenSize := varIntSize(int32(totalPayload))

	out := make([]byte, outerLenSize+totalPayload)
	offset := writeVarInt(out, int32(totalPayload))
	offset += writeVarInt(out[offset:], uncompressedLen)
	copy(out[offset:], compressedData)
	return out, nil
}

// DecompressFrame reads the data_length VarInt from frame and decompresses if needed.
// Returns the raw packet bytes (VarInt packet_id + data).
func DecompressFrame(frame []byte) ([]byte, error) {
	dataLength, consumed, ok, err := decodeVarInt(frame)
	if err != nil {
		return nil, fmt.Errorf("decompress frame: read data_length: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("decompress frame: incomplete data_length varint")
	}

	remaining := frame[consumed:]

	if dataLength == 0 {
		out := make([]byte, len(remaining))
		copy(out, remaining)
		return out, nil
	}

	if dataLength < 0 || int(dataLength) > maxDecompressedSize {
		return nil, fmt.Errorf("decompress frame: invalid data_length %d", dataLength)
	}

	var r io.ReadCloser
	pooled := zlibReaderPool.Get()
	if pooled != nil {
		resetter, ok := pooled.(zlib.Resetter)
		if ok {
			if resetErr := resetter.Reset(bytes.NewReader(remaining), nil); resetErr == nil {
				r = pooled.(io.ReadCloser)
			}
		}
	}
	if r == nil {
		var zlibErr error
		r, zlibErr = zlib.NewReader(bytes.NewReader(remaining))
		if zlibErr != nil {
			return nil, fmt.Errorf("decompress frame: %w: %w", ErrDecompressFailed, zlibErr)
		}
	}

	decompressed := make([]byte, dataLength)
	n, readErr := io.ReadFull(r, decompressed)
	r.Close()
	zlibReaderPool.Put(r)

	if readErr != nil {
		return nil, fmt.Errorf("decompress frame: %w: %w", ErrDecompressFailed, readErr)
	}
	if int32(n) != dataLength {
		return nil, fmt.Errorf("decompress frame: %w: got %d expected %d", ErrDecompressLength, n, dataLength)
	}

	return decompressed, nil
}
