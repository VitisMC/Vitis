package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// MoveMinecart sends minecart movement data to the client.
type MoveMinecart struct {
	Data []byte
}

func NewMoveMinecart() protocol.Packet { return &MoveMinecart{} }

func (p *MoveMinecart) ID() int32 {
	return int32(packetid.ClientboundMoveMinecart)
}

func (p *MoveMinecart) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *MoveMinecart) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
