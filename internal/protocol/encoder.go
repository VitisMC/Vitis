package protocol

import "fmt"

const defaultInitialEncodeCapacity = 256

// Encoder encodes typed packets into length-prefixed Minecraft frames.
type Encoder struct {
	registry      *Registry
	maxPacketSize int
}

// NewEncoder builds a packet encoder bound to a registry.
func NewEncoder(registry *Registry, maxPacketSize int) *Encoder {
	if maxPacketSize <= 0 {
		maxPacketSize = defaultMaxPacketSize
	}
	return &Encoder{registry: registry, maxPacketSize: maxPacketSize}
}

// Encode writes packet id + payload and prepends packet length to produce one frame.
func (e *Encoder) Encode(session *SessionState, packet Packet, reuse *Buffer) ([]byte, error) {
	if e == nil || e.registry == nil {
		return nil, fmt.Errorf("encode packet: nil encoder registry")
	}
	if session == nil {
		return nil, fmt.Errorf("encode packet: nil session")
	}
	if packet == nil {
		return nil, ErrNilPacket
	}

	packetID := packet.ID()
	if packetID < 0 {
		return nil, fmt.Errorf("encode packet: negative packet id %d", packetID)
	}

	if _, ok := e.registry.Lookup(session.ProtocolVersion(), session.State(), DirectionOutbound, packetID); !ok {
		return nil, fmt.Errorf("encode packet state=%s version=%d id=%d: %w", session.State(), session.ProtocolVersion(), packetID, ErrUnknownPacket)
	}

	buf := reuse
	if buf == nil {
		buf = NewBuffer(defaultInitialEncodeCapacity)
	}
	buf.Reset()

	prefix := maxVarIntBytes
	buf.Ensure(prefix)
	buf.w = prefix

	buf.WriteVarInt(packetID)
	if err := packet.Encode(buf); err != nil {
		return nil, fmt.Errorf("encode packet state=%s id=%d: %w", session.State(), packetID, err)
	}

	payloadLen := buf.w - prefix
	if payloadLen < 0 {
		return nil, fmt.Errorf("encode packet: invalid payload length")
	}
	if e.maxPacketSize > 0 && payloadLen > e.maxPacketSize {
		return nil, fmt.Errorf("encode packet id=%d: %w %d > %d", packetID, ErrPacketTooLarge, payloadLen, e.maxPacketSize)
	}

	header := [maxVarIntBytes]byte{}
	headerLen := EncodeVarInt(header[:], int32(payloadLen))
	copy(buf.data[headerLen:], buf.data[prefix:buf.w])
	copy(buf.data[:headerLen], header[:headerLen])
	buf.r = 0
	buf.w = headerLen + payloadLen

	e.applyOutboundTransitions(session, packet)
	return buf.Bytes(), nil
}

func (e *Encoder) applyOutboundTransitions(session *SessionState, packet Packet) {
	if transition, ok := packet.(ProtocolVersionTransition); ok {
		if version, shouldTransition := transition.ProtocolVersionTransition(); shouldTransition {
			session.SetProtocolVersion(version)
		}
	}

	if transition, ok := packet.(OutboundStateTransition); ok {
		nextState, shouldTransition := transition.OutboundStateTransition()
		if !shouldTransition {
			return
		}
		if nextState.Valid() {
			session.SetState(nextState)
		}
	}
}
