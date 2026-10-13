// Package datapack loads versioned server resources from folders and ZIP packs.
package datapack

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	// CurrentPackFormat is the data pack format supported by this server build.
	CurrentPackFormat = 61
	maxFileSize       = 32 << 20
	maxPackSize       = 512 << 20
	maxPackEntries    = 65536
)

// Resource identifies one resource at one pack priority.
type Resource struct {
	Pack string
	Data []byte
}

// Snapshot is a stable, immutable view of all loaded packs.
type Snapshot struct {
	resources map[string][]Resource
	packs     []string
	compiled  map[string]any
}

// Packs returns pack paths in increasing priority order.
func (s *Snapshot) Packs() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.packs...)
}

// Get returns the highest-priority resource at a canonical data path.
func (s *Snapshot) Get(name string) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	layers := s.resources[name]
	if len(layers) == 0 {
		return nil, false
	}
	return append([]byte(nil), layers[len(layers)-1].Data...), true
}

// Layers returns all definitions of a resource in pack priority order.
func (s *Snapshot) Layers(name string) []Resource {
	if s == nil {
		return nil
	}
	layers := s.resources[name]
	result := make([]Resource, len(layers))
	for i, layer := range layers {
		result[i] = Resource{Pack: layer.Pack, Data: append([]byte(nil), layer.Data...)}
	}
	return result
}

// Names returns the canonical paths of all resources.
func (s *Snapshot) Names() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.resources))
	for name := range s.resources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Compiled returns an immutable derived resource set produced during reload.
func (s *Snapshot) Compiled(name string) (any, bool) {
	if s == nil {
		return nil, false
	}
	value, ok := s.compiled[name]
	return value, ok
}

// Compiler validates and compiles one subsystem before a snapshot is published.
type Compiler func(*Snapshot) (any, error)

// Manager publishes resource snapshots without exposing partially loaded packs.
type Manager struct {
	current   atomic.Pointer[Snapshot]
	mu        sync.RWMutex
	reloadMu  sync.Mutex
	compilers map[string]Compiler
}

// RegisterCompiler adds a named resource compiler before the first reload.
func (m *Manager) RegisterCompiler(name string, compiler Compiler) error {
	if m == nil {
		return fmt.Errorf("register resource compiler: nil manager")
	}
	if name == "" || compiler == nil {
		return fmt.Errorf("register resource compiler: invalid compiler")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current.Load() != nil {
		return fmt.Errorf("register resource compiler %q after initial load", name)
	}
	if m.compilers == nil {
		m.compilers = make(map[string]Compiler)
	}
	if _, exists := m.compilers[name]; exists {
		return fmt.Errorf("register resource compiler %q twice", name)
	}
	m.compilers[name] = compiler
	return nil
}

// Current returns the last successfully loaded snapshot.
func (m *Manager) Current() *Snapshot {
	if m == nil {
		return nil
	}
	return m.current.Load()
}

// Reload loads each pack before atomically publishing the new snapshot.
func (m *Manager) Reload(paths []string) error {
	if m == nil {
		return fmt.Errorf("reload datapacks: nil manager")
	}
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	m.mu.RLock()
	compilers := make(map[string]Compiler, len(m.compilers))
	for name, compiler := range m.compilers {
		compilers[name] = compiler
	}
	m.mu.RUnlock()
	snapshot := &Snapshot{
		resources: make(map[string][]Resource),
		packs:     append([]string(nil), paths...),
		compiled:  make(map[string]any, len(compilers)),
	}
	for _, packPath := range paths {
		files, err := readPack(packPath)
		if err != nil {
			return fmt.Errorf("load datapack %q: %w", packPath, err)
		}
		if err := validateMetadata(files["pack.mcmeta"]); err != nil {
			return fmt.Errorf("load datapack %q: %w", packPath, err)
		}
		for name, data := range files {
			if !strings.HasPrefix(name, "data/") {
				continue
			}
			if name == "data/.mcassetsroot" {
				continue
			}
			if err := validateResourcePath(name); err != nil {
				return fmt.Errorf("load datapack %q: %w", packPath, err)
			}
			if strings.HasSuffix(name, ".json") && !json.Valid(data) {
				return fmt.Errorf("load datapack %q: invalid JSON in %q", packPath, name)
			}
			snapshot.resources[name] = append(snapshot.resources[name], Resource{Pack: packPath, Data: data})
		}
	}
	compilerNames := make([]string, 0, len(compilers))
	for name := range compilers {
		compilerNames = append(compilerNames, name)
	}
	sort.Strings(compilerNames)
	for _, name := range compilerNames {
		compiled, err := compilers[name](snapshot)
		if err != nil {
			return fmt.Errorf("compile server resources %q: %w", name, err)
		}
		if compiled == nil {
			return fmt.Errorf("compile server resources %q: nil result", name)
		}
		snapshot.compiled[name] = compiled
	}
	m.current.Store(snapshot)
	return nil
}

// readPack reads a folder or ZIP without following symlinks or accepting duplicates.
func readPack(packPath string) (map[string][]byte, error) {
	info, err := os.Lstat(packPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("pack path is a symlink")
	}
	if info.IsDir() {
		return readDirectory(packPath)
	}
	if info.Mode().IsRegular() && strings.EqualFold(filepath.Ext(packPath), ".zip") {
		return readZip(packPath)
	}
	return nil, fmt.Errorf("pack must be a directory or ZIP file")
}

// readDirectory reads all regular files in a pack directory.
func readDirectory(root string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	var total int64
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in pack: %s", filePath)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular file in pack: %s", filePath)
		}
		name, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if err := validatePackPath(name); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := addSize(&total, info.Size(), len(files)); err != nil {
			return err
		}
		stream, err := os.Open(filePath)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, maxFileSize+1))
		closeErr := stream.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) > maxFileSize {
			return fmt.Errorf("pack file %q exceeds size limit", name)
		}
		files[name] = data
		return nil
	})
	return files, err
}

