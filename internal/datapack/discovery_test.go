package datapack

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDiscoverOrdersWorldPacks verifies vanilla-first deterministic priority.
func TestDiscoverOrdersWorldPacks(t *testing.T) {
	root := t.TempDir()
	vanilla := filepath.Join(root, "vanilla")
	world := filepath.Join(root, "world")
	if err := os.MkdirAll(filepath.Join(world, "datapacks", "b_pack"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vanilla, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(world, "datapacks", "a_pack.zip"), []byte("zip"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(world, "datapacks", "ignored.txt"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(vanilla, world)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		vanilla,
		filepath.Join(world, "datapacks", "a_pack.zip"),
		filepath.Join(world, "datapacks", "b_pack"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
}

// TestDiscoverRejectsSymlink verifies pack discovery does not follow links.
func TestDiscoverRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	vanilla := filepath.Join(root, "vanilla")
	datapacks := filepath.Join(root, "world", "datapacks")
	if err := os.MkdirAll(vanilla, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(datapacks, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(vanilla, filepath.Join(datapacks, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(vanilla, filepath.Join(root, "world")); err == nil {
		t.Fatal("accepted linked datapack")
	}
}
