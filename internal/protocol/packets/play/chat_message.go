package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ServerboundChatMessage is sent by the client when typing in chat.
type ServerboundChatMessage struct {
	Message   string
	Timestamp int64
	Salt      int64
	HasSig    bool
	Signature []byte
	Offset    int32
	Ack       []byte
}

func NewServerboundChatMessage() protocol.Packet { return &ServerboundChatMessage{} }

func (p *ServerboundChatMessage) ID() int32 { return int32(packetid.ServerboundChatMessage) }

func (p *ServerboundChatMessage) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Message, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Timestamp, err = buf.ReadInt64(); err != nil {
		return err
	}
	if p.Salt, err = buf.ReadInt64(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.HasSig = b != 0
	if p.HasSig {
		p.Signature, err = buf.ReadBytes(256)
		if err != nil {
			return err
		}
	}
	if p.Offset, err = buf.ReadVarInt(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		p.Ack, err = buf.ReadBytes(remaining)
	}
	return err
}

func (p *ServerboundChatMessage) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Message)
	buf.WriteInt64(p.Timestamp)
	buf.WriteInt64(p.Salt)
	if p.HasSig {
		buf.WriteByte(1)
		buf.WriteBytes(p.Signature)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteVarInt(p.Offset)
	buf.WriteBytes(p.Ack)
	return nil
}
