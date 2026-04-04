package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Loader loads and validates configuration from a YAML file path.
type Loader struct {
	path string
}

// NewLoader creates a new Loader bound to path.
func NewLoader(path string) *Loader {
	if path == "" {
		path = DefaultConfigPath
	}
	return &Loader{path: path}
}

// Path returns the source path used for loading and reloading.
func (l *Loader) Path() string {
	if l == nil {
		return DefaultConfigPath
	}
	return l.path
}

// Load reads, decodes, applies defaults, and validates the current loader path.
func (l *Loader) Load() (*Config, error) {
	if l == nil {
		return nil, fmt.Errorf("load config: nil loader")
	}

	raw, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("load config from %q: %w", l.path, err)
	}

	cfg := Default()
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil && err != io.EOF {
		return nil, fmt.Errorf("decode config from %q: %w", l.path, err)
	}

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config from %q: %w", l.path, err)
	}

	return cfg, nil
}

// Reload re-reads and validates configuration from the same loader path.
func (l *Loader) Reload() (*Config, error) {
	return l.Load()
}

// Load reads, decodes, applies defaults, and validates configuration from path.
func Load(path string) (*Config, error) {
	return NewLoader(path).Load()
}
