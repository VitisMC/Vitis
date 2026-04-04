package protocol

import (
	"fmt"
)

const defaultMaxPacketSize = 2 << 20

// Decoder decodes already-framed packet payloads into typed packet instances.
type Decoder struct {
	registry      *Registry
	maxPacketSize int
}

// NewDecoder builds a packet decoder bound to a registry.
func NewDecoder(registry *Registry, maxPacketSize int) *Decoder {
	if maxPacketSize <= 0 {
		maxPacketSize = defaultMaxPacketSize
	}
	return &Decoder{registry: registry, maxPacketSize: maxPacketSize}
}

// Decode decodes one frame payload according to the session protocol version and state.
func (d *Decoder) Decode(session *SessionState, frame []byte) (Packet, error) {
	if d == nil || d.registry == nil {
		return nil, fmt.Errorf("decode packet: nil decoder registry")
	}
	if session == nil {
		return nil, fmt.Errorf("decode packet: nil session")
	}
	if len(frame) == 0 {
		return nil, fmt.Errorf("decode packet: empty frame")
	}
	if d.maxPacketSize > 0 && len(frame) > d.maxPacketSize {
		return nil, fmt.Errorf("decode packet: %w %d > %d", ErrPacketTooLarge, len(frame), d.maxPacketSize)
	}

	buffer := WrapBuffer(frame)
	packetID, err := buffer.ReadVarInt()
	if err != nil {
		return nil, fmt.Errorf("decode packet id: %w", err)
	}

	factory, ok := d.registry.Lookup(session.ProtocolVersion(), session.State(), DirectionInbound, packetID)
	if !ok {
		return &UnknownPacket{id: packetID, payload: buffer.RemainingBytes()}, nil
	}

	packet := factory()
	if packet == nil {
		return nil, fmt.Errorf("decode packet id=%d: %w", packetID, ErrNilPacket)
	}

	if err := packet.Decode(buffer); err != nil {
		return nil, fmt.Errorf("decode packet state=%s id=%d: %w", session.State(), packetID, err)
	}
	if !buffer.Exhausted() {
		return nil, fmt.Errorf("decode packet state=%s id=%d: %w", session.State(), packetID, ErrUnexpectedPayloadData)
	}

	d.applyProtocolVersionTransition(session, packet)
	if err := d.applyInboundStateTransition(session, packet); err != nil {
		return nil, err
	}

	return packet, nil
}

func (d *Decoder) applyInboundStateTransition(session *SessionState, packet Packet) error {
	transition, ok := packet.(InboundStateTransition)
	if !ok {
		return nil
	}

	nextState, shouldTransition := transition.InboundStateTransition()
	if !shouldTransition {
		return nil
	}
	if !nextState.Valid() {
		return fmt.Errorf("inbound state transition %s: %w", nextState.String(), ErrInvalidProtocolState)
	}

	session.SetState(nextState)
	return nil
}

func (d *Decoder) applyProtocolVersionTransition(session *SessionState, packet Packet) {
	transition, ok := packet.(ProtocolVersionTransition)
	if !ok {
		return
	}

	version, shouldTransition := transition.ProtocolVersionTransition()
	if !shouldTransition {
		return
	}

	session.SetProtocolVersion(version)
}
