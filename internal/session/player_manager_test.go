package session

import (
	"errors"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestPlayerManagerRejectsDuplicateAndChecksCleanupOwner(t *testing.T) {
	manager := NewPlayerManager()
	uuid := protocol.UUID{1, 2}
	oldSession, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	newSession, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	defer oldSession.Close()
	defer newSession.Close()

	oldPlayer := &OnlinePlayer{Session: oldSession, UUID: uuid, Name: "old"}
	if err := manager.AddPlayer(oldPlayer); err != nil {
		t.Fatal(err)
	}
	if err := manager.AddPlayer(&OnlinePlayer{Session: newSession, UUID: uuid, Name: "new"}); !errors.Is(err, ErrPlayerAlreadyOnline) {
		t.Fatalf("duplicate admission error = %v", err)
	}

	if !manager.RemovePlayer(uuid, oldSession) {
		t.Fatal("old owner failed to remove its player")
	}
	newPlayer := &OnlinePlayer{Session: newSession, UUID: uuid, Name: "new"}
	if err := manager.AddPlayer(newPlayer); err != nil {
		t.Fatal(err)
	}
	if manager.RemovePlayer(uuid, oldSession) {
		t.Fatal("stale owner removed the replacement session")
	}
	if got := manager.GetByUUID(uuid); got != newPlayer {
		t.Fatal("replacement session was removed by stale cleanup")
	}
}

func TestPlayerManagerEnforcesPlayAdmissionLimit(t *testing.T) {
	manager := NewPlayerManager()
	manager.SetMaxPlayers(1)
	first, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(Config{Connection: newMockConn(), Registry: protocol.NewRegistry()})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	defer second.Close()

	if err := manager.AddPlayer(&OnlinePlayer{Session: first, UUID: protocol.UUID{1, 1}, Name: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.AddPlayer(&OnlinePlayer{Session: second, UUID: protocol.UUID{2, 2}, Name: "second"}); !errors.Is(err, ErrPlayerLimitReached) {
		t.Fatalf("expected player limit error, got %v", err)
	}
	if manager.Count() != 1 {
		t.Fatalf("player count = %d, want 1", manager.Count())
	}
}
