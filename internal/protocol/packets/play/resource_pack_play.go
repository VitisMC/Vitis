package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundRemoveResourcePack tells the client to remove a resource pack during play.
type ClientboundRemoveResourcePack struct {
	HasUUID bool
	UUID    protocol.UUID
}

func NewClientboundRemoveResourcePack() protocol.Packet { return &ClientboundRemoveResourcePack{} }

func (p *ClientboundRemoveResourcePack) ID() int32 {
	return int32(packetid.ClientboundRemoveResourcePack)
}

func (p *ClientboundRemoveResourcePack) Decode(buf *protocol.Buffer) error {
	var err error
	if p.HasUUID, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasUUID {
		p.UUID, err = buf.ReadUUID()
	}
	return err
}

func (p *ClientboundRemoveResourcePack) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.HasUUID)
	if p.HasUUID {
		buf.WriteUUID(p.UUID)
	}
	return nil
}

// ClientboundAddResourcePack tells the client to add a resource pack during play.
type ClientboundAddResourcePack struct {
	UUID             protocol.UUID
	URL              string
	Hash             string
	Forced           bool
	HasPromptMessage bool
	PromptMessage    string
}

func NewClientboundAddResourcePack() protocol.Packet { return &ClientboundAddResourcePack{} }

func (p *ClientboundAddResourcePack) ID() int32 {
	return int32(packetid.ClientboundAddResourcePack)
}

func (p *ClientboundAddResourcePack) Decode(buf *protocol.Buffer) error {
	var err error
	if p.UUID, err = buf.ReadUUID(); err != nil {
		return err
	}
	if p.URL, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Hash, err = buf.ReadString(); err != nil {
		return err
	}
	if p.Forced, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasPromptMessage, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasPromptMessage {
		p.PromptMessage, err = buf.ReadString()
	}
	return err
}

func (p *ClientboundAddResourcePack) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.UUID)
	buf.WriteString(p.URL)
	buf.WriteString(p.Hash)
	buf.WriteBool(p.Forced)
	buf.WriteBool(p.HasPromptMessage)
	if p.HasPromptMessage {
		buf.WriteString(p.PromptMessage)
	}
	return nil
}
