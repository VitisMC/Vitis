package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChatSuggestions adds or removes custom chat suggestions on the client.
type ChatSuggestions struct {
	Action  int32
	Entries []string
}

func NewChatSuggestions() protocol.Packet { return &ChatSuggestions{} }

func (p *ChatSuggestions) ID() int32 {
	return int32(packetid.ClientboundChatSuggestions)
}

func (p *ChatSuggestions) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Action, err = buf.ReadVarInt(); err != nil {
		return err
	}
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Entries = make([]string, count)
	for i := int32(0); i < count; i++ {
		if p.Entries[i], err = buf.ReadString(); err != nil {
			return err
		}
	}
	return nil
}

func (p *ChatSuggestions) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Action)
	buf.WriteVarInt(int32(len(p.Entries)))
	for _, e := range p.Entries {
		buf.WriteString(e)
	}
	return nil
}
