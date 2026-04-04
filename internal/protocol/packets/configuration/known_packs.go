package configuration

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// KnownPack represents a single data pack entry.
type KnownPack struct {
	Namespace string
	ID        string
	Version   string
}

// ClientboundKnownPacks is sent by the server to inform the client of available data packs.
type ClientboundKnownPacks struct {
	Packs []KnownPack
}

// NewClientboundKnownPacks constructs an empty ClientboundKnownPacks packet.
func NewClientboundKnownPacks() protocol.Packet {
	return &ClientboundKnownPacks{}
}

// ID returns the protocol packet id.
func (p *ClientboundKnownPacks) ID() int32 {
	return int32(packetid.ClientboundConfigSelectKnownPacks)
}

// Decode reads ClientboundKnownPacks fields from buffer.
func (p *ClientboundKnownPacks) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode known_packs count: %w", err)
	}
	p.Packs = make([]KnownPack, count)
	for i := int32(0); i < count; i++ {
		ns, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode known_packs[%d] namespace: %w", i, err)
		}
		id, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode known_packs[%d] id: %w", i, err)
		}
		ver, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode known_packs[%d] version: %w", i, err)
		}
		p.Packs[i] = KnownPack{Namespace: ns, ID: id, Version: ver}
	}
	return nil
}

// Encode writes ClientboundKnownPacks fields to buffer.
func (p *ClientboundKnownPacks) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Packs)))
	for i, pack := range p.Packs {
		if err := buf.WriteString(pack.Namespace); err != nil {
			return fmt.Errorf("encode known_packs[%d] namespace: %w", i, err)
		}
		if err := buf.WriteString(pack.ID); err != nil {
			return fmt.Errorf("encode known_packs[%d] id: %w", i, err)
		}
		if err := buf.WriteString(pack.Version); err != nil {
			return fmt.Errorf("encode known_packs[%d] version: %w", i, err)
		}
	}
	return nil
}

// ServerboundKnownPacks is sent by the client to inform the server which data packs it has.
type ServerboundKnownPacks struct {
	Packs []KnownPack
}

// NewServerboundKnownPacks constructs an empty ServerboundKnownPacks packet.
func NewServerboundKnownPacks() protocol.Packet {
	return &ServerboundKnownPacks{}
}

// ID returns the protocol packet id.
func (p *ServerboundKnownPacks) ID() int32 {
	return int32(packetid.ServerboundConfigSelectKnownPacks)
}

// Decode reads ServerboundKnownPacks fields from buffer.
func (p *ServerboundKnownPacks) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode known_packs count: %w", err)
	}
	p.Packs = make([]KnownPack, count)
	for i := int32(0); i < count; i++ {
		ns, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode known_packs[%d] namespace: %w", i, err)
		}
		id, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode known_packs[%d] id: %w", i, err)
		}
		ver, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode known_packs[%d] version: %w", i, err)
		}
		p.Packs[i] = KnownPack{Namespace: ns, ID: id, Version: ver}
	}
	return nil
}

// Encode writes ServerboundKnownPacks fields to buffer.
func (p *ServerboundKnownPacks) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Packs)))
	for i, pack := range p.Packs {
		if err := buf.WriteString(pack.Namespace); err != nil {
			return fmt.Errorf("encode known_packs[%d] namespace: %w", i, err)
		}
		if err := buf.WriteString(pack.ID); err != nil {
			return fmt.Errorf("encode known_packs[%d] id: %w", i, err)
		}
		if err := buf.WriteString(pack.Version); err != nil {
			return fmt.Errorf("encode known_packs[%d] version: %w", i, err)
		}
	}
	return nil
}
