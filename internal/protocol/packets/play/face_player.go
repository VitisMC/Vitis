package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// FacePlayer rotates the client to face a specific position or entity.
type FacePlayer struct {
	FeetOrEyes    int32
	TargetX       float64
	TargetY       float64
	TargetZ       float64
	IsEntity      bool
	EntityID      int32
	EntityFeetOrEyes int32
}

func NewFacePlayer() protocol.Packet { return &FacePlayer{} }

func (p *FacePlayer) ID() int32 {
	return int32(packetid.ClientboundFacePlayer)
}

func (p *FacePlayer) Decode(buf *protocol.Buffer) error {
	var err error
	if p.FeetOrEyes, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.TargetX, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.TargetY, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.TargetZ, err = buf.ReadFloat64(); err != nil {
		return err
	}
	if p.IsEntity, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.IsEntity {
		if p.EntityID, err = buf.ReadVarInt(); err != nil {
			return err
		}
		p.EntityFeetOrEyes, err = buf.ReadVarInt()
	}
	return err
}

func (p *FacePlayer) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.FeetOrEyes)
	buf.WriteFloat64(p.TargetX)
	buf.WriteFloat64(p.TargetY)
	buf.WriteFloat64(p.TargetZ)
	buf.WriteBool(p.IsEntity)
	if p.IsEntity {
		buf.WriteVarInt(p.EntityID)
		buf.WriteVarInt(p.EntityFeetOrEyes)
	}
	return nil
}
