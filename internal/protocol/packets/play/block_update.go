package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// BlockUpdate sends a single block change to the client.
type BlockUpdate struct {
	Position BlockPos
	BlockID  int32
}

func NewBlockUpdate() protocol.Packet { return &BlockUpdate{} }

func (p *BlockUpdate) ID() int32 { return int32(packetid.ClientboundBlockChange) }

func (p *BlockUpdate) Decode(buf *protocol.Buffer) error {
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	p.BlockID, err = buf.ReadVarInt()
	return err
}

func (p *BlockUpdate) Encode(buf *protocol.Buffer) error {
	buf.WriteInt64(p.Position.Packed())
	buf.WriteVarInt(p.BlockID)
	return nil
}

// AcknowledgeBlockChange confirms a block change sequence to the client.
type AcknowledgeBlockChange struct {
	Sequence int32
}

func NewAcknowledgeBlockChange() protocol.Packet { return &AcknowledgeBlockChange{} }

func (p *AcknowledgeBlockChange) ID() int32 {
	return int32(packetid.ClientboundAcknowledgePlayerDigging)
}

func (p *AcknowledgeBlockChange) Decode(buf *protocol.Buffer) error {
	var err error
	p.Sequence, err = buf.ReadVarInt()
	return err
}

func (p *AcknowledgeBlockChange) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Sequence)
	return nil
}
