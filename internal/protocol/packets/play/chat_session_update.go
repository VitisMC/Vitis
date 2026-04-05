package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ChatSessionUpdate is sent by the client to update its chat session public key information.
type ChatSessionUpdate struct {
	SessionID    protocol.UUID
	ExpiresAt    int64
	PublicKey    []byte
	KeySignature []byte
}

func NewChatSessionUpdate() protocol.Packet { return &ChatSessionUpdate{} }

func (p *ChatSessionUpdate) ID() int32 {
	return int32(packetid.ServerboundChatSessionUpdate)
}

func (p *ChatSessionUpdate) Decode(buf *protocol.Buffer) error {
	var err error
	if p.SessionID, err = buf.ReadUUID(); err != nil {
		return err
	}
	if p.ExpiresAt, err = buf.ReadInt64(); err != nil {
		return err
	}
	keyLen, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	if p.PublicKey, err = buf.ReadBytes(int(keyLen)); err != nil {
		return err
	}
	sigLen, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.KeySignature, err = buf.ReadBytes(int(sigLen))
	return err
}

func (p *ChatSessionUpdate) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.SessionID)
	buf.WriteInt64(p.ExpiresAt)
	buf.WriteVarInt(int32(len(p.PublicKey)))
	buf.WriteBytes(p.PublicKey)
	buf.WriteVarInt(int32(len(p.KeySignature)))
	buf.WriteBytes(p.KeySignature)
	return nil
}
