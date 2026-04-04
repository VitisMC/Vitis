package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChatCommand is sent by the client when executing a slash command.
type ChatCommand struct {
	Command string
}

func NewChatCommand() protocol.Packet { return &ChatCommand{} }

func (p *ChatCommand) ID() int32 { return int32(packetid.ServerboundChatCommand) }

func (p *ChatCommand) Decode(buf *protocol.Buffer) error {
	var err error
	p.Command, err = buf.ReadString()
	return err
}

func (p *ChatCommand) Encode(buf *protocol.Buffer) error {
	return buf.WriteString(p.Command)
}
