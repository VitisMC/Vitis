package vanilla

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/vitismc/vitis/internal/datapack"
)

const (
	manifestURL        = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
	maxMetadataSize    = 4 << 20
	maxServerSize      = 128 << 20
	maxResourceSize    = 32 << 20
	maxResourceTotal   = 512 << 20
	maxResourceEntries = 65536
)

var runtimeDirectories = []string{"advancement", "loot_table", "recipe", "tags", "worldgen", "structure"}

// Prepare installs missing runtime resources without Java and preserves existing packs.
func Prepare(ctx context.Context, expected Version, output string) error {
	return PrepareWithSource(ctx, expected, output, "")
}

// PrepareWithSource installs a missing pack from a preserved local source or Mojang.
func PrepareWithSource(ctx context.Context, expected Version, output, source string) error {
	if ctx == nil {
		return fmt.Errorf("prepare vanilla resources: nil context")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Minute, CheckRedirect: secureRedirect}
	return prepareWithSource(ctx, expected, output, source, client, manifestURL)
}

// prepare validates existing resources or downloads and stages a missing pack.
func prepare(ctx context.Context, expected Version, output string, client *http.Client, manifest string) (err error) {
	return prepareWithSource(ctx, expected, output, "", client, manifest)
}

// prepareWithSource validates, stages and publishes resources without replacing existing data.
func prepareWithSource(ctx context.Context, expected Version, output, source string, client *http.Client, manifest string) (err error) {
	if !validVersion(expected.ID) || expected.Protocol <= 0 || expected.WorldVersion <= 0 || expected.PackVersion.Data <= 0 {
		return fmt.Errorf("invalid expected Minecraft version")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(output); err == nil {
		if err := Validate(ctx, expected, output); err != nil {
			return fmt.Errorf("existing vanilla pack %q is invalid (preserved): %w", output, err)
		}
		return ctx.Err()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if source != "" {
		if _, err := os.Lstat(source); err == nil {
			return copyRuntimePack(ctx, expected, source, output)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
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
	jar := filepath.Join(stage, "server.jar")
	if err := downloadServer(ctx, client, manifest, expected.ID, jar); err != nil {
		return err
	}
	metadata, inner, err := inspectBundle(jar, expected.ID)
	if err != nil {
		return err
	}
	if metadata != expected {
		return fmt.Errorf("server JAR version mismatch: got %+v, expected %+v", metadata, expected)
	}
	pack := filepath.Join(stage, "pack")
	if err := extractRuntimeData(ctx, inner, pack); err != nil {
		return err
	}
	manifestData, err := json.Marshal(map[string]any{"pack": map[string]any{
		"description": "Vitis vanilla " + expected.ID, "pack_format": expected.PackVersion.Data,
	}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), append(manifestData, '\n'), 0644); err != nil {
		return err
	}
	if err := writeVersion(pack, metadata); err != nil {
		return err
	}
	if err := Validate(ctx, expected, pack); err != nil {
		return fmt.Errorf("validate extracted vanilla pack: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Never replace a pack created by another process during preparation.
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("vanilla pack appeared during preparation at %q; retry startup", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(pack, output)
}

// extractRuntimeData copies bounded, regular resources into a new pack directory.
func extractRuntimeData(ctx context.Context, inner []byte, output string) error {
	reader, err := zip.NewReader(bytes.NewReader(inner), int64(len(inner)))
	if err != nil {
		return err
	}
	if len(reader.File) > maxResourceEntries {
		return fmt.Errorf("server JAR exceeds entry limit")
	}
	if err := os.Mkdir(output, 0755); err != nil {
		return err
	}
	seen := make(map[string]bool)
	var total uint64
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(file.Name, "/")
		if !portableArchivePath(name) {
			return fmt.Errorf("unsafe server archive path %q", file.Name)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate server archive entry %q", file.Name)
		}
		seen[key] = true
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("server archive symlink %q", file.Name)
		}
		if file.FileInfo().IsDir() || !strings.HasPrefix(name, "data/") || name == "data/.mcassetsroot" {
			continue
		}
		if !file.Mode().IsRegular() {
			return fmt.Errorf("non-regular server resource %q", name)
		}
		if file.UncompressedSize64 > maxResourceSize || total+file.UncompressedSize64 > maxResourceTotal {
			return fmt.Errorf("server resources exceed size limit")
		}
		total += file.UncompressedSize64
		content, err := readZipEntry(file, maxResourceSize)
		if err != nil {
			return err
		}
		target := filepath.Join(output, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return err
		}
	}
	return nil
}

// portableArchivePath rejects traversal and paths that alias special Windows files.
func portableArchivePath(name string) bool {
	if name == "" || path.Clean(name) != name || strings.ContainsAny(name, "\\:<>\"|?*") || strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") {
		return false
	}
	for _, char := range name {
		if char < 32 {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
			return false
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return false
		}
	}
	return true
}

// validateRuntimePack verifies required resource categories and pack contents.
func validateRuntimePack(output string) error {
	for _, name := range runtimeDirectories {
		directory := filepath.Join(output, "data", "minecraft", name)
		info, err := os.Lstat(directory)
		if err != nil {
			return fmt.Errorf("missing vanilla resources %s: %w", name, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("vanilla resources %s must be a directory", name)
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return fmt.Errorf("vanilla resources %s are empty", name)
		}
	}
	validator := &datapack.Manager{}
	return validator.Reload([]string{output})
}

// makeResourceDirectory creates parent directories without following symlinks.
func makeResourceDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("resource directory %q is not a regular directory", directory)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(directory)
	if parent != directory {
		if err := makeResourceDirectory(parent); err != nil {
			return err
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(directory, 0755); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err = os.Lstat(directory)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("resource directory %q is not a regular directory", directory)
		}
	}
	return nil
}
