package level

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBackupWorldRejectsSymlink checks that a backup cannot follow outside paths.
func TestBackupWorldRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	worldDir := filepath.Join(root, "world")
	if err := os.Mkdir(worldDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(worldDir, "outside")); err != nil {
		t.Fatal(err)
	}
	if _, err := BackupWorld(worldDir); err == nil {
		t.Fatal("backup accepted a symlink")
	}
	if _, err := os.Stat(worldDir + ".vitis-backup"); !os.IsNotExist(err) {
		t.Fatalf("incomplete backup published: %v", err)
	}
}
