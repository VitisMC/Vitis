package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestRandomTicksRespectActivityAndGameRule runs growth through the connected world tick.
func TestRandomTicksRespectActivityAndGameRule(t *testing.T) {
	storage := chunk.NewStorage(1)
	c := chunk.New(0, 0)
	wheat := block.StateID("minecraft:wheat", map[string]string{"age": "0"})
	farmland := block.StateID("minecraft:farmland", map[string]string{"moisture": "7"})
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, 63, z, farmland)
			c.SetBlock(x, 64, z, wheat)
		}
	}
	storage.Set(c)
	w, err := New(Config{Name: "random-ticks", Seed: 42, ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.SetGameRule("randomTickSpeed", "4096"); err != nil {
		t.Fatal(err)
	}
	w.Tick()
	if c.GetBlock(8, 64, 8) != wheat {
		t.Fatal("inactive crop grew")
	}
	activateTestChunk(t, w, storage, 0, 0)
	if err := w.SetGameRule("randomTickSpeed", "0"); err != nil {
		t.Fatal(err)
	}
	w.Tick()
	if c.GetBlock(8, 64, 8) != wheat {
		t.Fatal("randomTickSpeed=0 advanced crops")
	}
	if err := w.SetGameRule("randomTickSpeed", "4096"); err != nil {
		t.Fatal(err)
	}
	w.Tick()
	grown := 0
	for x := 1; x < 15; x++ {
		for z := 1; z < 15; z++ {
			if c.GetBlock(x, 64, z) != wheat {
				grown++
			}
		}
	}
	if grown == 0 || w.TickScheduler().Pending() != 0 {
		t.Fatalf("grown=%d, scheduled=%d", grown, w.TickScheduler().Pending())
	}
	if _, available := w.LookupBlockState(48, 64, 0); available {
		t.Fatal("unloaded chunk reported as available")
	}
}
