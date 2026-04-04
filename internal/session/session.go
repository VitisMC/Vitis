package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	defaultWorkerQueueCapacity = 2048
	defaultEncodeBufferSize    = 256
	defaultFrameBufferSize     = 256
	maxPooledBufferSize        = 256 * 1024
)

// Player is a game-layer player reference bound to a session after login.
type Player interface {
	ID() int32
}

// MovablePlayer extends Player with position/rotation mutation methods.
type MovablePlayer interface {
	Player
	SetPositionXYZ(x, y, z float64)
	SetRotationYP(yaw, pitch float32)
	SetOnGround(v bool)
}

// Connection is the network bridge used by sessions.
type Connection interface {
	Context() context.Context
	RemoteAddr() net.Addr
	Send(packet *network.Packet) error
	Close() error
	ForceClose(err error) error
	Done() <-chan struct{}
	EnableCompression(threshold int)
	EnableEncryption(encrypt, decrypt interface{})
}

// Session is the public session contract used by the manager and packet handlers.
type Session interface {
	ID() uint64
	NetworkSessionID() uint64
	Context() context.Context
	RemoteAddr() net.Addr
	Lifecycle() LifecycleState
	ProtocolState() protocol.State
	ProtocolVersion() int32
	Player() Player
	BindPlayer(player Player)
	CompressionEnabled() bool
	EncryptionEnabled() bool
	EnableCompression(enabled bool)
	EnableEncryption(enabled bool)
	HandleNetworkPacket(packet *network.Packet) error
	Send(packet protocol.Packet) error
	EnableNetworkCompression(threshold int)
	EnableNetworkEncryption(encrypt, decrypt interface{})
	Close() error
	ForceClose(err error) error
}

// Events allows optional callbacks for observability and instrumentation.
type Events struct {
	OnConnect       func(session Session)
	OnDisconnect    func(session Session, err error)
	OnPacketReceive func(session Session, packet protocol.Packet)
}

// Config defines construction options for DefaultSession.
type Config struct {
	ID               uint64
	NetworkSessionID uint64
	Connection       Connection
	Registry         *protocol.Registry
	Decoder          *protocol.Decoder
	Router           PacketRouter
	WorkerPool       *network.WorkerPool
	InitialVersion   int32
	InitialState     protocol.State
	Events           Events
}

type playerBinding struct {
	player Player
}

// DefaultSession is the production session implementation.
type DefaultSession struct {
	id               uint64
	networkSessionID uint64
	conn             Connection
	registry         *protocol.Registry
	decoder          *protocol.Decoder
	router           PacketRouter
	workerPool       *network.WorkerPool
	ownedWorkerPool  bool
	events           Events

	protocolState *protocol.SessionState
	lifecycle     atomic.Uint32

	compressionEnabled atomic.Bool
	encryptionEnabled  atomic.Bool

	player atomic.Pointer[playerBinding]

	sessionData sync.Map

	encodeBufferPool sync.Pool
	frameBufferPool  sync.Pool
	packetPool       sync.Pool

	closeOnce sync.Once
	closeErr  atomic.Value
	ctx       context.Context
}

