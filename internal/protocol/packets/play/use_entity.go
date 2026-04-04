package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	UseEntityInteract   int32 = 0
	UseEntityAttack     int32 = 1
	UseEntityInteractAt int32 = 2
)

// UseEntity (serverbound) is sent when the player interacts with an entity.
type UseEntity struct {
	EntityID int32
	Type     int32
	TargetX  float32
	TargetY  float32
	TargetZ  float32
	Hand     int32
	Sneaking bool
}

func NewUseEntity() protocol.Packet { return &UseEntity{} }

func (p *UseEntity) ID() int32 {
	return int32(packetid.ServerboundUseEntity)
}

func (p *UseEntity) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode use entity id: %w", err)
	}
	if p.Type, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode use entity type: %w", err)
	}
	switch p.Type {
	case UseEntityInteract:
		if p.Hand, err = buf.ReadVarInt(); err != nil {
			return fmt.Errorf("decode use entity hand: %w", err)
		}
	case UseEntityAttack:
	case UseEntityInteractAt:
		if p.TargetX, err = buf.ReadFloat32(); err != nil {
			return fmt.Errorf("decode use entity target x: %w", err)
		}
		if p.TargetY, err = buf.ReadFloat32(); err != nil {
			return fmt.Errorf("decode use entity target y: %w", err)
		}
		if p.TargetZ, err = buf.ReadFloat32(); err != nil {
			return fmt.Errorf("decode use entity target z: %w", err)
		}
		if p.Hand, err = buf.ReadVarInt(); err != nil {
			return fmt.Errorf("decode use entity hand: %w", err)
		}
	}
	b, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode use entity sneaking: %w", err)
	}
	p.Sneaking = b != 0
	return nil
}

func (p *UseEntity) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(p.Type)
	switch p.Type {
	case UseEntityInteract:
		buf.WriteVarInt(p.Hand)
	case UseEntityAttack:
	case UseEntityInteractAt:
		buf.WriteFloat32(p.TargetX)
		buf.WriteFloat32(p.TargetY)
		buf.WriteFloat32(p.TargetZ)
		buf.WriteVarInt(p.Hand)
	}
	if p.Sneaking {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
