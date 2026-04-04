package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// UpdateEntityPosition is the clientbound relative movement packet.
type UpdateEntityPosition struct {
	EntityID int32
	DeltaX   int16
	DeltaY   int16
	DeltaZ   int16
	OnGround bool
}

// NewUpdateEntityPosition constructs an empty UpdateEntityPosition packet.
func NewUpdateEntityPosition() protocol.Packet {
	return &UpdateEntityPosition{}
}

// ID returns the protocol packet id.
func (p *UpdateEntityPosition) ID() int32 {
	return int32(packetid.ClientboundRelEntityMove)
}

// Decode reads UpdateEntityPosition fields from buffer.
func (p *UpdateEntityPosition) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity position id: %w", err)
	}
	if p.DeltaX, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode entity position dx: %w", err)
	}
	if p.DeltaY, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode entity position dy: %w", err)
	}
	if p.DeltaZ, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode entity position dz: %w", err)
	}
	if p.OnGround, err = buf.ReadBool(); err != nil {
		return fmt.Errorf("decode entity position on_ground: %w", err)
	}
	return nil
}

// Encode writes UpdateEntityPosition fields to buffer.
func (p *UpdateEntityPosition) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteInt16(p.DeltaX)
	buf.WriteInt16(p.DeltaY)
	buf.WriteInt16(p.DeltaZ)
	buf.WriteBool(p.OnGround)
	return nil
}

// UpdateEntityPositionAndRotation is the clientbound relative move + rotation packet.
type UpdateEntityPositionAndRotation struct {
	EntityID int32
	DeltaX   int16
	DeltaY   int16
	DeltaZ   int16
	Yaw      byte
	Pitch    byte
	OnGround bool
}

// NewUpdateEntityPositionAndRotation constructs an empty UpdateEntityPositionAndRotation packet.
func NewUpdateEntityPositionAndRotation() protocol.Packet {
	return &UpdateEntityPositionAndRotation{}
}

// ID returns the protocol packet id.
func (p *UpdateEntityPositionAndRotation) ID() int32 {
	return int32(packetid.ClientboundEntityMoveLook)
}

// Decode reads UpdateEntityPositionAndRotation fields from buffer.
func (p *UpdateEntityPositionAndRotation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity pos+rot id: %w", err)
	}
	if p.DeltaX, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode entity pos+rot dx: %w", err)
	}
	if p.DeltaY, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode entity pos+rot dy: %w", err)
	}
	if p.DeltaZ, err = buf.ReadInt16(); err != nil {
		return fmt.Errorf("decode entity pos+rot dz: %w", err)
	}
	if p.Yaw, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode entity pos+rot yaw: %w", err)
	}
	if p.Pitch, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode entity pos+rot pitch: %w", err)
	}
	if p.OnGround, err = buf.ReadBool(); err != nil {
		return fmt.Errorf("decode entity pos+rot on_ground: %w", err)
	}
	return nil
}

// Encode writes UpdateEntityPositionAndRotation fields to buffer.
func (p *UpdateEntityPositionAndRotation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteInt16(p.DeltaX)
	buf.WriteInt16(p.DeltaY)
	buf.WriteInt16(p.DeltaZ)
	_ = buf.WriteByte(p.Yaw)
	_ = buf.WriteByte(p.Pitch)
	buf.WriteBool(p.OnGround)
	return nil
}

// UpdateEntityRotation is the clientbound rotation-only packet.
type UpdateEntityRotation struct {
	EntityID int32
	Yaw      byte
	Pitch    byte
	OnGround bool
}

// NewUpdateEntityRotation constructs an empty UpdateEntityRotation packet.
func NewUpdateEntityRotation() protocol.Packet {
	return &UpdateEntityRotation{}
}

// ID returns the protocol packet id.
func (p *UpdateEntityRotation) ID() int32 {
	return int32(packetid.ClientboundEntityLook)
}

// Decode reads UpdateEntityRotation fields from buffer.
func (p *UpdateEntityRotation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity rotation id: %w", err)
	}
	if p.Yaw, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode entity rotation yaw: %w", err)
	}
	if p.Pitch, err = buf.ReadByte(); err != nil {
		return fmt.Errorf("decode entity rotation pitch: %w", err)
	}
	if p.OnGround, err = buf.ReadBool(); err != nil {
		return fmt.Errorf("decode entity rotation on_ground: %w", err)
	}
	return nil
}

// Encode writes UpdateEntityRotation fields to buffer.
func (p *UpdateEntityRotation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	_ = buf.WriteByte(p.Yaw)
	_ = buf.WriteByte(p.Pitch)
	buf.WriteBool(p.OnGround)
	return nil
}
