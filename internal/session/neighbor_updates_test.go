package session

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TestDigPacketUpdatesCropSupportInItsDimension exercises the registered packet and world tick.
func TestDigPacketUpdatesCropSupportInItsDimension(t *testing.T) {
	f := newDimensionFixture(t)
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	crop := block.DefaultStateID("minecraft:wheat")
	farmland := block.DefaultStateID("minecraft:farmland")
	for i, runtime := range f.runtime {
		if err := runtime.SetGameRule("randomTickSpeed", "0"); err != nil {
			t.Fatal(err)
		}
		f.worlds[i].Accessor.SetBlock(9, 63, 8, farmland)
		f.worlds[i].Accessor.SetBlock(9, 64, 8, crop)
	}
	f.sessions[0].Player().(*entity.Player).Living().SetGameMode(1)
	packet := &play.BlockDig{Status: play.DigStarted, Position: play.BlockPos{X: 9, Y: 63, Z: 8}, Face: 1, Sequence: 1}
	if err := router.Handle(f.sessions[0], protocol.StatePlay, packet); err != nil {
		t.Fatal(err)
	}
	if f.worlds[0].Accessor.GetBlock(9, 63, 8) != farmland || f.runtime[0].ItemEntities().Count() != 0 {
		t.Fatal("network handler mutated block support")
	}
	f.runtime[0].Tick()
	if f.worlds[0].Accessor.GetBlock(9, 63, 8) != 0 || f.worlds[0].Accessor.GetBlock(9, 64, 8) != 0 || f.runtime[0].ItemEntities().Count() != 1 {
		t.Fatal("dig packet did not remove the unsupported crop exactly once")
	}
	f.runtime[1].Tick()
	if f.worlds[1].Accessor.GetBlock(9, 63, 8) != farmland || f.worlds[1].Accessor.GetBlock(9, 64, 8) != crop || f.runtime[1].ItemEntities().Count() != 0 {
		t.Fatal("neighbor changes crossed the dimension boundary")
	}
	if err := router.Handle(f.sessions[0], protocol.StatePlay, packet); err != nil {
		t.Fatal(err)
	}
	f.runtime[0].Tick()
	if f.runtime[0].ItemEntities().Count() != 1 {
		t.Fatal("repeated dig duplicated the crop drop")
	}
}
