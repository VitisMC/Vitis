package network

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	ErrDecompressLength = fmt.Errorf("decompressed length mismatch")
	ErrDecompressFailed = fmt.Errorf("zlib decompression failed")
)

const maxDecompressedSize = 8 << 20

const maxRetainedCompressionBuffer = 1 << 20

type compressionWorkspace struct {
	buffer bytes.Buffer
	writer *zlib.Writer
}

var zlibWriterCache = make(chan *compressionWorkspace, 32)

// acquireCompressionWorkspace returns an isolated reusable compressor and output buffer.
func acquireCompressionWorkspace() (*compressionWorkspace, error) {
	select {
	case workspace := <-zlibWriterCache:
		return workspace, nil
	default:
		writer, err := zlib.NewWriterLevel(io.Discard, zlib.BestSpeed)
		if err != nil {
			return nil, err
		}
		return &compressionWorkspace{writer: writer}, nil
	}
}

// releaseCompressionWorkspace retains bounded buffers for a later frame.
func releaseCompressionWorkspace(workspace *compressionWorkspace) {
	if workspace.buffer.Cap() > maxRetainedCompressionBuffer {
		return
	}
	workspace.buffer.Reset()
	select {
	case zlibWriterCache <- workspace:
	default:
	}
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

	workspace, err := acquireCompressionWorkspace()
	if err != nil {
		return nil, fmt.Errorf("create compressor: %w", err)
	}
	workspace.buffer.Reset()
	workspace.buffer.Grow(len(packetBytes))
	workspace.writer.Reset(&workspace.buffer)
	if _, err := workspace.writer.Write(packetBytes); err != nil {
		return nil, fmt.Errorf("compress frame: %w", err)
	}
	if err := workspace.writer.Close(); err != nil {
		return nil, fmt.Errorf("compress frame close: %w", err)
	}
	compressedData := workspace.buffer.Bytes()
	uncompressedLen := int32(len(packetBytes))
	dataLenSize := varIntSize(uncompressedLen)
	totalPayload := dataLenSize + len(compressedData)
	outerLenSize := varIntSize(int32(totalPayload))

	out := make([]byte, outerLenSize+totalPayload)
	offset := writeVarInt(out, int32(totalPayload))
	offset += writeVarInt(out[offset:], uncompressedLen)
	copy(out[offset:], compressedData)
	releaseCompressionWorkspace(workspace)
	return out, nil
}

// DecompressFrame reads one compressed protocol frame without enforcing a negotiated threshold.
func DecompressFrame(frame []byte) ([]byte, error) {
	return DecompressFrameWithThreshold(frame, -1)
}

// DecompressFrameWithThreshold reads one compressed protocol frame and validates
// the representation against the negotiated compression threshold.
func DecompressFrameWithThreshold(frame []byte, threshold int) ([]byte, error) {
	dataLength, consumed, ok, err := decodeVarInt(frame)
	if err != nil {
		return nil, fmt.Errorf("decompress frame: read data_length: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("decompress frame: incomplete data_length varint")
	}

	remaining := frame[consumed:]

	if dataLength == 0 {
		if threshold >= 0 && len(remaining) >= threshold {
			return nil, fmt.Errorf("decompress frame: uncompressed payload %d is not below threshold %d", len(remaining), threshold)
		}
		out := make([]byte, len(remaining))
		copy(out, remaining)
		return out, nil
	}

	if dataLength < 0 || int(dataLength) > maxDecompressedSize {
		return nil, fmt.Errorf("decompress frame: invalid data_length %d", dataLength)
	}
	if threshold >= 0 && int(dataLength) < threshold {
		return nil, fmt.Errorf("decompress frame: compressed payload %d is below threshold %d", dataLength, threshold)
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
	if readErr == nil {
		var trailing [1]byte
		trailingN, trailingErr := r.Read(trailing[:])
		switch {
		case trailingN != 0:
			readErr = fmt.Errorf("%w: output exceeds declared length %d", ErrDecompressLength, dataLength)
		case trailingErr != nil && !errors.Is(trailingErr, io.EOF):
			readErr = trailingErr
		case trailingErr == nil:
			readErr = fmt.Errorf("%w: stream did not terminate", ErrDecompressFailed)
		}
	}
	closeErr := r.Close()
	zlibReaderPool.Put(r)

	if readErr != nil {
		return nil, fmt.Errorf("decompress frame: %w: %w", ErrDecompressFailed, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("decompress frame: %w: %w", ErrDecompressFailed, closeErr)
	}
	if int32(n) != dataLength {
		return nil, fmt.Errorf("decompress frame: %w: got %d expected %d", ErrDecompressLength, n, dataLength)
	}

	return decompressed, nil
}
