package session

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	statuspacket "github.com/vitismc/vitis/internal/protocol/packets/status"
)

var (
	ErrPacketHandlerNotFound  = fmt.Errorf("packet handler not found")
	ErrPacketHandlerDuplicate = fmt.Errorf("packet handler already registered")
)

const (
	defaultStatusVersionName       = "1.21.4"
	defaultStatusProtocol    int32 = 769
	defaultStatusDescription       = "Vitis Server"
	defaultStatusMaxPlayers        = 200
)

// SessionCounter exposes session count for status online-player reporting.
type SessionCounter interface {
	Count() int64
}

// StatusInfo contains server metadata used for status response generation.
type StatusInfo struct {
	VersionName     string
	ProtocolVersion int32
	MaxPlayers      int
	OnlinePlayers   int
	Description     string
	Favicon         string
	Sample          []statuspacket.ResponsePlayerSample
}

// StatusInfoProvider provides status metadata for one status request.
type StatusInfoProvider interface {
	StatusInfo(session Session) StatusInfo
}

// StatusInfoProviderFunc adapts a function to StatusInfoProvider.
type StatusInfoProviderFunc func(session Session) StatusInfo

// StatusInfo returns dynamically built status metadata for a session.
func (f StatusInfoProviderFunc) StatusInfo(session Session) StatusInfo {
	if f == nil {
		return StatusInfo{}
	}
	return f(session)
}

// DefaultStatusInfoProvider is a production status metadata provider.
type DefaultStatusInfoProvider struct {
	VersionName     string
	ProtocolVersion int32
	MaxPlayers      int
	Description     string
	Favicon         string
	SessionCounter  SessionCounter
}

// StatusInfo returns a normalized status metadata snapshot.
func (p *DefaultStatusInfoProvider) StatusInfo(session Session) StatusInfo {
	versionName := p.VersionName
	if versionName == "" {
		versionName = defaultStatusVersionName
	}

	protocolVersion := p.ProtocolVersion
	if session != nil && session.ProtocolVersion() > 0 {
		protocolVersion = session.ProtocolVersion()
	}
	if protocolVersion <= 0 {
		protocolVersion = defaultStatusProtocol
	}

	maxPlayers := p.MaxPlayers
	if maxPlayers <= 0 {
		maxPlayers = defaultStatusMaxPlayers
	}

	onlinePlayers := 0
	if p.SessionCounter != nil {
		count := p.SessionCounter.Count()
		if count > 0 {
			maxInt := int64(int(^uint(0) >> 1))
			if count > maxInt {
				onlinePlayers = int(maxInt)
			} else {
				onlinePlayers = int(count)
			}
		}
	}

	description := p.Description
	if description == "" {
		description = defaultStatusDescription
	}

	return StatusInfo{
		VersionName:     versionName,
		ProtocolVersion: protocolVersion,
		MaxPlayers:      maxPlayers,
		OnlinePlayers:   onlinePlayers,
		Description:     description,
		Favicon:         p.Favicon,
	}
}

// PacketHandler processes a decoded protocol packet for a specific state.
type PacketHandler func(session Session, packet protocol.Packet) error

// PacketRouter routes decoded packets by protocol state and packet id.
type PacketRouter interface {
	Register(state protocol.State, packetID int32, handler PacketHandler) error
	Handle(session Session, state protocol.State, packet protocol.Packet) error
}

type handlerKey struct {
	state protocol.State
	id    int32
}

// DefaultPacketRouter provides lock-free reads and copy-on-write registrations.
type DefaultPacketRouter struct {
	mu       sync.Mutex
	handlers atomic.Value
}

// NewPacketRouter creates a packet router.
func NewPacketRouter() *DefaultPacketRouter {
	r := &DefaultPacketRouter{}
	r.handlers.Store(map[handlerKey]PacketHandler{})
	return r
}

