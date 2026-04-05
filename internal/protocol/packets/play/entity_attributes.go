package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// AttributeModifier represents a single modifier on an entity attribute.
type AttributeModifier struct {
	ID        string
	Amount    float64
	Operation int8
}

// AttributeProperty represents a single attribute with its base value and modifiers.
type AttributeProperty struct {
	ID        int32
	Value     float64
	Modifiers []AttributeModifier
}

// EntityUpdateAttributes synchronizes entity attributes (health, speed, etc.) to the client.
type EntityUpdateAttributes struct {
	EntityID   int32
	Properties []AttributeProperty
}

func NewEntityUpdateAttributes() protocol.Packet { return &EntityUpdateAttributes{} }

func (p *EntityUpdateAttributes) ID() int32 {
	return int32(packetid.ClientboundEntityUpdateAttributes)
}

func (p *EntityUpdateAttributes) Decode(_ *protocol.Buffer) error { return nil }

func (p *EntityUpdateAttributes) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(int32(len(p.Properties)))
	for _, prop := range p.Properties {
		buf.WriteVarInt(prop.ID)
		buf.WriteFloat64(prop.Value)
		buf.WriteVarInt(int32(len(prop.Modifiers)))
		for _, mod := range prop.Modifiers {
			buf.WriteString(mod.ID)
			buf.WriteFloat64(mod.Amount)
			buf.WriteByte(byte(mod.Operation))
		}
	}
	return nil
}
