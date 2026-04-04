package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Disconnect kicks a player from the server during play state.
// Reason is a Text Component encoded as NBT in 1.21.4.
type Disconnect struct {
	Text string
}

func NewDisconnect() protocol.Packet { return &Disconnect{} }

func (p *Disconnect) ID() int32 { return int32(packetid.ClientboundKickDisconnect) }

func (p *Disconnect) Decode(buf *protocol.Buffer) error {
	return nil
}

func (p *Disconnect) Encode(buf *protocol.Buffer) error {
	comp := nbt.NewCompound()
	comp.PutString("text", p.Text)
	enc := nbt.NewEncoder(128)
	if err := enc.WriteRootCompound(comp); err != nil {
		return err
	}
	buf.WriteBytes(enc.Bytes())
	return nil
}

// NewDisconnectText creates a Disconnect packet with a plain text reason.
func NewDisconnectText(text string) *Disconnect {
	return &Disconnect{Text: text}
}
