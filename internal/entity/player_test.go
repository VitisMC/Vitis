package entity

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestPlayerVisibleChunkKeysReusesStorage(t *testing.T) {
	p := NewPlayer(1, protocol.UUID{}, "player", Vec3{}, Vec2{}, nil, 2)
	first := p.VisibleChunkKeys()
	if len(first) == 0 {
		t.Fatal("expected visible chunks")
	}
	firstPtr := &first[0]
	second := p.VisibleChunkKeys()
	if &second[0] != firstPtr {
		t.Fatal("visible chunk keys allocated new backing storage")
	}
}

func TestPlayerClientLoadGateHasBoundedFallback(t *testing.T) {
	p := NewPlayer(1, protocol.UUID{}, "player", Vec3{}, Vec2{}, nil, 2)
	if p.HasClientLoaded() {
		t.Fatal("new player bypassed client load gate")
	}
	for range 59 {
		p.TickClientLoadTimeout()
	}
	if p.HasClientLoaded() {
		t.Fatal("client load gate expired early")
	}
	p.TickClientLoadTimeout()
	if !p.HasClientLoaded() {
		t.Fatal("client load fallback did not expire")
	}
	p.SetClientLoaded(false)
	if p.HasClientLoaded() {
		t.Fatal("reset did not restore client load gate")
	}
	p.SetClientLoaded(true)
	if !p.HasClientLoaded() {
		t.Fatal("PlayerLoaded did not release client load gate")
	}
}