// New creates a new session instance.
func New(cfg Config) (*DefaultSession, error) {
	if cfg.Connection == nil {
		return nil, ErrNilConnection
	}
	if cfg.Registry == nil {
		return nil, ErrNilRegistry
	}
	if !cfg.InitialState.Valid() {
		cfg.InitialState = protocol.StateHandshake
	}
	if cfg.InitialVersion == 0 {
		cfg.InitialVersion = protocol.AnyVersion
	}

	decoder := cfg.Decoder
	if decoder == nil {
		decoder = protocol.NewDecoder(cfg.Registry, 0)
	}

	workerPool := cfg.WorkerPool
	ownedPool := false
	if workerPool == nil {
		size := runtime.GOMAXPROCS(0)
		if size < 2 {
			size = 2
		}
		workerPool = network.NewWorkerPool(network.WorkerPoolConfig{
			Size:          size,
			QueueCapacity: defaultWorkerQueueCapacity,
		})
		ownedPool = true
	}

	s := &DefaultSession{
		id:               cfg.ID,
		networkSessionID: cfg.NetworkSessionID,
		conn:             cfg.Connection,
		registry:         cfg.Registry,
		decoder:          decoder,
		router:           cfg.Router,
		workerPool:       workerPool,
		ownedWorkerPool:  ownedPool,
		events:           cfg.Events,
		protocolState:    protocol.NewSessionState(cfg.InitialVersion, cfg.InitialState),
		ctx:              cfg.Connection.Context(),
	}
	if s.ctx == nil {
		s.ctx = context.Background()
	}
	s.lifecycle.Store(uint32(LifecycleActive))

	s.encodeBufferPool.New = func() any {
		return protocol.NewBuffer(defaultEncodeBufferSize)
	}
	s.frameBufferPool.New = func() any {
		return make([]byte, defaultFrameBufferSize)
	}
	s.packetPool.New = func() any {
		return &network.Packet{}
	}

	if s.events.OnConnect != nil {
		s.events.OnConnect(s)
	}

	go s.watchConnectionDone()
	return s, nil
}

// ID returns the stable process-local session id.
func (s *DefaultSession) ID() uint64 {
	return s.id
}

// NetworkSessionID returns the network session id associated with this session.
func (s *DefaultSession) NetworkSessionID() uint64 {
	return s.networkSessionID
}

// Context returns a lifecycle context bound to the underlying connection.
func (s *DefaultSession) Context() context.Context {
	return s.ctx
}

// RemoteAddr returns the connection remote address.
func (s *DefaultSession) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// Lifecycle returns the current lifecycle state.
func (s *DefaultSession) Lifecycle() LifecycleState {
	return LifecycleState(s.lifecycle.Load())
}

// ProtocolState returns the current protocol state.
func (s *DefaultSession) ProtocolState() protocol.State {
	return s.protocolState.State()
}

// ProtocolVersion returns the currently negotiated protocol version.
func (s *DefaultSession) ProtocolVersion() int32 {
	return s.protocolState.ProtocolVersion()
}

// Player returns currently bound player or nil.
func (s *DefaultSession) Player() Player {
	binding := s.player.Load()
	if binding == nil {
		return nil
	}
	return binding.player
}

// BindPlayer binds or unbinds player reference to this session.
func (s *DefaultSession) BindPlayer(player Player) {
	if player == nil {
		s.player.Store(nil)
		return
	}
	s.player.Store(&playerBinding{player: player})
}

// CompressionEnabled reports whether protocol compression is enabled for this session.
func (s *DefaultSession) CompressionEnabled() bool {
	return s.compressionEnabled.Load()
}

// EncryptionEnabled reports whether transport encryption is enabled for this session.
func (s *DefaultSession) EncryptionEnabled() bool {
	return s.encryptionEnabled.Load()
}

// EnableCompression toggles protocol compression flag.
func (s *DefaultSession) EnableCompression(enabled bool) {
	s.compressionEnabled.Store(enabled)
}

// EnableEncryption toggles encryption flag.
func (s *DefaultSession) EnableEncryption(enabled bool) {
	s.encryptionEnabled.Store(enabled)
}

// SessionData returns the session-scoped data map for storing arbitrary key-value pairs.
func (s *DefaultSession) SessionData() *sync.Map {
	return &s.sessionData
}

// EnableNetworkCompression enables protocol compression on the underlying connection.
func (s *DefaultSession) EnableNetworkCompression(threshold int) {
	s.conn.EnableCompression(threshold)
	s.compressionEnabled.Store(true)
}

// EnableNetworkEncryption enables AES-CFB8 encryption on the underlying connection.
func (s *DefaultSession) EnableNetworkEncryption(encrypt, decrypt interface{}) {
	s.conn.EnableEncryption(encrypt, decrypt)
	s.encryptionEnabled.Store(true)
}

