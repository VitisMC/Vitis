package network

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
)

var sessionIDCounter atomic.Uint64

// Session is the minimal network-facing abstraction used by packet handlers.
type Session interface {
	ID() uint64
	Context() context.Context
	RemoteAddr() net.Addr
	Send(packet *Packet) error
	Close() error
	Value(key any) any
	SetValue(key any, value any)
}

type connSession struct {
	id    uint64
	conn  *Conn
	ctx   context.Context
	attrs sync.Map
}

func newConnSession(conn *Conn, ctx context.Context) *connSession {
	return &connSession{
		id:   sessionIDCounter.Add(1),
		conn: conn,
		ctx:  ctx,
	}
}

// ID returns a stable process-local session identifier.
func (s *connSession) ID() uint64 {
	return s.id
}

// Context returns a cancellation context tied to the connection lifecycle.
func (s *connSession) Context() context.Context {
	return s.ctx
}

// RemoteAddr returns the peer network address.
func (s *connSession) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// Send queues an outbound packet for this session.
func (s *connSession) Send(packet *Packet) error {
	return s.conn.Send(packet)
}

// Close closes the underlying connection.
func (s *connSession) Close() error {
	return s.conn.Close()
}

// Value fetches a session-scoped attribute.
func (s *connSession) Value(key any) any {
	value, _ := s.attrs.Load(key)
	return value
}

// SetValue stores a session-scoped attribute.
func (s *connSession) SetValue(key any, value any) {
	s.attrs.Store(key, value)
}
