package streaming

import (
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
)

const defaultEncodeBufferSize = 4096

// ChunkProvider retrieves chunk data from the world.
type ChunkProvider interface {
	GetChunk(x, z int32) (*chunk.Chunk, bool)
}

// ChunkSender sends chunk-related packets to a player.
type ChunkSender interface {
	Send(packet protocol.Packet) error
}

// SendPipeline handles chunk serialization and packet dispatch.
type SendPipeline struct {
	bufferPool sync.Pool
}

// NewSendPipeline creates a send pipeline with pooled encode buffers.
func NewSendPipeline() *SendPipeline {
	sp := &SendPipeline{}
	sp.bufferPool.New = func() any {
		return make([]byte, 0, defaultEncodeBufferSize)
	}
	return sp
}

// SerializeChunk encodes chunk section data into a protocol-compliant byte slice.
func (sp *SendPipeline) SerializeChunk(c *chunk.Chunk) []byte {
	if c == nil {
		return nil
	}
	return SerializeChunkPayload(c)
}

// SendChunk serializes and sends a single chunk to the player.
func (sp *SendPipeline) SendChunk(pos ChunkPos, provider ChunkProvider, sender ChunkSender) error {
	c, ok := provider.GetChunk(pos.X, pos.Z)
	if !ok {
		return ErrChunkNotReady
	}

	var data []byte
	if payload := c.EncodePacketPayload(); payload != nil {
		data = payload
	} else {
		data = sp.SerializeChunk(c)
	}

	pkt := &play.ChunkDataAndUpdateLight{
		ChunkX:  pos.X,
		ChunkZ:  pos.Z,
		Payload: data,
	}
	return sender.Send(pkt)
}

// SendUnloadChunk sends an unload packet for the given chunk position.
func (sp *SendPipeline) SendUnloadChunk(pos ChunkPos, sender ChunkSender) error {
	pkt := &play.UnloadChunk{
		ChunkX: pos.X,
		ChunkZ: pos.Z,
	}
	return sender.Send(pkt)
}

// SendBatch sends up to len(chunks) chunk packets, stopping on backpressure.
func (sp *SendPipeline) SendBatch(chunks []ChunkPos, provider ChunkProvider, sender ChunkSender) int {
	sent := 0
	for _, pos := range chunks {
		if err := sp.SendChunk(pos, provider, sender); err != nil {
			return sent
		}
		sent++
	}
	return sent
}

func (sp *SendPipeline) acquireBuffer() []byte {
	buf := sp.bufferPool.Get().([]byte)
	return buf[:0]
}

func (sp *SendPipeline) releaseBuffer(buf []byte) {
	if cap(buf) > 1<<20 {
		return
	}
	sp.bufferPool.Put(buf[:0])
}

func appendUint16BE(buf []byte, v uint16) []byte {
	return append(buf, byte(v>>8), byte(v))
}
