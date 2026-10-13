package vanilla

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitismc/vitis/internal/protocol"
)

// Validate checks a local pack's identity, bounded file tree and runtime resources.
func Validate(ctx context.Context, expected Version, directory string) error {
	if ctx == nil {
		return fmt.Errorf("validate vanilla resources: nil context")
	}
	if err := walkPack(ctx, directory, nil); err != nil {
		return err
	}
	metadata, err := os.ReadFile(filepath.Join(directory, "pack.mcmeta"))
	if err != nil {
		return err
	}
	var pack struct {
		Pack struct {
			Description string `json:"description"`
			Format      int    `json:"pack_format"`
		} `json:"pack"`
	}
	if err := json.Unmarshal(metadata, &pack); err != nil {
		return err
	}
	if pack.Pack.Format != expected.PackVersion.Data {
		return fmt.Errorf("vanilla pack format %d, expected %d", pack.Pack.Format, expected.PackVersion.Data)
	}
	identity, err := os.ReadFile(filepath.Join(directory, "version.json"))
	if errors.Is(err, os.ErrNotExist) {
		// Older Vitis packs only recorded their version in the pack description.
		if pack.Pack.Description != "Vitis vanilla "+expected.ID || expected.ID != protocol.CurrentMinecraftVersion || expected.Protocol != protocol.CurrentVersion || expected.WorldVersion != protocol.CurrentDataVersion {
			return fmt.Errorf("legacy vanilla pack has no matching supported version identity")
		}
	} else if err != nil {
		return err
	} else {
		var actual Version
		if err := json.Unmarshal(identity, &actual); err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("vanilla version mismatch: got %+v, expected %+v", actual, expected)
		}
	}
	if err := validateRuntimePack(directory); err != nil {
		return err
	}
	return ctx.Err()
}

// writeVersion records the identity verified against the original nested JAR.
func writeVersion(directory string, version Version) error {
	data, err := json.Marshal(version)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "version.json"), append(data, '\n'), 0644)
}

// walkPack rejects unsafe local entries and bounds the complete tree before visiting files.
func walkPack(ctx context.Context, directory string, visit func(string, fs.FileInfo) error) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("vanilla pack %q must be a regular directory", directory)
	}
	if err := makeResourceDirectory(directory); err != nil {
		return err
	}
	seen := make(map[string]bool)
	var total int64
	return filepath.WalkDir(directory, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == directory {
			return nil
		}
		relative, err := filepath.Rel(directory, name)
		if err != nil {
			return err
		}
		portable := filepath.ToSlash(relative)
		if !portableArchivePath(portable) {
			return fmt.Errorf("unsafe vanilla path %q", relative)
		}
		key := strings.ToLower(portable)
		if seen[key] {
			return fmt.Errorf("duplicate vanilla path %q", relative)
		}
		seen[key] = true
		if len(seen) > maxResourceEntries {
			return fmt.Errorf("vanilla pack exceeds entry limit")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular vanilla entry %q", relative)
		}
		if info.Size() > maxResourceSize || info.Size() < 0 || total > maxResourceTotal-info.Size() {
			return fmt.Errorf("vanilla pack exceeds size limit")
		}
		total += info.Size()
		if visit != nil {
			return visit(relative, info)
		}
		return nil
	})
}

// copyRuntimePack publishes a validated local copy while preserving the source.
func copyRuntimePack(ctx context.Context, expected Version, source, output string) (err error) {
	if err := Validate(ctx, expected, source); err != nil {
		return fmt.Errorf("existing vanilla source %q is invalid (preserved): %w", source, err)
	}
	parent := filepath.Dir(output)
	if err := makeResourceDirectory(parent); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".vanilla-stage-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	pack := filepath.Join(stage, "pack")
	if err := os.Mkdir(pack, 0755); err != nil {
		return err
	}
	if err := walkPack(ctx, source, func(relative string, info fs.FileInfo) error {
		return copyPackFile(ctx, filepath.Join(source, relative), filepath.Join(pack, relative), info)
	}); err != nil {
		return err
	}
	if err := writeVersion(pack, expected); err != nil {
		return err
	}
	if err := Validate(ctx, expected, pack); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("vanilla pack appeared during preparation at %q; retry startup", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(pack, output)
}

// copyPackFile copies a stable regular file with cancellation and bounded reads.
func copyPackFile(ctx context.Context, source, target string, info fs.FileInfo) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, input.Close()) }()
	opened, err := input.Stat()
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() != info.Size() {
		return fmt.Errorf("vanilla source changed during copying: %q", source)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	reader := io.LimitReader(input, info.Size()+1)
	buffer := make([]byte, 32<<10)
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := reader.Read(buffer)
		if n > 0 {
			written, err := output.Write(buffer[:n])
			if err != nil {
				return err
			}
			copied += int64(written)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if copied != info.Size() {
		return fmt.Errorf("vanilla source size changed during copying: %q", source)
	}
	return output.Sync()
}
