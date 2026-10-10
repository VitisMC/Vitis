package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// MessageAcknowledgement is sent by the client to acknowledge previously received chat messages.
type MessageAcknowledgement struct {
	MessageCount int32
}

func NewMessageAcknowledgement() protocol.Packet { return &MessageAcknowledgement{} }

func (p *MessageAcknowledgement) ID() int32 {
	return int32(packetid.ServerboundMessageAcknowledgement)
}

func (p *MessageAcknowledgement) Decode(buf *protocol.Buffer) error {
	var err error
	p.MessageCount, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	if p.MessageCount < 0 || p.MessageCount > 64 {
		return fmt.Errorf("message acknowledgement count out of range: %d", p.MessageCount)
	}
	return nil
}

func (p *MessageAcknowledgement) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.MessageCount)
	return nil
}
