package streaming

import (
	"testing"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
)

type compressedBenchmarkSender struct {
	packet *protocol.Buffer
	frame  *protocol.Buffer
}

// Send encodes and compresses a chunk packet as the connection pipeline does.
func (s *compressedBenchmarkSender) Send(packet protocol.Packet) error {
	s.packet.Reset()
	if err := packet.Encode(s.packet); err != nil {
		return err
	}
	s.frame.Reset()
	s.frame.WriteVarInt(packet.ID())
	s.frame.WriteBytes(s.packet.Bytes())
	_, err := network.CompressFrame(s.frame.Bytes(), 256)
	return err
}

// BenchmarkReadyChunkSend measures repeated serialization and framing of a loaded chunk.
func BenchmarkReadyChunkSend(b *testing.B) {
	benchmarkReadyChunkSend(b, false)
}

// BenchmarkReadyChunkSendCold measures the first send after a chunk revision changes.
func BenchmarkReadyChunkSendCold(b *testing.B) {
	benchmarkReadyChunkSend(b, true)
}

// benchmarkReadyChunkSend runs the ready-chunk path with optional cache invalidation.
func benchmarkReadyChunkSend(b *testing.B, cold bool) {
	c := chunk.New(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.SetBlock(x, -64, z, genblock.BedrockDefaultState)
			for y := -63; y < 64; y++ {
				c.SetBlock(x, y, z, genblock.DirtDefaultState)
			}
			c.SetBlock(x, 64, z, genblock.GrassBlockDefaultState)
		}
	}
	c.EncodePacketPayload()
	provider := newMockProvider()
	provider.chunks[ChunkPos{}] = c
	sender := &compressedBenchmarkSender{
		packet: protocol.NewBuffer(16384),
		frame:  protocol.NewBuffer(16384),
	}
	pipeline := NewSendPipeline()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if cold {
			c.MarkDirty()
		}
		if err := pipeline.SendChunk(ChunkPos{}, provider, sender); err != nil {
			b.Fatal(err)
		}
	}
}
