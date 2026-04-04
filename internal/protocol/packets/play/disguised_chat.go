package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// DisguisedChatMessage sends a chat message without a sender UUID (profileless).
// Used for server-side chat when secure chat is not enforced.
type DisguisedChatMessage struct {
	MessageNBT []byte
	ChatType   int32
	SenderNBT  []byte
	HasTarget  bool
	TargetNBT  []byte
}

func NewDisguisedChatMessage() protocol.Packet { return &DisguisedChatMessage{} }

func (p *DisguisedChatMessage) ID() int32 {
	return int32(packetid.ClientboundProfilelessChat)
}

func (p *DisguisedChatMessage) Decode(buf *protocol.Buffer) error {
	return nil
}

func (p *DisguisedChatMessage) Encode(buf *protocol.Buffer) error {
	buf.WriteBytes(p.MessageNBT)
	// "ID or X" wire format: VarInt(0) = inline definition, VarInt(N>0) = registry ref to entry N-1.
	buf.WriteVarInt(p.ChatType + 1)
	buf.WriteBytes(p.SenderNBT)
	buf.WriteBool(p.HasTarget)
	if p.HasTarget {
		buf.WriteBytes(p.TargetNBT)
	}
	return nil
}

// NewDisguisedChat creates a DisguisedChatMessage with pre-encoded NBT components.
func NewDisguisedChat(messageNBT []byte, chatType int32, senderNBT []byte) *DisguisedChatMessage {
	return &DisguisedChatMessage{
		MessageNBT: messageNBT,
		ChatType:   chatType,
		SenderNBT:  senderNBT,
	}
}

// NewDisguisedChatSimple creates a simple disguised chat with plain text.
func NewDisguisedChatSimple(message, senderName string) *DisguisedChatMessage {
	msgComp := nbt.NewCompound()
	msgComp.PutString("text", message)
	msgEnc := nbt.NewEncoder(128)
	_ = msgEnc.WriteRootCompound(msgComp)

	senderComp := nbt.NewCompound()
	senderComp.PutString("text", senderName)
	senderEnc := nbt.NewEncoder(64)
	_ = senderEnc.WriteRootCompound(senderComp)

	return &DisguisedChatMessage{
		MessageNBT: msgEnc.Bytes(),
		ChatType:   0,
		SenderNBT:  senderEnc.Bytes(),
	}
}
