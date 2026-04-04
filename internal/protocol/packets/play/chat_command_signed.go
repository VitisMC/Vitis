package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChatCommandSigned is sent by the client when executing a command that
// contains a message-type argument (e.g. /say, /msg). The signature
// fields are ignored on an unsigned (offline-mode) server.
type ChatCommandSigned struct {
	Command string
}

func NewChatCommandSigned() protocol.Packet { return &ChatCommandSigned{} }

func (p *ChatCommandSigned) ID() int32 { return int32(packetid.ServerboundChatCommandSigned) }

func (p *ChatCommandSigned) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Command, err = buf.ReadString(); err != nil {
		return err
	}
	// Skip: Timestamp (Long), Salt (Long), ArgumentSignatures array,
	// MessageCount (VarInt), Acknowledged (BitSet).
	// We consume whatever remains since we only need the Command string.
	remaining := buf.Remaining()
	if remaining > 0 {
		_, err = buf.ReadBytes(remaining)
	}
	return err
}

func (p *ChatCommandSigned) Encode(buf *protocol.Buffer) error {
	return buf.WriteString(p.Command)
}
