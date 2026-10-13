package level

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/storage"
)

// ReadSavedData reads one bounded gzip-compressed persistent data document.
func ReadSavedData(path string) (*nbt.Compound, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := readGzipNBT(file)
	if err != nil {
		return nil, err
	}
	_, root, err := nbt.NewDecoder(data).ReadNamedRootCompound()
	if err != nil {
		return nil, err
	}
	version, ok := root.GetInt("DataVersion")
	if !ok || version != protocol.CurrentDataVersion {
		return nil, fmt.Errorf("unsupported SavedData version %d", version)
	}
	return root, nil
}

// WriteSavedData atomically writes an independently captured data document.
func WriteSavedData(path string, root *nbt.Compound) error {
	data, err := encodeGzipNBT(root, 16384)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return storage.WriteFileAtomic(path, data, 0644)
}
