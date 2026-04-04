package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const (
	TeamActionCreate      int8 = 0
	TeamActionRemove      int8 = 1
	TeamActionUpdateInfo  int8 = 2
	TeamActionAddEntities int8 = 3
	TeamActionRemEntities int8 = 4
)

type UpdateTeams struct {
	TeamName          string
	Action            int8
	DisplayName       string
	FriendlyFlags     int8
	NameTagVisibility string
	CollisionRule     string
	Color             int32
	Prefix            string
	Suffix            string
	Entities          []string
}

func NewUpdateTeams() protocol.Packet { return &UpdateTeams{} }

func (p *UpdateTeams) ID() int32 { return int32(packetid.ClientboundTeams) }

func (p *UpdateTeams) Decode(_ *protocol.Buffer) error { return nil }

func (p *UpdateTeams) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.TeamName)
	buf.WriteByte(byte(p.Action))

	if p.Action == 0 || p.Action == 2 {
		writeNBTTextComponent(buf, p.DisplayName)
		buf.WriteByte(byte(p.FriendlyFlags))
		buf.WriteString(p.NameTagVisibility)
		buf.WriteString(p.CollisionRule)
		buf.WriteVarInt(p.Color)
		writeNBTTextComponent(buf, p.Prefix)
		writeNBTTextComponent(buf, p.Suffix)
	}

	if p.Action == 0 || p.Action == 3 || p.Action == 4 {
		buf.WriteVarInt(int32(len(p.Entities)))
		for _, e := range p.Entities {
			buf.WriteString(e)
		}
	}
	return nil
}

type ResetScore struct {
	EntityName    string
	HasObjective  bool
	ObjectiveName string
}

func NewResetScore() protocol.Packet { return &ResetScore{} }

func (p *ResetScore) ID() int32 { return int32(packetid.ClientboundResetScore) }

func (p *ResetScore) Decode(_ *protocol.Buffer) error { return nil }

func (p *ResetScore) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.EntityName)
	if p.HasObjective {
		buf.WriteByte(1)
		buf.WriteString(p.ObjectiveName)
	} else {
		buf.WriteByte(0)
	}
	return nil
}
