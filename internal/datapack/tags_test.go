package datapack

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeTag adds one tag file to a test datapack.
func writeTag(t *testing.T, root, name, data string) {
	t.Helper()
	filePath := filepath.Join(root, "data", "minecraft", "tags", "block", name+".json")
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestResolveTagLayers verifies nested tags, defaults, and pack replacement.
func TestResolveTagLayers(t *testing.T) {
	first := writeTestPack(t, `{}`)
	second := writeTestPack(t, `{}`)
	writeTag(t, first, "base", `{"values":["minecraft:stone","minecraft:dirt"]}`)
	writeTag(t, first, "group", `{"values":["#base","minecraft:gravel"]}`)
	writeTag(t, second, "group", `{"values":["minecraft:stone",{"id":"#missing","required":false}]}`)
	manager := &Manager{}
	if err := manager.Reload([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	got, err := manager.Current().ResolveTag("block", "minecraft:group")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"minecraft:stone", "minecraft:dirt", "minecraft:gravel"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("group = %v, want %v", got, want)
	}
	writeTag(t, second, "group", `{"replace":true,"values":["minecraft:deepslate"]}`)
	if err := manager.Reload([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	got, err = manager.Current().ResolveTag("block", "group")
	if err != nil || !reflect.DeepEqual(got, []string{"minecraft:deepslate"}) {
		t.Fatalf("replaced group = %v, error %v", got, err)
	}
}

// TestResolveTagMissingAndCycle verifies missing required references and cycles.
func TestResolveTagMissingAndCycle(t *testing.T) {
	root := writeTestPack(t, `{}`)
	writeTag(t, root, "missing_ref", `{"values":["#missing"]}`)
	writeTag(t, root, "cycle_a", `{"values":["#cycle_b"]}`)
	writeTag(t, root, "cycle_b", `{"values":["#cycle_a"]}`)
	manager := &Manager{}
	if err := manager.Reload([]string{root}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Current().ResolveTag("block", "missing_ref"); !errors.Is(err, ErrTagNotFound) {
		t.Fatalf("missing ref error = %v", err)
	}
	if _, err := manager.Current().ResolveTag("block", "cycle_a"); err == nil {
		t.Fatal("accepted cyclic tags")
	}
}
