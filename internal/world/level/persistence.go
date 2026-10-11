package level

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/storage"
)

// encodeGzipNBT encodes a named root compound into a gzip-compressed payload.
func encodeGzipNBT(root *nbt.Compound, initialCapacity int) ([]byte, error) {
	enc := nbt.NewEncoder(initialCapacity)
	if err := enc.WriteNamedRootCompound("", root); err != nil {
		return nil, fmt.Errorf("encode nbt: %w", err)
	}

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(enc.Bytes()); err != nil {
		return nil, fmt.Errorf("compress nbt: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("finish nbt compression: %w", err)
	}
	return compressed.Bytes(), nil
}

// backupCurrentFile atomically snapshots an existing file before replacement.
func backupCurrentFile(path, backupPath string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read current file: %w", err)
	}
	if err := storage.WriteFileAtomic(backupPath, data, 0644); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	return nil
}
