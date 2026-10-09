// Package vanilla prepares a complete, versioned server resource pack.
package vanilla

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/vitismc/vitis/internal/datapack"
)

const maxArchiveEntry = 64 << 20

// Version describes the identity and pack format of a server JAR.
type Version struct {
	ID           string `json:"id"`
	WorldVersion int32  `json:"world_version"`
	Protocol     int32  `json:"protocol_version"`
	PackVersion  struct {
		Data int `json:"data"`
	} `json:"pack_version"`
}

// Generate creates a complete pack from a server JAR and replaces output atomically.
func Generate(ctx context.Context, serverJAR, versionID, output string) error {
	if !validVersion(versionID) {
		return fmt.Errorf("invalid version %q", versionID)
	}
	if ctx == nil {
		return fmt.Errorf("generate vanilla data: nil context")
	}
	metadata, innerJAR, err := inspectBundle(serverJAR, versionID)
	if err != nil {
		return err
	}
	if metadata.ID != versionID || metadata.PackVersion.Data <= 0 || metadata.Protocol <= 0 {
		return fmt.Errorf("server JAR has invalid version metadata for %q", versionID)
	}
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".vanilla-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	packDir := filepath.Join(stage, "pack")
	command := exec.CommandContext(ctx, "java", "-DbundlerMainClass=net.minecraft.data.Main", "-jar", serverJAR, "--server", "--reports", "--output", packDir)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run data generator: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(packDir, ".cache")); err != nil {
		return fmt.Errorf("remove generator cache: %w", err)
	}
	if err := extractMissingData(innerJAR, packDir); err != nil {
		return err
	}
	manifest, err := json.Marshal(struct {
		Pack struct {
			Description string `json:"description"`
			Format      int    `json:"pack_format"`
		} `json:"pack"`
	}{Pack: struct {
		Description string `json:"description"`
		Format      int    `json:"pack_format"`
	}{Description: "Vitis vanilla " + versionID, Format: metadata.PackVersion.Data}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(packDir, "pack.mcmeta"), append(manifest, '\n'), 0644); err != nil {
		return err
	}
	if err := writeVersion(packDir, metadata); err != nil {
		return err
	}
	for _, required := range []string{
		"reports/registries.json",
		"reports/blocks.json",
		"reports/items.json",
		"reports/packets.json",
		"data/minecraft/advancement",
		"data/minecraft/loot_table",
		"data/minecraft/recipe",
		"data/minecraft/tags",
		"data/minecraft/worldgen",
		"data/minecraft/structure",
	} {
		if _, err := os.Stat(filepath.Join(packDir, filepath.FromSlash(required))); err != nil {
			return fmt.Errorf("generated pack missing %s: %w", required, err)
		}
	}
	validator := &datapack.Manager{}
	if err := validator.Reload([]string{packDir}); err != nil {
		return fmt.Errorf("validate generated resource pack: %w", err)
	}
	if err := replaceDirectory(packDir, output); err != nil {
		return err
	}
	return nil
}

// inspectBundle verifies the nested JAR and reads its version metadata.
func inspectBundle(serverJAR, versionID string) (Version, []byte, error) {
	outer, err := zip.OpenReader(serverJAR)
	if err != nil {
		return Version{}, nil, fmt.Errorf("open server JAR: %w", err)
	}
	defer outer.Close()
	name := "META-INF/versions/" + versionID + "/server-" + versionID + ".jar"
	file, err := uniqueZipEntry(outer.File, name)
	if err != nil {
		return Version{}, nil, err
	}
	data, err := readZipEntry(file, 128<<20)
	if err != nil {
		return Version{}, nil, err
	}
	inner, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Version{}, nil, fmt.Errorf("open nested server JAR: %w", err)
	}
	entry, err := uniqueZipEntry(inner.File, "version.json")
	if err != nil {
		return Version{}, nil, err
	}
	versionData, err := readZipEntry(entry, 1<<20)
	if err != nil {
		return Version{}, nil, err
	}
	var version Version
	if err := json.Unmarshal(versionData, &version); err != nil {
		return Version{}, nil, fmt.Errorf("parse version.json: %w", err)
	}
	return version, data, nil
}

// uniqueZipEntry selects one bounded, regular archive entry without ambiguous duplicates.
func uniqueZipEntry(files []*zip.File, name string) (*zip.File, error) {
	if len(files) > maxResourceEntries {
		return nil, fmt.Errorf("server archive exceeds entry limit")
	}
	var found *zip.File
	for _, file := range files {
		if file.Name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("duplicate server archive entry %q", name)
		}
		if !file.Mode().IsRegular() {
			return nil, fmt.Errorf("non-regular server archive entry %q", name)
		}
		found = file
	}
	if found == nil {
		return nil, fmt.Errorf("server archive has no %s", name)
	}
	return found, nil
}

// extractMissingData adds embedded resources absent from generated output.
func extractMissingData(innerJAR []byte, output string) error {
	reader, err := zip.NewReader(bytes.NewReader(innerJAR), int64(len(innerJAR)))
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasPrefix(file.Name, "data/") {
			continue
		}
		if path.Clean(file.Name) != file.Name || strings.Contains(file.Name, "\\") || strings.Contains(file.Name, "/../") || strings.HasPrefix(file.Name, "/") {
			return fmt.Errorf("unsafe server resource path %q", file.Name)
		}
		if !file.Mode().IsRegular() {
			return fmt.Errorf("non-regular server resource %q", file.Name)
		}
		target := filepath.Join(output, filepath.FromSlash(file.Name))
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		content, err := readZipEntry(file, maxArchiveEntry)
		if err != nil {
			return fmt.Errorf("read %q: %w", file.Name, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return err
		}
	}
	return nil
}

// readZipEntry reads one bounded ZIP entry and verifies its checksum.
func readZipEntry(file *zip.File, limit int64) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("archive entry %q exceeds size limit", file.Name)
	}
	stream, err := file.Open()
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(stream, limit+1))
	closeErr := stream.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("archive entry %q exceeds size limit", file.Name)
	}
	return content, nil
}

// replaceDirectory publishes a generated directory or restores the old one.
func replaceDirectory(source, target string) error {
	backup := target + ".previous"
	if _, err := os.Stat(backup); err == nil {
		return fmt.Errorf("previous backup exists at %q", backup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	oldExists := false
	if _, err := os.Stat(target); err == nil {
		oldExists = true
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		if oldExists {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore previous data: %w", restoreErr))
			}
		}
		return err
	}
	if oldExists {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove previous data: %w", err)
		}
	}
	return nil
}

// validVersion accepts release and snapshot IDs without allowing filesystem paths.
func validVersion(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}