// Send encodes and sends a protocol packet through the network layer asynchronously.
func (s *DefaultSession) Send(packet protocol.Packet) error {
	if packet == nil {
		return protocol.ErrNilPacket
	}
	if err := s.ensureActive(); err != nil {
		return err
	}

	currentState := s.protocolState.State()
	if _, ok := s.registry.Lookup(s.protocolState.ProtocolVersion(), currentState, protocol.DirectionOutbound, packet.ID()); !ok {
		return fmt.Errorf("send packet state=%s version=%d id=%d: %w", currentState.String(), s.protocolState.ProtocolVersion(), packet.ID(), protocol.ErrUnknownPacket)
	}

	encodeBuffer := s.acquireEncodeBuffer()
	defer s.releaseEncodeBuffer(encodeBuffer)
	encodeBuffer.Reset()

	if err := packet.Encode(encodeBuffer); err != nil {
		return fmt.Errorf("encode outbound packet id=%d: %w", packet.ID(), err)
	}

	networkPacket := s.acquireNetworkPacket()
	networkPacket.ID = packet.ID()
	networkPacket.Payload = encodeBuffer.Bytes()
	err := s.conn.Send(networkPacket)
	networkPacket.Payload = nil
	s.releaseNetworkPacket(networkPacket)
	if err != nil {
		return err
	}

	s.applyOutboundVersionTransition(packet)
	if err := s.applyOutboundStateTransition(currentState, packet); err != nil {
		_ = s.ForceClose(err)
		return err
	}
	return nil
}

// HandleNetworkPacket accepts a decoded network packet and dispatches protocol handling on worker pool.
func (s *DefaultSession) HandleNetworkPacket(packet *network.Packet) error {
	if packet == nil {
		return fmt.Errorf("handle packet: nil packet")
	}
	if err := s.ensureActive(); err != nil {
		return err
	}

	frameSize := protocol.VarIntSize(packet.ID) + len(packet.Payload)
	frame := s.acquireFrameBuffer(frameSize)
	offset := protocol.EncodeVarInt(frame, packet.ID)
	copy(frame[offset:], packet.Payload)

	err := s.workerPool.Submit(func() {
		s.processInboundFrame(frame[:frameSize])
	})
	if err != nil {
		s.releaseFrameBuffer(frame)
		if errors.Is(err, network.ErrWorkerPoolFull) {
			return fmt.Errorf("session worker pool saturated: %w", err)
		}
		if errors.Is(err, network.ErrWorkerPoolClosed) {
			return ErrSessionClosing
		}
		return err
	}

	return nil
}

// Close gracefully closes the session and underlying connection.
func (s *DefaultSession) Close() error {
	if !s.markClosing() {
		if s.Lifecycle() == LifecycleClosed {
			return ErrSessionClosed
		}
		return ErrSessionClosing
	}

	err := s.conn.Close()
	s.finalize(err)
	return err
}

// ForceClose force-closes the session and underlying connection.
func (s *DefaultSession) ForceClose(err error) error {
	if !s.markClosing() {
		if s.Lifecycle() == LifecycleClosed {
			return ErrSessionClosed
		}
		return ErrSessionClosing
	}

	closeErr := s.conn.ForceClose(err)
	s.finalize(closeErr)
	return closeErr
}

func (s *DefaultSession) processInboundFrame(frame []byte) {
	defer s.releaseFrameBuffer(frame)
	if s.Lifecycle() != LifecycleActive {
		return
	}

	previousState := s.protocolState.State()
	packet, err := s.decoder.Decode(s.protocolState, frame)
	if err != nil {
		logger.Warn("decode error", "session", s.id, "state", previousState.String(), "error", err)
		_ = s.ForceClose(err)
		return
	}

	if err := ValidateProtocolTransition(previousState, s.protocolState.State()); err != nil {
		logger.Warn("invalid protocol transition", "session", s.id, "from", previousState.String(), "to", s.protocolState.State().String())
		s.protocolState.SetState(previousState)
		_ = s.ForceClose(err)
		return
	}

	if s.events.OnPacketReceive != nil {
		s.events.OnPacketReceive(s, packet)
	}

	if s.router != nil {
		if err := s.router.Handle(s, previousState, packet); err != nil && !errors.Is(err, ErrPacketHandlerNotFound) {
			logger.Warn("handler error", "session", s.id, "state", previousState.String(), "packet_id", fmt.Sprintf("0x%02X", packet.ID()), "error", err)
			_ = s.ForceClose(err)
		}
	}
}

