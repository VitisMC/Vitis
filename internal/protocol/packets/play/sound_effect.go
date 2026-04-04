package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SoundEffect plays a sound at a specific position.
type SoundEffect struct {
	SoundID       int32
	SoundCategory int32
	X             int32
	Y             int32
	Z             int32
	Volume        float32
	Pitch         float32
	Seed          int64
}

func NewSoundEffect() protocol.Packet { return &SoundEffect{} }

func (p *SoundEffect) ID() int32 { return int32(packetid.ClientboundSoundEffect) }

func (p *SoundEffect) Decode(buf *protocol.Buffer) error { return nil }

func (p *SoundEffect) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SoundID)
	buf.WriteVarInt(p.SoundCategory)
	buf.WriteInt32(p.X)
	buf.WriteInt32(p.Y)
	buf.WriteInt32(p.Z)
	buf.WriteFloat32(p.Volume)
	buf.WriteFloat32(p.Pitch)
	buf.WriteInt64(p.Seed)
	return nil
}

// EntityAnimation plays an animation on an entity (swing arm, take damage, etc).
type EntityAnimation struct {
	EntityID  int32
	Animation byte
}

const (
	AnimationSwingMainArm byte = 0
	AnimationTakeDamage   byte = 1
	AnimationLeaveBed     byte = 2
	AnimationSwingOffhand byte = 3
	AnimationCritical     byte = 4
	AnimationMagicCrit    byte = 5
)

func NewEntityAnimation() protocol.Packet { return &EntityAnimation{} }

func (p *EntityAnimation) ID() int32 { return int32(packetid.ClientboundAnimation) }

func (p *EntityAnimation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.Animation, err = buf.ReadByte()
	return err
}

func (p *EntityAnimation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteByte(p.Animation)
	return nil
}

// SetExperience sends the player's experience bar state.
type SetExperience struct {
	ExperienceBar   float32
	ExperienceLevel int32
	TotalExperience int32
}

func NewSetExperience() protocol.Packet { return &SetExperience{} }

func (p *SetExperience) ID() int32 { return int32(packetid.ClientboundExperience) }

func (p *SetExperience) Decode(buf *protocol.Buffer) error {
	var err error
	if p.ExperienceBar, err = buf.ReadFloat32(); err != nil {
		return err
	}
	if p.ExperienceLevel, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.TotalExperience, err = buf.ReadVarInt()
	return err
}

func (p *SetExperience) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat32(p.ExperienceBar)
	buf.WriteVarInt(p.ExperienceLevel)
	buf.WriteVarInt(p.TotalExperience)
	return nil
}

// Respawn is sent when the player changes dimension or respawns.
type Respawn struct {
	DimensionType    int32
	DimensionName    string
	HashedSeed       int64
	GameMode         byte
	PreviousGameMode int8
	IsDebug          bool
	IsFlat           bool
	HasDeathLocation bool
	PortalCooldown   int32
	SeaLevel         int32
	DataKept         byte
}

func NewRespawn() protocol.Packet { return &Respawn{} }

func (p *Respawn) ID() int32 { return int32(packetid.ClientboundRespawn) }

func (p *Respawn) Decode(buf *protocol.Buffer) error { return nil }

func (p *Respawn) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.DimensionType)
	buf.WriteString(p.DimensionName)
	buf.WriteInt64(p.HashedSeed)
	buf.WriteByte(p.GameMode)
	buf.WriteByte(byte(p.PreviousGameMode))
	if p.IsDebug {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	if p.IsFlat {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	if p.HasDeathLocation {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	buf.WriteVarInt(p.PortalCooldown)
	buf.WriteVarInt(p.SeaLevel)
	buf.WriteByte(p.DataKept)
	return nil
}

// EntityVelocity sets an entity's velocity.
type EntityVelocity struct {
	EntityID  int32
	VelocityX int16
	VelocityY int16
	VelocityZ int16
}

func NewEntityVelocity() protocol.Packet { return &EntityVelocity{} }

func (p *EntityVelocity) ID() int32 { return int32(packetid.ClientboundEntityVelocity) }

func (p *EntityVelocity) Decode(buf *protocol.Buffer) error { return nil }

func (p *EntityVelocity) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteInt16(p.VelocityX)
	buf.WriteInt16(p.VelocityY)
	buf.WriteInt16(p.VelocityZ)
	return nil
}
