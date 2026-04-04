package handshake

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
)

const packetID int32 = 0x00

// Handshake is the initial packet used to negotiate protocol version and next state.
type Handshake struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	NextState       protocol.State
}

// New creates a zero-value handshake packet.
func New() protocol.Packet {
	return &Handshake{}
}

// ID returns the protocol packet id.
func (p *Handshake) ID() int32 {
	return packetID
}

// Decode reads handshake packet fields from buffer.
func (p *Handshake) Decode(buf *protocol.Buffer) error {
	version, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode protocol version: %w", err)
	}

	address, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode server address: %w", err)
	}

	port, err := buf.ReadUint16()
	if err != nil {
		return fmt.Errorf("decode server port: %w", err)
	}

	next, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode next state: %w", err)
	}

	nextState, err := decodeNextState(next)
	if err != nil {
		return err
	}

	p.ProtocolVersion = version
	p.ServerAddress = address
	p.ServerPort = port
	p.NextState = nextState
	return nil
}

// Encode writes handshake packet fields to buffer.
func (p *Handshake) Encode(buf *protocol.Buffer) error {
	nextState, err := encodeNextState(p.NextState)
	if err != nil {
		return err
	}

	buf.WriteVarInt(p.ProtocolVersion)
	if err := buf.WriteString(p.ServerAddress); err != nil {
		return fmt.Errorf("encode server address: %w", err)
	}
	buf.WriteUint16(p.ServerPort)
	buf.WriteVarInt(nextState)
	return nil
}

// InboundStateTransition updates session state from handshake next-state value.
func (p *Handshake) InboundStateTransition() (protocol.State, bool) {
	return p.NextState, true
}

// ProtocolVersionTransition updates session protocol version from handshake value.
func (p *Handshake) ProtocolVersionTransition() (int32, bool) {
	return p.ProtocolVersion, true
}

func decodeNextState(value int32) (protocol.State, error) {
	switch value {
	case 1:
		return protocol.StateStatus, nil
	case 2:
		return protocol.StateLogin, nil
	case 3:
		return protocol.StateLogin, nil
	default:
		return protocol.StateHandshake, fmt.Errorf("decode next state %d: %w", value, protocol.ErrInvalidProtocolState)
	}
}

func encodeNextState(state protocol.State) (int32, error) {
	switch state {
	case protocol.StateStatus:
		return 1, nil
	case protocol.StateLogin:
		return 2, nil
	default:
		return 0, fmt.Errorf("encode next state %s: %w", state.String(), protocol.ErrInvalidProtocolState)
	}
}
