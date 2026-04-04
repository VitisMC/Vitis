package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SyncEntityPosition teleports an entity to absolute coordinates.
// This is the 1.21.4 replacement for the old EntityTeleport for position sync.
type SyncEntityPosition struct {
	EntityID int32
	X        float64
	Y        float64
	Z        float64
	Yaw      float32
	Pitch    float32
	OnGround bool
}

func NewSyncEntityPosition() protocol.Packet { return &SyncEntityPosition{} }

func (p *SyncEntityPosition) ID() int32 {
	return int32(packetid.ClientboundSyncEntityPosition)
}

func (p *SyncEntityPosition) Decode(buf *protocol.Buffer) error { return nil }

func (p *SyncEntityPosition) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.EntityID)
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Y)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat64(0)
	buf.WriteFloat64(0)
	buf.WriteFloat64(0)
	buf.WriteFloat32(p.Yaw)
	buf.WriteFloat32(p.Pitch)
	if p.OnGround {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