func (s *DefaultSession) ensureActive() error {
	switch s.Lifecycle() {
	case LifecycleActive:
		return nil
	case LifecycleClosing:
		return ErrSessionClosing
	default:
		return ErrSessionClosed
	}
}

func (s *DefaultSession) markClosing() bool {
	for {
		state := s.lifecycle.Load()
		switch LifecycleState(state) {
		case LifecycleClosed, LifecycleClosing:
			return false
		case LifecycleActive:
			if s.lifecycle.CompareAndSwap(uint32(LifecycleActive), uint32(LifecycleClosing)) {
				return true
			}
		default:
			return false
		}
	}
}

func (s *DefaultSession) finalize(err error) {
	s.closeOnce.Do(func() {
		if err != nil {
			s.closeErr.Store(err)
		}
		s.lifecycle.Store(uint32(LifecycleClosed))
		if s.ownedWorkerPool {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = s.workerPool.Stop(ctx)
			cancel()
		}
		var cause error
		stored := s.closeErr.Load()
		if stored != nil {
			cause, _ = stored.(error)
		}
		logger.Debug("session finalize", "session", s.id, "cause", cause)
		if s.events.OnDisconnect != nil {
			s.events.OnDisconnect(s, cause)
		}
	})
}

func (s *DefaultSession) watchConnectionDone() {
	<-s.conn.Done()
	s.markClosing()
	s.finalize(nil)
}

func (s *DefaultSession) applyOutboundVersionTransition(packet protocol.Packet) {
	transition, ok := packet.(protocol.ProtocolVersionTransition)
	if !ok {
		return
	}
	version, shouldTransition := transition.ProtocolVersionTransition()
	if shouldTransition {
		s.protocolState.SetProtocolVersion(version)
	}
}

func (s *DefaultSession) applyOutboundStateTransition(current protocol.State, packet protocol.Packet) error {
	transition, ok := packet.(protocol.OutboundStateTransition)
	if !ok {
		return nil
	}
	nextState, shouldTransition := transition.OutboundStateTransition()
	if !shouldTransition {
		return nil
	}
	if err := ValidateProtocolTransition(current, nextState); err != nil {
		return err
	}
	s.protocolState.SetState(nextState)
	return nil
}

func (s *DefaultSession) acquireEncodeBuffer() *protocol.Buffer {
	buffer := s.encodeBufferPool.Get().(*protocol.Buffer)
	buffer.Reset()
	return buffer
}

func (s *DefaultSession) releaseEncodeBuffer(buffer *protocol.Buffer) {
	if buffer == nil {
		return
	}
	if buffer.Cap() > maxPooledBufferSize {
		return
	}
	buffer.Reset()
	s.encodeBufferPool.Put(buffer)
}

func (s *DefaultSession) acquireFrameBuffer(size int) []byte {
	buffer := s.frameBufferPool.Get().([]byte)
	if cap(buffer) < size {
		return make([]byte, size)
	}
	return buffer[:size]
}

func (s *DefaultSession) releaseFrameBuffer(buffer []byte) {
	if cap(buffer) > maxPooledBufferSize {
		return
	}
	s.frameBufferPool.Put(buffer[:cap(buffer)])
}

func (s *DefaultSession) acquireNetworkPacket() *network.Packet {
	return s.packetPool.Get().(*network.Packet)
}

func (s *DefaultSession) releaseNetworkPacket(packet *network.Packet) {
	if packet == nil {
		return
	}
	packet.ID = 0
	packet.Payload = nil
	s.packetPool.Put(packet)
}
