package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestWhitelistAddRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "whitelist.json")
	wl := NewWhitelist(path)

	uuid := protocol.OfflinePlayerUUID("Steve")
	added, err := wl.Add(uuid, "Steve")
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !added {
		t.Fatal("expected newly added")
	}
	if !wl.IsWhitelisted(uuid) {
		t.Fatal("expected whitelisted")
	}
	if wl.Count() != 1 {
		t.Fatalf("expected count 1, got %d", wl.Count())
	}

	added, err = wl.Add(uuid, "Steve")
	if err != nil {
		t.Fatalf("add duplicate failed: %v", err)
	}
	if added {
		t.Fatal("expected not added (duplicate)")
	}

	removed, err := wl.RemoveByName("Steve")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if !removed {
		t.Fatal("expected removed")
	}
	if wl.IsWhitelisted(uuid) {
		t.Fatal("expected not whitelisted after remove")
	}

	removed, err = wl.RemoveByName("Steve")
	if err != nil {
		t.Fatalf("remove missing failed: %v", err)
	}
	if removed {
		t.Fatal("expected not removed (not found)")
	}
}

func TestWhitelistPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "whitelist.json")

	wl := NewWhitelist(path)
	uuid := protocol.OfflinePlayerUUID("Alex")
	_, _ = wl.Add(uuid, "Alex")

	wl2 := NewWhitelist(path)
	if err := wl2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !wl2.IsWhitelisted(uuid) {
		t.Fatal("expected whitelisted after reload")
	}
	if wl2.Count() != 1 {
		t.Fatalf("expected count 1, got %d", wl2.Count())
	}
}

func TestWhitelistLoadMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")
	wl := NewWhitelist(path)
	if err := wl.Load(); err != nil {
		t.Fatalf("load missing file should not error: %v", err)
	}
	if wl.Count() != 0 {
		t.Fatalf("expected count 0, got %d", wl.Count())
	}
}

func TestWhitelistEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "whitelist.json")
	wl := NewWhitelist(path)

	if wl.Enabled() {
		t.Fatal("expected disabled by default")
	}
	wl.SetEnabled(true)
	if !wl.Enabled() {
		t.Fatal("expected enabled")
	}
}

func TestWhitelistNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "whitelist.json")
	wl := NewWhitelist(path)

	_, _ = wl.Add(protocol.OfflinePlayerUUID("A"), "A")
	_, _ = wl.Add(protocol.OfflinePlayerUUID("B"), "B")

	names := wl.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}

	_ = os.Remove(path)
}