// Register binds packet handler for protocol state and packet id.
func (r *DefaultPacketRouter) Register(state protocol.State, packetID int32, handler PacketHandler) error {
	if r == nil {
		return fmt.Errorf("register packet handler: nil router")
	}
	if !state.Valid() {
		return fmt.Errorf("register packet handler id=%d: %w", packetID, protocol.ErrInvalidProtocolState)
	}
	if packetID < 0 {
		return fmt.Errorf("register packet handler: invalid packet id %d", packetID)
	}
	if handler == nil {
		return fmt.Errorf("register packet handler id=%d: nil handler", packetID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.handlers.Load().(map[handlerKey]PacketHandler)
	next := make(map[handlerKey]PacketHandler, len(current)+1)
	for key, value := range current {
		next[key] = value
	}

	key := handlerKey{state: state, id: packetID}
	if _, exists := next[key]; exists {
		return fmt.Errorf("register packet handler state=%s id=%d: %w", state.String(), packetID, ErrPacketHandlerDuplicate)
	}

	next[key] = handler
	r.handlers.Store(next)
	return nil
}

// Handle dispatches a decoded packet to the registered handler.
func (r *DefaultPacketRouter) Handle(session Session, state protocol.State, packet protocol.Packet) error {
	if r == nil {
		return nil
	}
	if session == nil {
		return fmt.Errorf("handle packet: nil session")
	}
	if packet == nil {
		return protocol.ErrNilPacket
	}

	handlers := r.handlers.Load().(map[handlerKey]PacketHandler)
	handler, ok := handlers[handlerKey{state: state, id: packet.ID()}]
	if !ok {
		return fmt.Errorf("handle packet state=%s id=%d: %w", state.String(), packet.ID(), ErrPacketHandlerNotFound)
	}
	return handler(session, packet)
}

// RegisterStatusHandlers registers status request and ping handlers on a packet router.
func RegisterStatusHandlers(router PacketRouter, provider StatusInfoProvider) error {
	if router == nil {
		return fmt.Errorf("register status handlers: nil router")
	}
	if provider == nil {
		provider = &DefaultStatusInfoProvider{}
	}

	statusRequestID := statuspacket.NewStatusRequest().ID()
	if err := router.Register(protocol.StateStatus, statusRequestID, func(session Session, packet protocol.Packet) error {
		_, ok := packet.(*statuspacket.StatusRequest)
		if !ok {
			return fmt.Errorf("handle status request: unexpected packet type %T", packet)
		}

		info := provider.StatusInfo(session)
		response := &statuspacket.StatusResponse{
			JSONResponse: statuspacket.BuildResponseJSON(statuspacket.ResponsePayload{
				Version: statuspacket.ResponseVersion{
					Name:     info.VersionName,
					Protocol: info.ProtocolVersion,
				},
				Players: statuspacket.ResponsePlayers{
					Max:    info.MaxPlayers,
					Online: info.OnlinePlayers,
					Sample: info.Sample,
				},
				Description: statuspacket.ResponseDescription{Text: info.Description},
				Favicon:     info.Favicon,
			}),
		}
		return session.Send(response)
	}); err != nil {
		return err
	}

	pingRequestID := statuspacket.NewPingRequest().ID()
	if err := router.Register(protocol.StateStatus, pingRequestID, func(session Session, packet protocol.Packet) error {
		ping, ok := packet.(*statuspacket.PingRequest)
		if !ok {
			return fmt.Errorf("handle ping request: unexpected packet type %T", packet)
		}
		return session.Send(&statuspacket.PingResponse{Payload: ping.Payload})
	}); err != nil {
		return err
	}

	return nil
}

// InboundHandler adapts network inbound packets to session packet processing.
type InboundHandler struct {
	manager Manager
}

// NewInboundHandler creates a network pipeline handler for sessions.
func NewInboundHandler(manager Manager) *InboundHandler {
	return &InboundHandler{manager: manager}
}

// HandleInbound routes packets from network session to application session asynchronously.
func (h *InboundHandler) HandleInbound(netSession network.Session, packet *network.Packet) error {
	if h == nil || h.manager == nil {
		return fmt.Errorf("handle inbound: nil session manager")
	}
	if netSession == nil {
		return fmt.Errorf("handle inbound: nil network session")
	}

	s, ok := h.manager.GetByNetworkSessionID(netSession.ID())
	if !ok {
		return fmt.Errorf("network session id=%d: %w", netSession.ID(), ErrSessionNotFound)
	}

	return s.HandleNetworkPacket(packet)
}

// HandleOutbound is a no-op because outbound packets are sent through Session.Send.
func (h *InboundHandler) HandleOutbound(_ network.Session, _ *network.Packet) error {
	return nil
}
