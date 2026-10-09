package vanilla

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// localFixture creates an old-style Vitis resource pack without downloading anything.
func localFixture(t *testing.T, directory string) Version {
	t.Helper()
	expected, bundle := runtimeFixture(t, nil)
	jar := filepath.Join(resourceRoot(t), "server.jar")
	if err := os.WriteFile(jar, bundle, 0600); err != nil {
		t.Fatal(err)
	}
	_, inner, err := inspectBundle(jar, expected.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractRuntimeData(context.Background(), inner, directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "pack.mcmeta"), []byte(`{"pack":{"description":"Vitis vanilla 1.21.4","pack_format":61}}`), 0644); err != nil {
		t.Fatal(err)
	}
	return expected
}

// TestPrepareWithLocalSource verifies offline migration, preservation and destination precedence.
func TestPrepareWithLocalSource(t *testing.T) {
	root := resourceRoot(t)
	source := filepath.Join(root, ".mcdata", "1.21.4", "vanilla")
	expected := localFixture(t, source)
	output := filepath.Join(root, "custom-world", "vanilla", expected.ID)
	before, err := os.ReadFile(filepath.Join(source, "pack.mcmeta"))
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareWithSource(context.Background(), expected, output, source, nil, "unused"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(source, "pack.mcmeta"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("source changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "version.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source identity was modified: %v", err)
	}
	if err := Validate(context.Background(), expected, output); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "pack.mcmeta"), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := prepareWithSource(context.Background(), expected, output, source, nil, "unused"); err != nil {
		t.Fatalf("existing destination did not take precedence: %v", err)
	}
}

// TestLocalPackRejectsUnsafeAndMismatchedSources checks errors leave both paths intact.
func TestLocalPackRejectsUnsafeAndMismatchedSources(t *testing.T) {
	for _, name := range []string{"checksum identity", "protocol", "data version", "format", "symlink", "unsafe name", "duplicate", "oversized", "damaged"} {
		t.Run(name, func(t *testing.T) {
			root := resourceRoot(t)
			source := filepath.Join(root, "old")
			expected := localFixture(t, source)
			switch name {
			case "checksum identity", "protocol", "data version", "format":
				wrong := expected
				switch name {
				case "checksum identity":
					wrong.ID = "1.21.5"
				case "protocol":
					wrong.Protocol--
				case "data version":
					wrong.WorldVersion--
				case "format":
					wrong.PackVersion.Data--
				}
				if err := writeVersion(source, wrong); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(source, "pack.mcmeta"), filepath.Join(source, "linked")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "unsafe name":
				if err := os.WriteFile(filepath.Join(source, "CON"), []byte("keep"), 0644); err != nil {
					t.Skipf("unsafe filename unavailable: %v", err)
				}
			case "duplicate":
				if err := os.WriteFile(filepath.Join(source, "PACK.MCMETA"), []byte("keep"), 0644); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				file, err := os.Create(filepath.Join(source, "large"))
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Truncate(maxResourceSize + 1); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			case "damaged":
				if err := os.WriteFile(filepath.Join(source, "data", "minecraft", "recipe", "test.json"), []byte("broken"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(root, "world", "vanilla", expected.ID)
			err := prepareWithSource(context.Background(), expected, output, source, nil, "unused")
			if err == nil || !strings.Contains(err.Error(), "preserved") {
				t.Fatalf("error = %v", err)
			}
			assertNoPublishedPack(t, output)
			if _, err := os.Stat(filepath.Join(source, "pack.mcmeta")); err != nil {
				t.Fatalf("source removed: %v", err)
			}
		})
	}
}

// TestLocalMigrationCancellationAndRetry checks cancellation never publishes a partial pack.
func TestLocalMigrationCancellationAndRetry(t *testing.T) {
	root := resourceRoot(t)
	source := filepath.Join(root, "old")
	expected := localFixture(t, source)
	output := filepath.Join(root, "world", "vanilla", expected.ID)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := prepareWithSource(ctx, expected, output, source, nil, "unused"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	assertNoPublishedPack(t, output)
	if err := prepareWithSource(context.Background(), expected, output, source, nil, "unused"); err != nil {
		t.Fatal(err)
	}
	identity, err := os.ReadFile(filepath.Join(output, "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual Version
	if err := json.Unmarshal(identity, &actual); err != nil || actual != expected {
		t.Fatalf("identity = %+v, error = %v", actual, err)
	}
}

// migrationStopContext cancels as soon as a resource is opened in the private stage.
type migrationStopContext struct {
	context.Context
	cancel   context.CancelFunc
	pattern  string
	observed bool
}

// Err simulates a stop signal during local copying without timing-dependent goroutines.
func (c *migrationStopContext) Err() error {
	paths, err := filepath.Glob(c.pattern)
	if err == nil && len(paths) > 0 {
		c.observed = true
		c.cancel()
	}
	return c.Context.Err()
}

// TestLocalMigrationStopsDuringCopy removes staged files and permits a subsequent retry.
func TestLocalMigrationStopsDuringCopy(t *testing.T) {
	root := resourceRoot(t)
	source := filepath.Join(root, "old")
	expected := localFixture(t, source)
	output := filepath.Join(root, "world", "vanilla", expected.ID)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &migrationStopContext{Context: parent, cancel: cancel, pattern: filepath.Join(filepath.Dir(output), ".vanilla-stage-*", "pack", "data", "minecraft", "advancement", "test.json")}
	if err := prepareWithSource(ctx, expected, output, source, nil, "unused"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if !ctx.observed {
		t.Fatal("cancellation did not happen during staging")
	}
	assertNoPublishedPack(t, output)
	if err := prepareWithSource(context.Background(), expected, output, source, nil, "unused"); err != nil {
		t.Fatal(err)
	}
}
