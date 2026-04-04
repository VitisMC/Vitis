package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ScoreboardObjective creates, removes, or updates a scoreboard objective.
type ScoreboardObjective struct {
	Name       string
	Mode       int8
	Value      string
	Type       int32
	HasNumber  bool
	NumberFmt  int32
}

func NewScoreboardObjective() protocol.Packet { return &ScoreboardObjective{} }
func (p *ScoreboardObjective) ID() int32 {
	return int32(packetid.ClientboundScoreboardObjective)
}
func (p *ScoreboardObjective) Decode(_ *protocol.Buffer) error { return nil }
func (p *ScoreboardObjective) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Name)
	buf.WriteByte(byte(p.Mode))
	if p.Mode == 0 || p.Mode == 2 {
		writeNBTTextComponent(buf, p.Value)
		buf.WriteVarInt(p.Type)
		if p.HasNumber {
			buf.WriteByte(1)
			buf.WriteVarInt(p.NumberFmt)
		} else {
			buf.WriteByte(0)
		}
	}
	return nil
}

// DisplayObjective sets which objective is displayed in a particular slot.
type DisplayObjective struct {
	Position  int32
	ScoreName string
}

func NewDisplayObjective() protocol.Packet { return &DisplayObjective{} }
func (p *DisplayObjective) ID() int32 {
	return int32(packetid.ClientboundScoreboardDisplayObjective)
}
func (p *DisplayObjective) Decode(_ *protocol.Buffer) error { return nil }
func (p *DisplayObjective) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Position)
	buf.WriteString(p.ScoreName)
	return nil
}

// UpdateScore updates or removes a score on a scoreboard objective.
type UpdateScore struct {
	EntityName    string
	ObjectiveName string
	Value         int32
	HasDisplay    bool
	DisplayName   string
	HasNumberFmt  bool
	NumberFmt     int32
}

func NewUpdateScore() protocol.Packet { return &UpdateScore{} }
func (p *UpdateScore) ID() int32      { return int32(packetid.ClientboundScoreboardScore) }
func (p *UpdateScore) Decode(_ *protocol.Buffer) error { return nil }
func (p *UpdateScore) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.EntityName)
	buf.WriteString(p.ObjectiveName)
	buf.WriteVarInt(p.Value)
	if p.HasDisplay {
		buf.WriteByte(1)
		comp := nbt.NewCompound()
		comp.PutString("text", p.DisplayName)
		enc := nbt.NewEncoder(64)
		_ = enc.WriteRootCompound(comp)
		buf.WriteBytes(enc.Bytes())
	} else {
		buf.WriteByte(0)
	}
	if p.HasNumberFmt {
		buf.WriteByte(1)
		buf.WriteVarInt(p.NumberFmt)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
