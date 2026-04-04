package login

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// LoginProperty holds a single player property entry in the Login Success packet.
type LoginProperty struct {
	Name      string
	Value     string
	Signature string
	HasSig    bool
}

// LoginSuccess confirms authentication and instructs the client to acknowledge before transitioning.
type LoginSuccess struct {
	UUID       protocol.UUID
	Name       string
	Properties []LoginProperty
}

// NewLoginSuccess constructs an empty LoginSuccess packet.
func NewLoginSuccess() protocol.Packet {
	return &LoginSuccess{}
}

// ID returns the protocol packet id.
func (p *LoginSuccess) ID() int32 {
	return int32(packetid.ClientboundLoginSuccess)
}

// Decode reads LoginSuccess fields from buffer.
func (p *LoginSuccess) Decode(buf *protocol.Buffer) error {
	uuid, err := buf.ReadUUID()
	if err != nil {
		return fmt.Errorf("decode uuid: %w", err)
	}
	name, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode username: %w", err)
	}

	propCount, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode property count: %w", err)
	}
	if propCount < 0 {
		return fmt.Errorf("decode property count: %w", protocol.ErrInvalidLength)
	}

	props := make([]LoginProperty, propCount)
	for i := int32(0); i < propCount; i++ {
		pName, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode property[%d] name: %w", i, err)
		}
		pValue, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode property[%d] value: %w", i, err)
		}
		hasSig, err := buf.ReadBool()
		if err != nil {
			return fmt.Errorf("decode property[%d] has_signature: %w", i, err)
		}
		prop := LoginProperty{Name: pName, Value: pValue, HasSig: hasSig}
		if hasSig {
			sig, err := buf.ReadString()
			if err != nil {
				return fmt.Errorf("decode property[%d] signature: %w", i, err)
			}
			prop.Signature = sig
		}
		props[i] = prop
	}

	p.UUID = uuid
	p.Name = name
	p.Properties = props
	return nil
}

// Encode writes LoginSuccess fields to buffer.
func (p *LoginSuccess) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.UUID)
	if err := buf.WriteString(p.Name); err != nil {
		return fmt.Errorf("encode username: %w", err)
	}

	buf.WriteVarInt(int32(len(p.Properties)))
	for i, prop := range p.Properties {
		if err := buf.WriteString(prop.Name); err != nil {
			return fmt.Errorf("encode property[%d] name: %w", i, err)
		}
		if err := buf.WriteString(prop.Value); err != nil {
			return fmt.Errorf("encode property[%d] value: %w", i, err)
		}
		buf.WriteBool(prop.HasSig)
		if prop.HasSig {
			if err := buf.WriteString(prop.Signature); err != nil {
				return fmt.Errorf("encode property[%d] signature: %w", i, err)
			}
		}
	}
	return nil
}
