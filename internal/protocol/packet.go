package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync/atomic"
)

var (
	ErrNilPacket             = errors.New("nil packet")
	ErrUnknownPacket         = errors.New("unknown packet")
	ErrInvalidProtocolState  = errors.New("invalid protocol state")
	ErrUnexpectedPayloadData = errors.New("unexpected payload data")
	ErrPacketTooLarge        = errors.New("packet too large")
)

// UUID represents a 128-bit UUID as two unsigned 64-bit integers (most significant first).
type UUID [2]uint64

// GenerateUUID creates a random version-4 UUID.
func GenerateUUID() UUID {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hi := binary.BigEndian.Uint64(buf[0:8])
	lo := binary.BigEndian.Uint64(buf[8:16])
	return UUID{hi, lo}
}

const (
	AnyVersion int32 = -1
)

// Packet is the protocol packet contract used by decoder and encoder.
type Packet interface {
	ID() int32
	Decode(*Buffer) error
	Encode(*Buffer) error
}

// UnknownPacket represents a packet with an unregistered ID that was skipped by the decoder.
type UnknownPacket struct {
	id      int32
	payload []byte
}

// ID returns the raw packet id.
func (p *UnknownPacket) ID() int32 { return p.id }

// Decode is a no-op; payload was already captured.
func (p *UnknownPacket) Decode(_ *Buffer) error { return nil }

// Encode writes the raw payload back.
func (p *UnknownPacket) Encode(buf *Buffer) error {
	if len(p.payload) > 0 {
		buf.WriteBytes(p.payload)
	}
	return nil
}

// PacketFactory constructs a packet instance without reflection.
type PacketFactory func() Packet

// InboundStateTransition is implemented by packets that change state after inbound decode.
type InboundStateTransition interface {
	InboundStateTransition() (State, bool)
}

// OutboundStateTransition is implemented by packets that change state after outbound encode.
type OutboundStateTransition interface {
	OutboundStateTransition() (State, bool)
}

// ProtocolVersionTransition is implemented by packets that update the active protocol version.
type ProtocolVersionTransition interface {
	ProtocolVersionTransition() (int32, bool)
}

// SessionState stores protocol version and protocol state for one connection.
type SessionState struct {
	version atomic.Int32
	state   atomic.Uint32
}

// NewSessionState creates a new protocol session state container.
func NewSessionState(version int32, state State) *SessionState {
	s := &SessionState{}
	s.version.Store(version)
	s.state.Store(uint32(state))
	return s
}

// ProtocolVersion returns the currently active protocol version.
func (s *SessionState) ProtocolVersion() int32 {
	if s == nil {
		return AnyVersion
	}
	return s.version.Load()
}

// SetProtocolVersion updates the active protocol version.
func (s *SessionState) SetProtocolVersion(version int32) {
	if s == nil {
		return
	}
	s.version.Store(version)
}

// State returns the currently active protocol state.
func (s *SessionState) State() State {
	if s == nil {
		return StateHandshake
	}
	return State(s.state.Load())
}

// SetState updates the active protocol state.
func (s *SessionState) SetState(state State) {
	if s == nil {
		return
	}
	s.state.Store(uint32(state))
}
