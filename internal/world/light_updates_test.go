package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/streaming"
)

// lightTestSender captures packets without a network goroutine.
type lightTestSender struct{ packets []protocol.Packet }

// Send records immutable packets emitted by the world tick.
func (s *lightTestSender) Send(p protocol.Packet) error {
	s.packets = append(s.packets, p)
	return nil
}

// TestWorldLightCrossesChunkBoundaryAndClears verifies positive and negative seams.
func TestWorldLightCrossesChunkBoundaryAndClears(t *testing.T) {
	for _, x := range []int32{-1, 0} {
		t.Run(protocol.UUIDToString(protocol.UUID{uint64(uint32(x))}), func(t *testing.T) {
			storage := chunk.NewStorage(2)
			left, right := chunk.New(x, 0), chunk.New(x+1, 0)
			storage.Set(left)
			storage.Set(right)
			w, err := New(Config{Name: "light", Dimension: "minecraft:the_nether", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = w.Close(nil) })
			ax := int(x)*16 + 15
			left.SetBlock(ax, 64, 8, block.DefaultStateID("minecraft:glowstone"))
			w.Tick()
			if got := right.Data().LightEngine.BlockLight(0, 64, 8); got != 14 {
				t.Fatalf("border light = %d, want 14", got)
			}
			if got := right.Data().LightEngine.SkyLight(0, 64, 8); got != 0 {
				t.Fatalf("Nether has sky light: %d", got)
			}
			left.SetBlock(ax, 64, 8, 0)
			w.Tick()
			if got := right.Data().LightEngine.BlockLight(0, 64, 8); got != 0 {
				t.Fatalf("removed source left stale border light: %d", got)
			}
		})
	}
}

// TestLightUpdatesReachOnlyObservers checks standalone update delivery and payload ownership.
func TestLightUpdatesReachOnlyObservers(t *testing.T) {
	storage := chunk.NewStorage(1)
	c := chunk.New(0, 0)
	storage.Set(c)
	w, err := New(Config{Name: "light-observers", ChunkManager: chunk.NewManager(chunk.ManagerConfig{Storage: storage})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(nil) })
	first, second := &lightTestSender{}, &lightTestSender{}
	for index, sender := range []*lightTestSender{first, second} {
		id := int32(index + 1)
		w.AddPlayer(entity.NewPlayer(id, protocol.UUID{uint64(id)}, "observer", entity.Vec3{X: 8, Y: 64, Z: 8}, entity.Vec2{}, sender, 2))
	}
	streamer, _ := w.streaming.GetStreamer(1)
	streamer.View().MarkLoaded(streaming.ChunkPos{X: 0, Z: 0})
	first.packets, second.packets = nil, nil
	w.bindChunkLight(c)
	c.SetBlock(8, 64, 8, block.DefaultStateID("minecraft:glowstone"))
	w.tickLighting()
	if len(first.packets) != 1 || len(second.packets) != 0 {
		t.Fatalf("observer packets=%d, unobserved packets=%d", len(first.packets), len(second.packets))
	}
	packet, ok := first.packets[0].(*playpacket.UpdateLight)
	if !ok || len(packet.Data) == 0 {
		t.Fatal("missing light update payload")
	}
	frozen := string(packet.Data)
	c.SetBlock(8, 64, 8, 0)
	w.tickLighting()
	if string(packet.Data) != frozen {
		t.Fatal("subsequent light mutations changed a queued packet")
	}
}
