package streaming

import (
	"testing"

	"github.com/vitismc/vitis/internal/block"
	genbe "github.com/vitismc/vitis/internal/data/generated/block_entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestBedEntitiesSurviveChunkResends verifies the packets used by login and respawn.
func TestBedEntitiesSurviveChunkResends(t *testing.T) {
	provider := newMockProvider()
	provider.addChunk(0, 0)
	owner := provider.chunks[ChunkPos{}]
	for index, part := range []string{"head", "foot"} {
		owner.SetBlock(1, 64, 1+index, block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": part, "occupied": "false"}))
	}
	pipeline, sender := NewSendPipeline(), newMockSender()
	for pass := 0; pass < 3; pass++ {
		sender.reset()
		if err := pipeline.SendChunk(ChunkPos{}, provider, sender); err != nil {
			t.Fatal(err)
		}
		assertBedChunkEntries(t, sender.packets[0].data, 2)
		if pass == 1 {
			owner.Data().BlockEntities = nil
			encoded, err := chunk.EncodeChunkNBT(owner)
			if err != nil {
				t.Fatal(err)
			}
			owner, err = chunk.DecodeChunkNBT(encoded, 0, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			provider.chunks[ChunkPos{}] = owner
		}
	}
	owner.SetBlock(1, 64, 1, 0)
	owner.SetBlock(1, 64, 2, 0)
	sender.reset()
	if err := pipeline.SendChunk(ChunkPos{}, provider, sender); err != nil {
		t.Fatal(err)
	}
	assertBedChunkEntries(t, sender.packets[0].data, 0)
}

// assertBedChunkEntries decodes the complete bed records from a chunk wire packet.
func assertBedChunkEntries(t *testing.T, packet []byte, count int32) {
	t.Helper()
	buf := protocol.WrapBuffer(packet)
	if _, err := buf.ReadBytes(8); err != nil {
		t.Fatal(err)
	}
	decoder := nbt.NewDecoder(buf.RemainingBytes())
	if _, err := decoder.ReadRootCompound(); err != nil {
		t.Fatal(err)
	}
	if _, err := buf.ReadBytes(decoder.Pos()); err != nil {
		t.Fatal(err)
	}
	length, err := buf.ReadVarInt()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buf.ReadBytes(int(length)); err != nil {
		t.Fatal(err)
	}
	actual, err := buf.ReadVarInt()
	if err != nil || actual != count {
		t.Fatalf("chunk bed count = %d, want %d (%v)", actual, count, err)
	}
	seen := map[byte]bool{}
	for i := int32(0); i < count; i++ {
		xz, err := buf.ReadByte()
		if err != nil {
			t.Fatal(err)
		}
		y, err := buf.ReadInt16()
		if err != nil || y != 64 {
			t.Fatalf("bed height = %d (%v)", y, err)
		}
		typeID, err := buf.ReadVarInt()
		if err != nil || typeID != genbe.BlockEntityBed {
			t.Fatalf("bed type = %d (%v)", typeID, err)
		}
		decoder = nbt.NewDecoder(buf.RemainingBytes())
		root, err := decoder.ReadRootCompound()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := buf.ReadBytes(decoder.Pos()); err != nil {
			t.Fatal(err)
		}
		if id, _ := root.GetString("id"); id != "minecraft:bed" || seen[xz] || (xz != 0x11 && xz != 0x12) {
			t.Fatalf("invalid bed record: id=%q position=%x", id, xz)
		}
		seen[xz] = true
	}
}
