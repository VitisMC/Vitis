package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SystemChatMessage sends an unsigned system message to the client.
// Content is a Text Component encoded as NBT in 1.21.4.
type SystemChatMessage struct {
	Text    string
	RawNBT  []byte
	Overlay bool
}

func NewSystemChatMessage() protocol.Packet { return &SystemChatMessage{} }

func (p *SystemChatMessage) ID() int32 { return int32(packetid.ClientboundSystemChat) }

func (p *SystemChatMessage) Decode(buf *protocol.Buffer) error {
	return nil
}

func (p *SystemChatMessage) Encode(buf *protocol.Buffer) error {
	if len(p.RawNBT) > 0 {
		buf.WriteBytes(p.RawNBT)
	} else {
		comp := nbt.NewCompound()
		comp.PutString("text", p.Text)
		enc := nbt.NewEncoder(128)
		if err := enc.WriteRootCompound(comp); err != nil {
			return err
		}
		buf.WriteBytes(enc.Bytes())
	}
	if p.Overlay {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}

// NewSystemChatText creates a SystemChatMessage with plain text content.
func NewSystemChatText(text string) *SystemChatMessage {
	return &SystemChatMessage{Text: text}
}

// NewSystemChatNBT creates a SystemChatMessage with pre-encoded NBT TextComponent.
func NewSystemChatNBT(nbtData []byte) *SystemChatMessage {
	return &SystemChatMessage{RawNBT: nbtData}
}