// readZip reads a ZIP pack while enforcing expanded-size and path limits.
func readZip(packPath string) (map[string][]byte, error) {
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	files := make(map[string][]byte)
	var total int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if !file.Mode().IsRegular() {
			return nil, fmt.Errorf("non-regular ZIP entry %q", file.Name)
		}
		if err := validatePackPath(file.Name); err != nil {
			return nil, err
		}
		if _, exists := files[file.Name]; exists {
			return nil, fmt.Errorf("duplicate ZIP entry %q", file.Name)
		}
		if file.UncompressedSize64 > maxFileSize {
			return nil, fmt.Errorf("ZIP entry %q exceeds size limit", file.Name)
		}
		if err := addSize(&total, int64(file.UncompressedSize64), len(files)); err != nil {
			return nil, err
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, maxFileSize+1))
		closeErr := stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(data) > maxFileSize {
			return nil, fmt.Errorf("ZIP entry %q exceeds size limit", file.Name)
		}
		files[file.Name] = data
	}
	return files, nil
}

// addSize enforces pack-wide resource limits.
func addSize(total *int64, size int64, entries int) error {
	if size < 0 || size > maxFileSize {
		return fmt.Errorf("pack file exceeds size limit")
	}
	if entries >= maxPackEntries || *total > maxPackSize-size {
		return fmt.Errorf("pack exceeds size or entry limit")
	}
	*total += size
	return nil
}

// validatePackPath rejects absolute and escaping paths.
func validatePackPath(name string) error {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return fmt.Errorf("invalid pack path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." || part == "" {
			return fmt.Errorf("invalid pack path %q", name)
		}
	}
	return nil
}

// validateResourcePath checks the namespace and resource segments.
func validateResourcePath(name string) error {
	parts := strings.Split(name, "/")
	if len(parts) < 4 || parts[0] != "data" || !validIdentifier(parts[1], false) {
		return fmt.Errorf("invalid resource path %q", name)
	}
	for _, part := range parts[2:] {
		if !validIdentifier(part, true) {
			return fmt.Errorf("invalid resource path %q", name)
		}
	}
	return nil
}

// validIdentifier checks one canonical namespace or path segment.
func validIdentifier(value string, allowDot bool) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-' || allowDot && char == '.' {
			continue
		}
		return false
	}
	return true
}

// validateMetadata verifies that a pack supports the server data format.
func validateMetadata(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("missing pack.mcmeta")
	}
	var metadata struct {
		Pack struct {
			Format           int             `json:"pack_format"`
			SupportedFormats json.RawMessage `json:"supported_formats"`
		} `json:"pack"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("invalid pack.mcmeta: %w", err)
	}
	if metadata.Pack.Format <= 0 {
		return fmt.Errorf("pack.mcmeta has no valid pack_format")
	}
	minimum, maximum := metadata.Pack.Format, metadata.Pack.Format
	if len(metadata.Pack.SupportedFormats) > 0 {
		var single int
		var bounds []int
		if err := json.Unmarshal(metadata.Pack.SupportedFormats, &single); err == nil {
			minimum, maximum = single, single
		} else if err := json.Unmarshal(metadata.Pack.SupportedFormats, &bounds); err == nil && len(bounds) == 2 {
			minimum, maximum = bounds[0], bounds[1]
		} else {
			var named struct {
				Min int `json:"min_inclusive"`
				Max int `json:"max_inclusive"`
			}
			if err := json.Unmarshal(metadata.Pack.SupportedFormats, &named); err != nil {
				return fmt.Errorf("invalid supported_formats: %w", err)
			}
			minimum, maximum = named.Min, named.Max
		}
	}
	if minimum <= 0 || maximum < minimum || CurrentPackFormat < minimum || CurrentPackFormat > maximum {
		return fmt.Errorf("unsupported datapack format %d-%d, expected %d", minimum, maximum, CurrentPackFormat)
	}
	return nil
}
