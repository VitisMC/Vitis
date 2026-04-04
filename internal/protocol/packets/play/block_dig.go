package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	DigStarted    int32 = 0
	DigCancelled  int32 = 1
	DigFinished   int32 = 2
	DigDropStack  int32 = 3
	DigDropItem   int32 = 4
	DigShootArrow int32 = 5
	DigSwapHands  int32 = 6
)

// BlockDig is sent by the client when breaking/interacting with blocks.
type BlockDig struct {
	Status   int32
	Position BlockPos
	Face     int8
	Sequence int32
}

// BlockPos encodes a block position as a packed int64.
type BlockPos struct {
	X, Y, Z int32
}

// Packed returns the protocol wire format of a block position.
func (p BlockPos) Packed() int64 {
	return (int64(p.X&0x3FFFFFF) << 38) | (int64(p.Z&0x3FFFFFF) << 12) | int64(p.Y&0xFFF)
}

// UnpackBlockPos decodes a packed block position.
func UnpackBlockPos(v int64) BlockPos {
	x := int32(v >> 38)
	z := int32(v << 26 >> 38)
	y := int32(v << 52 >> 52)
	return BlockPos{X: x, Y: y, Z: z}
}

func NewBlockDig() protocol.Packet { return &BlockDig{} }

func (p *BlockDig) ID() int32 { return int32(packetid.ServerboundBlockDig) }

func (p *BlockDig) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Status, err = buf.ReadVarInt(); err != nil {
		return err
	}
	pos, err := buf.ReadInt64()
	if err != nil {
		return err
	}
	p.Position = UnpackBlockPos(pos)
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.Face = int8(b)
	p.Sequence, err = buf.ReadVarInt()
	return err
}

func (p *BlockDig) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Status)
	buf.WriteInt64(p.Position.Packed())
	buf.WriteByte(byte(p.Face))
	buf.WriteVarInt(p.Sequence)
	return nil
}
