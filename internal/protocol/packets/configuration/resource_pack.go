package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// RemoveResourcePack tells the client to remove a resource pack during configuration.
type RemoveResourcePack struct {
	HasUUID bool
	UUID    protocol.UUID
}

func NewRemoveResourcePack() protocol.Packet { return &RemoveResourcePack{} }

func (p *RemoveResourcePack) ID() int32 {
	return int32(packetid.ClientboundConfigRemoveResourcePack)
}

func (p *RemoveResourcePack) Decode(buf *protocol.Buffer) error {
	var err error
	if p.HasUUID, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasUUID {
		p.UUID, err = buf.ReadUUID()
	}
	return err
}

func (p *RemoveResourcePack) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.HasUUID)
	if p.HasUUID {
		buf.WriteUUID(p.UUID)
	}
	return nil
}

// AddResourcePack tells the client to add a resource pack during configuration.
type AddResourcePack struct {
	UUID               protocol.UUID
	URL                string
	Hash               string
	Forced             bool
	HasPromptMessage   bool
	PromptMessage      string
}

func NewAddResourcePack() protocol.Packet { return &AddResourcePack{} }

func (p *AddResourcePack) ID() int32 {
	return int32(packetid.ClientboundConfigAddResourcePack)
}

func (p *AddResourcePack) Decode(buf *protocol.Buffer) error {
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

func (p *AddResourcePack) Encode(buf *protocol.Buffer) error {
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

// ResourcePackReceive is sent by the client to report the status of a resource pack download during configuration.
type ResourcePackReceive struct {
	UUID   protocol.UUID
	Result int32
}

func NewResourcePackReceive() protocol.Packet { return &ResourcePackReceive{} }

func (p *ResourcePackReceive) ID() int32 {
	return int32(packetid.ServerboundConfigResourcePackReceive)
}

func (p *ResourcePackReceive) Decode(buf *protocol.Buffer) error {
	var err error
	if p.UUID, err = buf.ReadUUID(); err != nil {
		return err
	}
	p.Result, err = buf.ReadVarInt()
	return err
}

func (p *ResourcePackReceive) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.UUID)
	buf.WriteVarInt(p.Result)
	return nil
}
