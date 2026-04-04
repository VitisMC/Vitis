package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// InitializeWorldBorder sets up the initial world border state.
type InitializeWorldBorder struct {
	X                      float64
	Z                      float64
	OldDiameter            float64
	NewDiameter            float64
	Speed                  int64
	PortalTeleportBoundary int32
	WarningBlocks          int32
	WarningTime            int32
}

func NewInitializeWorldBorder() protocol.Packet { return &InitializeWorldBorder{} }
func (p *InitializeWorldBorder) ID() int32 {
	return int32(packetid.ClientboundInitializeWorldBorder)
}
func (p *InitializeWorldBorder) Decode(_ *protocol.Buffer) error { return nil }
func (p *InitializeWorldBorder) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Z)
	buf.WriteFloat64(p.OldDiameter)
	buf.WriteFloat64(p.NewDiameter)
	writeVarLong(buf, p.Speed)
	buf.WriteVarInt(p.PortalTeleportBoundary)
	buf.WriteVarInt(p.WarningBlocks)
	buf.WriteVarInt(p.WarningTime)
	return nil
}

func writeVarLong(buf *protocol.Buffer, value int64) {
	uv := uint64(value)
	for uv >= 0x80 {
		buf.WriteByte(byte(uv&0x7F) | 0x80)
		uv >>= 7
	}
	buf.WriteByte(byte(uv))
}

// SetBorderCenter updates the world border center position.
type SetBorderCenter struct {
	X float64
	Z float64
}

func NewSetBorderCenter() protocol.Packet                  { return &SetBorderCenter{} }
func (p *SetBorderCenter) ID() int32                       { return int32(packetid.ClientboundWorldBorderCenter) }
func (p *SetBorderCenter) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetBorderCenter) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.X)
	buf.WriteFloat64(p.Z)
	return nil
}

// SetBorderSize sets the world border diameter instantly.
type SetBorderSize struct {
	Diameter float64
}

func NewSetBorderSize() protocol.Packet                  { return &SetBorderSize{} }
func (p *SetBorderSize) ID() int32                       { return int32(packetid.ClientboundWorldBorderSize) }
func (p *SetBorderSize) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetBorderSize) Encode(buf *protocol.Buffer) error {
	buf.WriteFloat64(p.Diameter)
	return nil
}

// SetBorderWarningDelay sets the warning time before the border shrinks.
type SetBorderWarningDelay struct {
	WarningTime int32
}

func NewSetBorderWarningDelay() protocol.Packet { return &SetBorderWarningDelay{} }
func (p *SetBorderWarningDelay) ID() int32 {
	return int32(packetid.ClientboundWorldBorderWarningDelay)
}
func (p *SetBorderWarningDelay) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetBorderWarningDelay) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WarningTime)
	return nil
}

// SetBorderWarningDistance sets the warning distance from the border.
type SetBorderWarningDistance struct {
	WarningBlocks int32
}

func NewSetBorderWarningDistance() protocol.Packet { return &SetBorderWarningDistance{} }
func (p *SetBorderWarningDistance) ID() int32 {
	return int32(packetid.ClientboundWorldBorderWarningReach)
}
func (p *SetBorderWarningDistance) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetBorderWarningDistance) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WarningBlocks)
	return nil
}
