package vanilla

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestInspectBundle verifies nested server metadata and version matching.
func TestInspectBundle(t *testing.T) {
	inner := zipBytes(t, map[string][]byte{
		"version.json":                      []byte(`{"id":"1.21.4","world_version":4189,"protocol_version":769,"pack_version":{"data":61}}`),
		"data/minecraft/structure/test.nbt": []byte{1, 2, 3},
	})
	outer := zipBytes(t, map[string][]byte{
		"META-INF/versions/1.21.4/server-1.21.4.jar": inner,
	})
	filePath := filepath.Join(t.TempDir(), "server.jar")
	if err := os.WriteFile(filePath, outer, 0600); err != nil {
		t.Fatal(err)
	}
	version, nested, err := inspectBundle(filePath, "1.21.4")
	if err != nil {
		t.Fatal(err)
	}
	if version.ID != "1.21.4" || version.Protocol != 769 || version.PackVersion.Data != 61 {
		t.Fatalf("unexpected metadata: %+v", version)
	}
	if !bytes.Equal(nested, inner) {
		t.Fatal("nested JAR differs from source")
	}
	if _, _, err := inspectBundle(filePath, "1.21.5"); err == nil {
		t.Fatal("accepted a missing nested version")
	}
}

// TestExtractMissingData preserves generated files and extracts missing assets.
func TestExtractMissingData(t *testing.T) {
	inner := zipBytes(t, map[string][]byte{
		"data/minecraft/structure/test.nbt": []byte{1, 2, 3},
		"data/minecraft/recipe/test.json":   []byte(`{"jar":true}`),
		"version.json":                      []byte(`{}`),
	})
	root := t.TempDir()
	recipePath := filepath.Join(root, "data", "minecraft", "recipe", "test.json")
	if err := os.MkdirAll(filepath.Dir(recipePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recipePath, []byte(`{"generated":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extractMissingData(inner, root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(recipePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"generated":true}` {
		t.Fatalf("generated resource overwritten: %s", got)
	}
	got, err = os.ReadFile(filepath.Join(root, "data", "minecraft", "structure", "test.nbt"))
	if err != nil || !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("structure = %v, error %v", got, err)
	}
}

// TestReplaceDirectory verifies publish and replacement behavior.
func TestReplaceDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	for _, content := range []string{"first", "second"} {
		source, err := os.MkdirTemp(root, "source-")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "value"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := replaceDirectory(source, target); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(target, "value"))
		if err != nil || string(got) != content {
			t.Fatalf("published value = %q, error %v", got, err)
		}
	}
}

// TestValidVersion rejects filesystem paths and accepts release identifiers.
func TestValidVersion(t *testing.T) {
	for _, value := range []string{"1.21.4", "24w46a", "1.21.5-pre1"} {
		if !validVersion(value) {
			t.Fatalf("rejected version %q", value)
		}
	}
	for _, value := range []string{"", ".", "..", "../1.21.4", "1.21/4", "1.21 4"} {
		if validVersion(value) {
			t.Fatalf("accepted invalid version %q", value)
		}
	}
}

// zipBytes creates a deterministic ZIP archive for tests.
func zipBytes(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, data := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
