package vanilla

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Unpack extracts a verified server bundle and its libraries for developer tooling.
func Unpack(ctx context.Context, serverJAR, versionID, output string) (err error) {
	if ctx == nil || !validVersion(versionID) {
		return fmt.Errorf("unpack server bundle: invalid context or version")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	metadata, inner, err := inspectBundle(serverJAR, versionID)
	if err != nil {
		return err
	}
	if metadata.ID != versionID {
		return fmt.Errorf("unpack server bundle: version mismatch")
	}
	if err := makeResourceDirectory(filepath.Dir(output)); err != nil {
		return err
	}
	if err := os.Mkdir(output, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(output))
		}
	}()
	if err := os.WriteFile(filepath.Join(output, "server.jar"), inner, 0644); err != nil {
		return err
	}
	archive, err := zip.OpenReader(serverJAR)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, archive.Close()) }()
	seen := make(map[string]bool)
	var total uint64
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(file.Name, "/")
		if !portableArchivePath(name) || seen[strings.ToLower(name)] {
			return fmt.Errorf("unsafe or duplicate bundle path %q", file.Name)
		}
		seen[strings.ToLower(name)] = true
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle symlink %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if !file.Mode().IsRegular() {
			return fmt.Errorf("non-regular bundle path %q", file.Name)
		}
		if !strings.HasPrefix(name, "META-INF/libraries/") {
			continue
		}
		if !strings.HasSuffix(name, ".jar") || file.UncompressedSize64 > maxResourceSize || total+file.UncompressedSize64 > maxResourceTotal {
			return fmt.Errorf("invalid or oversized server library %q", name)
		}
		total += file.UncompressedSize64
		data, err := readZipEntry(file, maxResourceSize)
		if err != nil {
			return err
		}
		target := filepath.Join(output, "libraries", filepath.FromSlash(strings.TrimPrefix(name, "META-INF/libraries/")))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
	}
	return ctx.Err()
}
