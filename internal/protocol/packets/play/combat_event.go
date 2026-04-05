package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EnterCombatEvent notifies the client that it has entered combat.
type EnterCombatEvent struct{}

func NewEnterCombatEvent() protocol.Packet { return &EnterCombatEvent{} }

func (p *EnterCombatEvent) ID() int32 {
	return int32(packetid.ClientboundEnterCombatEvent)
}

func (p *EnterCombatEvent) Decode(_ *protocol.Buffer) error { return nil }
func (p *EnterCombatEvent) Encode(_ *protocol.Buffer) error { return nil }

// EndCombatEvent notifies the client that combat has ended.
type EndCombatEvent struct {
	Duration int32
}

func NewEndCombatEvent() protocol.Packet { return &EndCombatEvent{} }

func (p *EndCombatEvent) ID() int32 {
	return int32(packetid.ClientboundEndCombatEvent)
}

func (p *EndCombatEvent) Decode(buf *protocol.Buffer) error {
	var err error
	p.Duration, err = buf.ReadVarInt()
	return err
}

func (p *EndCombatEvent) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.Duration)
	return nil
}
