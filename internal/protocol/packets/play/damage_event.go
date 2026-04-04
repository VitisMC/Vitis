package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// DamageEvent notifies the client that an entity took damage.
type DamageEvent struct {
	EntityID       int32
	SourceTypeID   int32
	SourceCauseID  int32
	SourceDirectID int32
	HasSourcePos   bool
	SourceX        float64
	SourceY        float64
	SourceZ        float64
}

func NewDamageEvent() protocol.Packet { return &DamageEvent{} }

func (p *DamageEvent) ID() int32 { return int32(packetid.ClientboundDamageEvent) }

func (p *DamageEvent) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.SourceTypeID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.SourceCauseID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.SourceDirectID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.HasSourcePos = b != 0
	if p.HasSourcePos {
		if p.SourceX, err = buf.ReadFloat64(); err != nil {
			return err
		}
		if p.SourceY, err = buf.ReadFloat64(); err != nil {
			return err
		}
		if p.SourceZ, err = buf.ReadFloat64(); err != nil {
			return err
		}
	}
	return nil
}

func (p *DamageEvent) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(p.SourceTypeID)
	buf.WriteVarInt(p.SourceCauseID)
	buf.WriteVarInt(p.SourceDirectID)
	if p.HasSourcePos {
		buf.WriteByte(1)
		buf.WriteFloat64(p.SourceX)
		buf.WriteFloat64(p.SourceY)
		buf.WriteFloat64(p.SourceZ)
	} else {
		buf.WriteByte(0)
	}
	return nil
}

// HurtAnimation sends a hurt (red flash) animation to the client.
type HurtAnimation struct {
	EntityID int32
	Yaw      float32
}

func NewHurtAnimation() protocol.Packet { return &HurtAnimation{} }

func (p *HurtAnimation) ID() int32 { return int32(packetid.ClientboundHurtAnimation) }

func (p *HurtAnimation) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.Yaw, err = buf.ReadFloat32()
	return err
}

func (p *HurtAnimation) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteFloat32(p.Yaw)
	return nil
}

// DeathCombatEvent notifies the client that the player died, showing the death screen.
type DeathCombatEvent struct {
	PlayerID int32
	Message  string
}

func NewDeathCombatEvent() protocol.Packet { return &DeathCombatEvent{} }

func (p *DeathCombatEvent) ID() int32 { return int32(packetid.ClientboundDeathCombatEvent) }

func (p *DeathCombatEvent) Decode(buf *protocol.Buffer) error {
	var err error
	if p.PlayerID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.Message, err = buf.ReadString()
	return err
}

func (p *DeathCombatEvent) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.PlayerID)
	comp := nbt.NewCompound()
	comp.PutString("text", p.Message)
	enc := nbt.NewEncoder(128)
	if err := enc.WriteRootCompound(comp); err != nil {
		return err
	}
	buf.WriteBytes(enc.Bytes())
	return nil
}
