package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// QueryBlockNbt is sent by the client to query the NBT of a block entity.
type QueryBlockNbt struct {
	TransactionID int32
	Position      BlockPos
}

func NewQueryBlockNbt() protocol.Packet { return &QueryBlockNbt{} }

func (p *QueryBlockNbt) ID() int32 {
	return int32(packetid.ServerboundQueryBlockNbt)
}

func (p *QueryBlockNbt) Decode(buf *protocol.Buffer) error {
	var err error
	if p.TransactionID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	return nil
}

func (p *QueryBlockNbt) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TransactionID)
	buf.WriteInt64(p.Position.Packed())
	return nil
}

// QueryEntityNbt is sent by the client to query the NBT of an entity.
type QueryEntityNbt struct {
	TransactionID int32
	EntityID      int32
}

func NewQueryEntityNbt() protocol.Packet { return &QueryEntityNbt{} }

func (p *QueryEntityNbt) ID() int32 {
	return int32(packetid.ServerboundQueryEntityNbt)
}

func (p *QueryEntityNbt) Decode(buf *protocol.Buffer) error {
	var err error
	if p.TransactionID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.EntityID, err = buf.ReadVarInt()
	return err
}

func (p *QueryEntityNbt) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TransactionID)
	buf.WriteVarInt(p.EntityID)
	return nil
}
