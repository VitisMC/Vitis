package rcon

import (
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/vitismc/vitis/internal/logger"
)

const (
	packetTypeCommand  int32 = 2
	packetTypeAuth     int32 = 3
	packetTypeResponse int32 = 0
	packetTypeAuthResp int32 = 2

	maxPayloadSize = 4096
	authFailID     = -1
)

// CommandExecutor processes an RCON command string and returns the response text.
type CommandExecutor func(command string) string

// Config holds RCON server configuration.
type Config struct {
	Address  string
	Password string
	Executor CommandExecutor
}

// Server is a Minecraft RCON server.
type Server struct {
	cfg         Config
	listener    net.Listener
	mu          sync.Mutex
	closed      bool
	wg          sync.WaitGroup
	connections sync.Map
}

// New creates a new RCON server.
func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Start begins listening for RCON connections.
func (s *Server) Start() error {
	if s == nil || !ValidPassword(s.cfg.Password) {
		return fmt.Errorf("rcon: password is too weak")
	}
	ln, err := net.Listen("tcp", s.cfg.Address)
	if err != nil {
		return fmt.Errorf("rcon listen: %w", err)
	}
	s.listener = ln
	logger.Info("rcon server started", "address", s.cfg.Address)

	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

// Stop shuts down the RCON server.
func (s *Server) Stop() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.connections.Range(func(key, _ any) bool {
		if conn, ok := key.(net.Conn); ok {
			_ = conn.Close()
		}
		return true
	})
	s.wg.Wait()
	logger.Info("rcon server stopped")
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			logger.Error("rcon accept error", "error", err)
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	s.connections.Store(conn, struct{}{})
	defer s.connections.Delete(conn)
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return
	}

	addr := conn.RemoteAddr().String()
	logger.Debug("rcon connection", "addr", addr)

	authenticated := false

	for {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		reqID, pktType, payload, err := readPacket(conn)
		if err != nil {
			if err != io.EOF {
				s.mu.Lock()
				closed := s.closed
				s.mu.Unlock()
				if !closed {
					logger.Debug("rcon read error", "addr", addr, "error", err)
				}
			}
			return
		}

		switch pktType {
		case packetTypeAuth:
			if subtle.ConstantTimeCompare([]byte(payload), []byte(s.cfg.Password)) == 1 {
				authenticated = true
				_ = writePacket(conn, reqID, packetTypeAuthResp, "")
				logger.Info("rcon authenticated", "addr", addr)
			} else {
				_ = writePacket(conn, authFailID, packetTypeAuthResp, "")
				logger.Warn("rcon auth failed", "addr", addr)
				return
			}

		case packetTypeCommand:
			if !authenticated {
				_ = writePacket(conn, authFailID, packetTypeResponse, "")
				return
			}

			cmd := strings.TrimSpace(payload)
			if cmd == "" {
				_ = writePacket(conn, reqID, packetTypeResponse, "")
				continue
			}

			var response string
			if s.cfg.Executor != nil {
				response = s.cfg.Executor(cmd)
			}
			_ = writePacket(conn, reqID, packetTypeResponse, response)

		default:
			logger.Debug("rcon unknown packet type", "type", pktType, "addr", addr)
		}
	}
}

func readPacket(r io.Reader) (requestID, pktType int32, payload string, err error) {
	var length int32
	if err = binary.Read(r, binary.LittleEndian, &length); err != nil {
		return
	}
	if length < 10 || length > maxPayloadSize+10 {
		err = fmt.Errorf("rcon packet length out of range: %d", length)
		return
	}

	buf := make([]byte, length)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}

	requestID = int32(binary.LittleEndian.Uint32(buf[0:4]))
	pktType = int32(binary.LittleEndian.Uint32(buf[4:8]))

	payloadEnd := len(buf) - 2
	if payloadEnd < 8 {
		payloadEnd = 8
	}
	payload = string(buf[8:payloadEnd])
	return
}

func writePacket(w io.Writer, requestID, pktType int32, payload string) error {
	payloadBytes := []byte(payload)
	length := int32(4 + 4 + len(payloadBytes) + 2)

	buf := make([]byte, 4+length)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(length))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(requestID))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(pktType))
	copy(buf[12:], payloadBytes)
	buf[len(buf)-2] = 0
	buf[len(buf)-1] = 0

	_, err := w.Write(buf)
	return err
}
