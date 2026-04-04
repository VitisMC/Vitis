package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

type BlockEntityData struct {
	X       int32
	Y       int32
	Z       int32
	Type    int32
	NBTData []byte
}

func NewBlockEntityData() protocol.Packet { return &BlockEntityData{} }

func (p *BlockEntityData) ID() int32 {
	return int32(packetid.ClientboundTileEntityData)
}

func (p *BlockEntityData) Encode(buf *protocol.Buffer) error {
	position := encodeBlockPosition(p.X, p.Y, p.Z)
	buf.WriteInt64(int64(position))
	buf.WriteVarInt(p.Type)
	buf.WriteBytes(p.NBTData)
	return nil
}

func (p *BlockEntityData) Decode(buf *protocol.Buffer) error {
	posInt64, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.X, p.Y, p.Z = decodeBlockPosition(uint64(posInt64))

	p.Type, err = buf.ReadVarInt()
	if err != nil {
		return err
	}

	p.NBTData = buf.RemainingBytes()
	return nil
}

func encodeBlockPosition(x, y, z int32) uint64 {
	return (uint64(x&0x3FFFFFF) << 38) | (uint64(z&0x3FFFFFF) << 12) | uint64(y&0xFFF)
}

func decodeBlockPosition(val uint64) (x, y, z int32) {
	x = int32(val >> 38)
	z = int32((val >> 12) & 0x3FFFFFF)
	y = int32(val & 0xFFF)

	if x >= 1<<25 {
		x -= 1 << 26
	}
	if z >= 1<<25 {
		z -= 1 << 26
	}
	if y >= 1<<11 {
		y -= 1 << 12
	}
	return
}
