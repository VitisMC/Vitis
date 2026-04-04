package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ServerData sends the server's MOTD and icon to the client (shown in pause menu).
// MOTD is a Text Component encoded as NBT (not JSON string).
type ServerData struct {
	MOTDText string
	HasIcon  bool
	Icon     []byte
}

func NewServerData() protocol.Packet { return &ServerData{} }

func (p *ServerData) ID() int32 { return int32(packetid.ClientboundServerData) }

func (p *ServerData) Decode(buf *protocol.Buffer) error {
	return nil
}

func (p *ServerData) Encode(buf *protocol.Buffer) error {
	comp := nbt.NewCompound()
	comp.PutString("text", p.MOTDText)
	enc := nbt.NewEncoder(128)
	if err := enc.WriteRootCompound(comp); err != nil {
		return err
	}
	buf.WriteBytes(enc.Bytes())

	if p.HasIcon && len(p.Icon) > 0 {
		buf.WriteByte(1)
		buf.WriteVarInt(int32(len(p.Icon)))
		buf.WriteBytes(p.Icon)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
