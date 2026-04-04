package session

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
)

// Manager defines lifecycle and lookup operations for active sessions.
type Manager interface {
	Create(conn *network.Conn) (Session, error)
	CreateWithConnection(conn Connection, networkSessionID uint64) (Session, error)
	Get(id uint64) (Session, bool)
	GetByNetworkSessionID(networkSessionID uint64) (Session, bool)
	Remove(id uint64)
	Count() int64
}

// ManagerConfig configures default behavior for newly created sessions.
type ManagerConfig struct {
	Registry       *protocol.Registry
	Router         PacketRouter
	WorkerPool     *network.WorkerPool
	InitialVersion int32
	InitialState   protocol.State
	Events         Events
}

// DefaultManager is the production session manager implementation.
type DefaultManager struct {
	cfg ManagerConfig

	nextID atomic.Uint64
	count  atomic.Int64

	sessionsByID        sync.Map
	sessionsByNetworkID sync.Map
}

// NewManager creates a session manager with protocol registry and optional router.
func NewManager(cfg ManagerConfig) (*DefaultManager, error) {
	if cfg.Registry == nil {
		return nil, ErrNilRegistry
	}
	if !cfg.InitialState.Valid() {
		cfg.InitialState = protocol.StateHandshake
	}
	if cfg.InitialVersion == 0 {
		cfg.InitialVersion = protocol.AnyVersion
	}

	return &DefaultManager{cfg: cfg}, nil
}

// Create creates and stores a session from a network connection.
func (m *DefaultManager) Create(conn *network.Conn) (Session, error) {
	if conn == nil {
		return nil, ErrNilConnection
	}
	networkSession := conn.Session()
	if networkSession == nil {
		return nil, fmt.Errorf("create session: nil network session")
	}
	return m.CreateWithConnection(conn, networkSession.ID())
}

// CreateWithConnection creates and stores a session from a generic connection reference.
func (m *DefaultManager) CreateWithConnection(conn Connection, networkSessionID uint64) (Session, error) {
	if m == nil {
		return nil, fmt.Errorf("create session: nil manager")
	}
	if conn == nil {
		return nil, ErrNilConnection
	}
	if networkSessionID == 0 {
		return nil, fmt.Errorf("create session: invalid network session id")
	}

	sessionID := m.nextID.Add(1)
	s, err := New(Config{
		ID:               sessionID,
		NetworkSessionID: networkSessionID,
		Connection:       conn,
		Registry:         m.cfg.Registry,
		Router:           m.cfg.Router,
		WorkerPool:       m.cfg.WorkerPool,
		InitialVersion:   m.cfg.InitialVersion,
		InitialState:     m.cfg.InitialState,
		Events:           m.cfg.Events,
	})
	if err != nil {
		return nil, err
	}

	if _, loaded := m.sessionsByID.LoadOrStore(sessionID, s); loaded {
		return nil, fmt.Errorf("create session id=%d: %w", sessionID, ErrSessionAlreadyExists)
	}
	if existing, loaded := m.sessionsByNetworkID.LoadOrStore(networkSessionID, s); loaded {
		m.sessionsByID.Delete(sessionID)
		existingSession, _ := existing.(Session)
		if existingSession != nil {
			return nil, fmt.Errorf("create session network_id=%d existing_id=%d: %w", networkSessionID, existingSession.ID(), ErrSessionAlreadyExists)
		}
		return nil, fmt.Errorf("create session network_id=%d: %w", networkSessionID, ErrSessionAlreadyExists)
	}
	m.count.Add(1)

	go func(id uint64, netID uint64, session Session) {
		<-session.Context().Done()
		m.removeByIdentifiers(id, netID)
	}(sessionID, networkSessionID, s)

	return s, nil
}

// Get returns a session by internal session id.
func (m *DefaultManager) Get(id uint64) (Session, bool) {
	if m == nil {
		return nil, false
	}
	value, ok := m.sessionsByID.Load(id)
	if !ok {
		return nil, false
	}
	s, ok := value.(Session)
	if !ok {
		return nil, false
	}
	return s, true
}

// GetByNetworkSessionID returns a session by network session id.
func (m *DefaultManager) GetByNetworkSessionID(networkSessionID uint64) (Session, bool) {
	if m == nil {
		return nil, false
	}
	value, ok := m.sessionsByNetworkID.Load(networkSessionID)
	if !ok {
		return nil, false
	}
	s, ok := value.(Session)
	if !ok {
		return nil, false
	}
	return s, true
}

// Remove removes a session by internal id.
func (m *DefaultManager) Remove(id uint64) {
	if m == nil {
		return
	}
	value, ok := m.sessionsByID.Load(id)
	if !ok {
		return
	}
	s, ok := value.(Session)
	if !ok {
		return
	}
	m.removeByIdentifiers(id, s.NetworkSessionID())
}

// Count returns currently tracked session count.
func (m *DefaultManager) Count() int64 {
	if m == nil {
		return 0
	}
	return m.count.Load()
}

// CountOnlinePlayers returns active in-game player count for status reporting.
func (m *DefaultManager) CountOnlinePlayers() int64 {
	if m == nil {
		return 0
	}

	var online int64
	m.sessionsByID.Range(func(_, value any) bool {
		s, ok := value.(Session)
		if !ok || s == nil {
			return true
		}
		if s.Lifecycle() != LifecycleActive {
			return true
		}
		if s.ProtocolState() != protocol.StatePlay {
			return true
		}
		if s.Player() == nil {
			return true
		}
		online++
		return true
	})

	return online
}

func (m *DefaultManager) removeByIdentifiers(id uint64, networkSessionID uint64) {
	if _, ok := m.sessionsByID.LoadAndDelete(id); !ok {
		return
	}
	m.sessionsByNetworkID.Delete(networkSessionID)
	m.count.Add(-1)
}
