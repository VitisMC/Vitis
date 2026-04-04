//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Version     string
	DataDir     string
	ProjectRoot string
}

func ParseConfig() Config {
	version := flag.String("version", "1.21.4", "Minecraft version")
	flag.Parse()

	projectRoot := findProjectRoot()
	dataDir := filepath.Join(projectRoot, ".mcdata", *version)

	return Config{
		Version:     *version,
		DataDir:     dataDir,
		ProjectRoot: projectRoot,
	}
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, _ := os.Getwd()
	return wd
}

func (c Config) InputFile(name string) string {
	return filepath.Join(c.DataDir, name)
}

func (c Config) DecompiledDir() string {
	return filepath.Join(c.ProjectRoot, ".mc-decompiled", c.Version+"-decompiled")
}

func (c Config) DecompiledFile(path string) string {
	return filepath.Join(c.DecompiledDir(), path)
}

func (c Config) OutputFile(subdir, name string) string {
	return filepath.Join(c.ProjectRoot, "internal", "data", "generated", subdir, name)
}

func (c Config) EnsureOutputDir(subdir string) error {
	dir := filepath.Join(c.ProjectRoot, "internal", "data", "generated", subdir)
	return os.MkdirAll(dir, 0755)
}

func (c Config) MustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}
	return data
}

func (c Config) MustWriteFile(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", filepath.Dir(path), err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
		os.Exit(1)
	}
}
