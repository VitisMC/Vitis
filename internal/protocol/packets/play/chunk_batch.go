package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChunkBatchStart signals the beginning of a chunk batch to the client.
type ChunkBatchStart struct{}

func NewChunkBatchStart() protocol.Packet { return &ChunkBatchStart{} }

func (p *ChunkBatchStart) ID() int32 { return int32(packetid.ClientboundChunkBatchStart) }

func (p *ChunkBatchStart) Decode(_ *protocol.Buffer) error { return nil }

func (p *ChunkBatchStart) Encode(_ *protocol.Buffer) error { return nil }

// ChunkBatchFinished signals the end of a chunk batch and reports how many chunks were sent.
type ChunkBatchFinished struct {
	BatchSize int32
}

func NewChunkBatchFinished() protocol.Packet { return &ChunkBatchFinished{} }

func (p *ChunkBatchFinished) ID() int32 { return int32(packetid.ClientboundChunkBatchFinished) }

func (p *ChunkBatchFinished) Decode(buf *protocol.Buffer) error {
	var err error
	p.BatchSize, err = buf.ReadVarInt()
	return err
}

func (p *ChunkBatchFinished) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.BatchSize)
	return nil
}

// ChunkBatchReceived is sent by the client to acknowledge a chunk batch.
type ChunkBatchReceived struct {
	ChunksPerTick float32
}

func NewChunkBatchReceived() protocol.Packet { return &ChunkBatchReceived{} }

func (p *ChunkBatchReceived) ID() int32 { return int32(packetid.ServerboundChunkBatchReceived) }

func (p *ChunkBatchReceived) Decode(buf *protocol.Buffer) error {
	var err error
	p.ChunksPerTick, err = buf.ReadFloat32()
	return err
}

func (p *ChunkBatchReceived) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat32(p.ChunksPerTick)
	return nil
}
