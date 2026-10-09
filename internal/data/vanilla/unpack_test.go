package vanilla

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestUnpackExtractsOnlyServerAndLibraries checks developer inputs preserve Maven paths.
func TestUnpackExtractsOnlyServerAndLibraries(t *testing.T) {
	root := resourceRoot(t)
	inner := zipBytes(t, map[string][]byte{"version.json": []byte(`{"id":"1.21.4"}`)})
	jar := filepath.Join(root, "bundle.jar")
	data := zipBytes(t, map[string][]byte{
		"META-INF/versions/1.21.4/server-1.21.4.jar": inner,
		"META-INF/libraries/group/test/1.0/test.jar": []byte("fixture"),
	})
	if err := os.WriteFile(jar, data, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "unpacked")
	if err := Unpack(context.Background(), jar, "1.21.4", output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "libraries", "group", "test", "1.0", "test.jar")); err != nil {
		t.Fatal(err)
	}
	if err := Unpack(context.Background(), jar, "1.21.4", output); err == nil {
		t.Fatal("overwrote an existing destination")
	}
}

// TestUnpackRejectsUnsafeEntries prevents developer extraction from escaping its stage.
func TestUnpackRejectsUnsafeEntries(t *testing.T) {
	root := resourceRoot(t)
	inner := zipBytes(t, map[string][]byte{"version.json": []byte(`{"id":"1.21.4"}`)})
	jar := filepath.Join(root, "bundle.jar")
	data := zipBytes(t, map[string][]byte{
		"META-INF/versions/1.21.4/server-1.21.4.jar": inner,
		"META-INF/libraries/../../../escape":         []byte("unsafe"),
	})
	if err := os.WriteFile(jar, data, 0600); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "stage")
	if err := Unpack(context.Background(), jar, "1.21.4", stage); err == nil {
		t.Fatal("accepted traversal")
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("failed extraction left a partial directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatalf("escaped destination: %v", err)
	}
}
