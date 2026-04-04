package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// GameEvent is sent by the server for various game events like weather, game mode changes, etc.
type GameEvent struct {
	Event byte
	Value float32
}

// NewGameEvent constructs an empty GameEvent packet.
func NewGameEvent() protocol.Packet {
	return &GameEvent{}
}

// ID returns the protocol packet id.
func (p *GameEvent) ID() int32 {
	return int32(packetid.ClientboundGameStateChange)
}

// Decode reads GameEvent fields from buffer.
func (p *GameEvent) Decode(buf *protocol.Buffer) error {
	event, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode game_event event: %w", err)
	}
	p.Event = event

	value, err := buf.ReadFloat32()
	if err != nil {
		return fmt.Errorf("decode game_event value: %w", err)
	}
	p.Value = value
	return nil
}

// Encode writes GameEvent fields to buffer.
func (p *GameEvent) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteByte(p.Event); err != nil {
		return fmt.Errorf("encode game_event event: %w", err)
	}
	buf.WriteFloat32(p.Value)
	return nil
}
