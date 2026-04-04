package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	BossBarActionAdd      int32 = 0
	BossBarActionRemove   int32 = 1
	BossBarActionHealth   int32 = 2
	BossBarActionTitle    int32 = 3
	BossBarActionStyle    int32 = 4
	BossBarActionFlags    int32 = 5
)

// BossBar manages boss bar display.
type BossBar struct {
	UUID    protocol.UUID
	Action  int32
	Title   string
	Health  float32
	Color   int32
	Division int32
	Flags   byte
}

func NewBossBar() protocol.Packet { return &BossBar{} }
func (p *BossBar) ID() int32      { return int32(packetid.ClientboundBossBar) }
func (p *BossBar) Decode(_ *protocol.Buffer) error { return nil }
func (p *BossBar) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.UUID)
	buf.WriteVarInt(p.Action)
	switch p.Action {
	case BossBarActionAdd:
		comp := nbt.NewCompound()
		comp.PutString("text", p.Title)
		enc := nbt.NewEncoder(64)
		_ = enc.WriteRootCompound(comp)
		buf.WriteBytes(enc.Bytes())
		buf.WriteFloat32(p.Health)
		buf.WriteVarInt(p.Color)
		buf.WriteVarInt(p.Division)
		buf.WriteByte(p.Flags)
	case BossBarActionRemove:
	case BossBarActionHealth:
		buf.WriteFloat32(p.Health)
	case BossBarActionTitle:
		comp := nbt.NewCompound()
		comp.PutString("text", p.Title)
		enc := nbt.NewEncoder(64)
		_ = enc.WriteRootCompound(comp)
		buf.WriteBytes(enc.Bytes())
	case BossBarActionStyle:
		buf.WriteVarInt(p.Color)
		buf.WriteVarInt(p.Division)
	case BossBarActionFlags:
		buf.WriteByte(p.Flags)
	}
	return nil
}
