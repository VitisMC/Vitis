package streaming

import "github.com/vitismc/vitis/internal/world/chunk"

// SerializeChunkPayload encodes the authoritative palette-based chunk state.
func SerializeChunkPayload(c *chunk.Chunk) []byte {
	if c == nil {
		return nil
	}
	return c.EncodePacketPayload()
}
