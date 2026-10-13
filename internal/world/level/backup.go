package level

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// BackupWorld creates a durable, one-time sibling copy before a world is modified.
// An existing completed backup is never replaced.
func BackupWorld(worldDir string) (string, error) {
	source, err := filepath.Abs(worldDir)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("backup world: %s is not a directory", source)
	}
	target := source + ".vitis-backup"
	if existing, err := os.Lstat(target); err == nil {
		if !existing.IsDir() {
			return "", fmt.Errorf("backup world: %s is not a directory", target)
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(source)
	temp, err := os.MkdirTemp(parent, "."+filepath.Base(source)+".vitis-backup-")
	if err != nil {
		return "", fmt.Errorf("backup world: create temporary directory: %w", err)
	}
	defer os.RemoveAll(temp)
	var dirs []string
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(temp, rel)
		if entry.IsDir() {
			if rel != "." {
				if err := os.Mkdir(destination, 0755); err != nil {
					return err
				}
			}
			dirs = append(dirs, destination)
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("backup world: unsupported file type at %s", path)
		}
		return copyBackupFile(path, destination)
	})
	if err != nil {
		return "", fmt.Errorf("backup world: copy: %w", err)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncBackupDir(dirs[i]); err != nil {
			return "", fmt.Errorf("backup world: sync: %w", err)
		}
	}
	if existing, err := os.Lstat(target); err == nil {
		if !existing.IsDir() {
			return "", fmt.Errorf("backup world: %s is not a directory", target)
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(temp, target); err != nil {
		return "", fmt.Errorf("backup world: publish: %w", err)
	}
	if err := syncBackupDir(parent); err != nil {
		return "", fmt.Errorf("backup world: sync parent: %w", err)
	}
	return target, nil
}

// copyBackupFile copies a regular file and syncs it before publication.
func copyBackupFile(source, destination string) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup world: %s changed type during copy", source)
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); err == nil {
			err = closeErr
		}
	}()
	if _, err = io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}

// syncBackupDir persists directory entries before the backup is published.
func syncBackupDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
