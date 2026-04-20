package data

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestBanListBanPardon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banned-players.json")
	bl := NewBanList(path)

	uuid := protocol.OfflinePlayerUUID("Griefer")
	if bl.IsBanned(uuid) {
		t.Fatal("expected not banned initially")
	}

	if err := bl.Ban(uuid, "Griefer", "griefing", "Server", "forever"); err != nil {
		t.Fatalf("ban failed: %v", err)
	}
	if !bl.IsBanned(uuid) {
		t.Fatal("expected banned")
	}
	if bl.GetBanReason(uuid) != "griefing" {
		t.Fatalf("expected reason 'griefing', got %q", bl.GetBanReason(uuid))
	}
	if bl.Count() != 1 {
		t.Fatalf("expected count 1, got %d", bl.Count())
	}

	removed, err := bl.Pardon("Griefer")
	if err != nil {
		t.Fatalf("pardon failed: %v", err)
	}
	if !removed {
		t.Fatal("expected removed")
	}
	if bl.IsBanned(uuid) {
		t.Fatal("expected not banned after pardon")
	}

	removed, err = bl.Pardon("Griefer")
	if err != nil {
		t.Fatalf("pardon missing failed: %v", err)
	}
	if removed {
		t.Fatal("expected not removed (not found)")
	}
}

func TestBanListExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banned-players.json")
	bl := NewBanList(path)

	uuid := protocol.OfflinePlayerUUID("TempBan")
	expires := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	if err := bl.Ban(uuid, "TempBan", "test", "Server", expires); err != nil {
		t.Fatalf("ban failed: %v", err)
	}

	if bl.IsBanned(uuid) {
		t.Fatal("expected expired ban to not count as banned")
	}
}

func TestBanListPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banned-players.json")

	bl := NewBanList(path)
	uuid := protocol.OfflinePlayerUUID("Banned")
	_ = bl.Ban(uuid, "Banned", "bad", "Server", "forever")

	bl2 := NewBanList(path)
	if err := bl2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !bl2.IsBanned(uuid) {
		t.Fatal("expected banned after reload")
	}
}

func TestIPBanListBanPardon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banned-ips.json")
	ibl := NewIPBanList(path)

	ip := "192.168.1.100"
	if ibl.IsBanned(ip) {
		t.Fatal("expected not banned initially")
	}

	if err := ibl.Ban(ip, "spam", "Server", "forever"); err != nil {
		t.Fatalf("ban failed: %v", err)
	}
	if !ibl.IsBanned(ip) {
		t.Fatal("expected banned")
	}
	if ibl.Count() != 1 {
		t.Fatalf("expected count 1, got %d", ibl.Count())
	}

	removed, err := ibl.Pardon(ip)
	if err != nil {
		t.Fatalf("pardon failed: %v", err)
	}
	if !removed {
		t.Fatal("expected removed")
	}
	if ibl.IsBanned(ip) {
		t.Fatal("expected not banned after pardon")
	}
}

func TestIPBanListExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banned-ips.json")
	ibl := NewIPBanList(path)

	ip := "10.0.0.1"
	expires := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	if err := ibl.Ban(ip, "test", "Server", expires); err != nil {
		t.Fatalf("ban failed: %v", err)
	}

	if ibl.IsBanned(ip) {
		t.Fatal("expected expired IP ban to not count as banned")
	}
}
