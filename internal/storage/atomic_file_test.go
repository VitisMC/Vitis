package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicReplacesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o640); err != nil {
		t.Fatalf("atomic write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replacement: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat replacement: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}
