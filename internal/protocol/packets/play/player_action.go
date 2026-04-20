package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EntityAction action IDs.
const (
	ActionStartSneaking    int32 = 0
	ActionStopSneaking     int32 = 1
	ActionLeaveBed         int32 = 2
	ActionStartSprinting   int32 = 3
	ActionStopSprinting    int32 = 4
	ActionStartJumpHorse   int32 = 5
	ActionStopJumpHorse    int32 = 6
	ActionOpenVehicleInv   int32 = 7
	ActionStartFlyElytra   int32 = 8
)

// ServerboundEntityAction is sent when the player performs an entity action
// (sneak, sprint, leave bed, etc.).
type ServerboundEntityAction struct {
	EntityID  int32
	ActionID  int32
	JumpBoost int32
}

func NewServerboundEntityAction() protocol.Packet { return &ServerboundEntityAction{} }

func (p *ServerboundEntityAction) ID() int32 {
	return int32(packetid.ServerboundEntityAction)
}

func (p *ServerboundEntityAction) Decode(buf *protocol.Buffer) error {
	var err error
	if p.EntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity_action entity_id: %w", err)
	}
	if p.ActionID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity_action action_id: %w", err)
	}
	if p.JumpBoost, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode entity_action jump_boost: %w", err)
	}
	return nil
}

func (p *ServerboundEntityAction) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteVarInt(p.ActionID)
	buf.WriteVarInt(p.JumpBoost)
	return nil
}

// ServerboundUseItem is sent when the client right-clicks with an item (not on a block).
type ServerboundUseItem struct {
	Hand     int32
	Sequence int32
	Yaw      float32
	Pitch    float32
}

func NewServerboundUseItem() protocol.Packet { return &ServerboundUseItem{} }

func (p *ServerboundUseItem) ID() int32 {
	return int32(packetid.ServerboundUseItem)
}

func (p *ServerboundUseItem) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Hand, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode use_item hand: %w", err)
	}
	if p.Sequence, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode use_item sequence: %w", err)
	}
	if p.Yaw, err = buf.ReadFloat32(); err != nil {
		return fmt.Errorf("decode use_item yaw: %w", err)
	}
	if p.Pitch, err = buf.ReadFloat32(); err != nil {
		return fmt.Errorf("decode use_item pitch: %w", err)
	}
	return nil
}

func (p *ServerboundUseItem) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Hand)
	buf.WriteVarInt(p.Sequence)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	return nil
}
