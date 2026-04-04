package protocol

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	ErrPacketAlreadyRegistered = errors.New("packet already registered")
	ErrNilPacketFactory        = errors.New("nil packet factory")
)

const (
	DirectionInbound Direction = iota
	DirectionOutbound
)

// Direction identifies whether a packet mapping is used for inbound or outbound flow.
type Direction uint8

type registryKey struct {
	version int32
	state   State
	id      int32
}

type registrySnapshot struct {
	inbound  map[registryKey]PacketFactory
	outbound map[registryKey]PacketFactory
}

// Registry stores packet constructors keyed by version, protocol state, direction and packet id.
type Registry struct {
	mu       sync.Mutex
	snapshot atomic.Value
}

// NewRegistry constructs an empty packet registry.
func NewRegistry() *Registry {
	r := &Registry{}
	r.snapshot.Store(registrySnapshot{
		inbound:  make(map[registryKey]PacketFactory),
		outbound: make(map[registryKey]PacketFactory),
	})
	return r
}

// Register adds a packet constructor for a specific version, state, direction and packet id.
func (r *Registry) Register(version int32, state State, direction Direction, packetID int32, factory PacketFactory) error {
	if r == nil {
		return fmt.Errorf("register packet: nil registry")
	}
	if !state.Valid() {
		return fmt.Errorf("register packet %d: %w %s", packetID, ErrInvalidProtocolState, state.String())
	}
	if packetID < 0 {
		return fmt.Errorf("register packet: negative packet id %d", packetID)
	}
	if factory == nil {
		return ErrNilPacketFactory
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.snapshot.Load().(registrySnapshot)
	next := registrySnapshot{
		inbound:  cloneFactoryMap(current.inbound),
		outbound: cloneFactoryMap(current.outbound),
	}

	target := next.inbound
	if direction == DirectionOutbound {
		target = next.outbound
	}

	key := registryKey{version: version, state: state, id: packetID}
	if _, exists := target[key]; exists {
		return fmt.Errorf("register packet state=%s direction=%d id=%d version=%d: %w", state, direction, packetID, version, ErrPacketAlreadyRegistered)
	}

	target[key] = factory
	r.snapshot.Store(next)
	return nil
}

// RegisterPacket adds packet mapping using packet.ID() from the factory output.
func (r *Registry) RegisterPacket(version int32, state State, direction Direction, factory PacketFactory) error {
	if factory == nil {
		return ErrNilPacketFactory
	}
	packet := factory()
	if packet == nil {
		return ErrNilPacketFactory
	}
	return r.Register(version, state, direction, packet.ID(), factory)
}

// Lookup resolves a packet constructor for version, state and packet id.
func (r *Registry) Lookup(version int32, state State, direction Direction, packetID int32) (PacketFactory, bool) {
	if r == nil {
		return nil, false
	}

	current := r.snapshot.Load().(registrySnapshot)
	table := current.inbound
	if direction == DirectionOutbound {
		table = current.outbound
	}

	key := registryKey{version: version, state: state, id: packetID}
	if factory, ok := table[key]; ok {
		return factory, true
	}

	fallback := registryKey{version: AnyVersion, state: state, id: packetID}
	factory, ok := table[fallback]
	return factory, ok
}

func cloneFactoryMap(src map[registryKey]PacketFactory) map[registryKey]PacketFactory {
	dst := make(map[registryKey]PacketFactory, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
