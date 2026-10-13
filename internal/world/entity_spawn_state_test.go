package world

import (
	"testing"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestRestoredEntityStateReachesOnlyChunkObservers verifies restart synchronization and single spawns.
func TestRestoredEntityStateReachesOnlyChunkObservers(t *testing.T) {
	c := chunk.New(0, 0)
	w, _ := persistentTestWorld(t, t.TempDir(), c)
	defer func() {
		if err := w.Close(nil); err != nil {
			t.Error(err)
		}
	}()
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_pickaxe"), 1)
	stack.SetDamage(7)
	dropped := entity.NewItemEntity(1, protocol.UUID{9, 1}, entity.Vec3{X: 8, Y: 64, Z: 8}, stack)
	orb := entity.NewXPOrb(2, protocol.UUID{9, 2}, entity.Vec3{X: 8, Y: 64, Z: 8}, 13)
	itemNBT, err := dropped.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	xpNBT, err := orb.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	c.LoadEntityNBT(entityRegionNBT(t, 0, 0, itemNBT, xpNBT))
	if err := w.bindChunkEntities(c); err != nil {
		t.Fatal(err)
	}
	first, second := &lightTestSender{}, &lightTestSender{}
	p1 := entity.NewPlayer(100, protocol.UUID{10, 1}, "observer", entity.Vec3{X: 8, Y: 64, Z: 8}, entity.Vec2{}, first, 2)
	p2 := entity.NewPlayer(101, protocol.UUID{10, 2}, "hidden", entity.Vec3{X: 8, Y: 64, Z: 8}, entity.Vec2{}, second, 2)
	w.AddPlayer(p1)
	w.AddPlayer(p2)
	filter := func(id, x, z int32) bool { return id == p1.ID() && x == 0 && z == 0 }
	w.tracker.TickWithChunkFilter(w.entities, filter)
	var metadataPacket *playpacket.SetEntityMetadata
	spawns, orbs := 0, 0
	for _, packet := range first.packets {
		switch pkt := packet.(type) {
		case *playpacket.SpawnEntity:
			if pkt.EntityUUID == dropped.UUID() {
				spawns++
			}
		case *playpacket.SpawnEntityExperienceOrb:
			if pkt.Count != 13 {
				t.Fatal("XP spawn value lost")
			}
			orbs++
		case *playpacket.SetEntityMetadata:
			if w.items.Get(pkt.EntityID) != nil {
				metadataPacket = pkt
			}
		}
	}
	if spawns != 1 || orbs != 1 || metadataPacket == nil || len(second.packets) != 0 {
		t.Fatalf("incorrect observer state: spawns=%d orbs=%d hidden=%d", spawns, orbs, len(second.packets))
	}
	buffer := protocol.WrapBuffer(metadataPacket.Metadata)
	index, err := buffer.ReadByte()
	if err != nil || index != metadata.IndexItemEntityItem {
		t.Fatal("missing item metadata index")
	}
	typ, err := buffer.ReadVarInt()
	if err != nil || typ != metadata.TypeSlot {
		t.Fatal("invalid item metadata type")
	}
	saved, err := inventory.DecodeItemStack(buffer)
	if err != nil || saved.ItemID != stack.ItemID || saved.Damage != 7 {
		t.Fatalf("item components not synchronized: %+v %v", saved, err)
	}
	w.items.Get(metadataPacket.EntityID).SetStack(inventory.NewSlot(1, 3))
	w.tracker.TickWithChunkFilter(w.entities, filter)
	spawns = 0
	for _, packet := range first.packets {
		if pkt, ok := packet.(*playpacket.SpawnEntity); ok && pkt.EntityUUID == dropped.UUID() {
			spawns++
		}
	}
	if spawns != 1 {
		t.Fatal("repeated tracking duplicated item spawn")
	}
}
