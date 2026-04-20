package query

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/vitismc/vitis/internal/logger"
)

const (
	packetTypeHandshake byte = 0x09
	packetTypeStat      byte = 0x00

	challengeExpiry = 30 * time.Second
)

// ServerInfo provides dynamic server information to the query server.
type ServerInfo interface {
	MOTD() string
	GameType() string
	Map() string
	NumPlayers() int
	MaxPlayers() int
	HostPort() int
	HostIP() string
	OnlinePlayerNames() []string
	Plugins() string
	Version() string
}

// Config holds query server configuration.
type Config struct {
	Address string
	Info    ServerInfo
}

// Server implements the GS4 query protocol over UDP.
type Server struct {
	cfg        Config
	conn       *net.UDPConn
	mu         sync.Mutex
	closed     bool
	challenges sync.Map
}

type challengeEntry struct {
	token   int32
	created time.Time
}

// New creates a new query server.
func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Start begins listening for query requests.
func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.Address)
	if err != nil {
		return fmt.Errorf("query resolve: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("query listen: %w", err)
	}
	s.conn = conn
	logger.Info("query server started", "address", s.cfg.Address)

	go s.readLoop()
	go s.cleanupLoop()
	return nil
}

// Stop shuts down the query server.
func (s *Server) Stop() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	if s.conn != nil {
		_ = s.conn.Close()
	}
	logger.Info("query server stopped")
}

func (s *Server) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			logger.Debug("query read error", "error", err)
			continue
		}
		if n < 7 {
			continue
		}
		s.handlePacket(buf[:n], addr)
	}
}

func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return
		}
		now := time.Now()
		s.challenges.Range(func(key, value interface{}) bool {
			if ce, ok := value.(*challengeEntry); ok {
				if now.Sub(ce.created) > challengeExpiry {
					s.challenges.Delete(key)
				}
			}
			return true
		})
	}
}

func (s *Server) handlePacket(data []byte, addr *net.UDPAddr) {
	if data[0] != 0xFE || data[1] != 0xFD {
		return
	}

	pktType := data[2]
	sessionID := int32(binary.BigEndian.Uint32(data[3:7]))

	switch pktType {
	case packetTypeHandshake:
		s.handleHandshake(sessionID, addr)
	case packetTypeStat:
		if len(data) < 11 {
			return
		}
		challengeToken := int32(binary.BigEndian.Uint32(data[7:11]))
		if !s.validateChallenge(addr, challengeToken) {
			return
		}
		if len(data) > 11 {
			s.handleFullStat(sessionID, addr)
		} else {
			s.handleBasicStat(sessionID, addr)
		}
	}
}

func (s *Server) handleHandshake(sessionID int32, addr *net.UDPAddr) {
	token := rand.Int31() & 0x0F0F0F0F
	s.challenges.Store(addr.String(), &challengeEntry{
		token:   token,
		created: time.Now(),
	})

	tokenStr := strconv.Itoa(int(token))
	resp := make([]byte, 5+len(tokenStr)+1)
	resp[0] = packetTypeHandshake
	binary.BigEndian.PutUint32(resp[1:5], uint32(sessionID))
	copy(resp[5:], tokenStr)
	resp[len(resp)-1] = 0

	_, _ = s.conn.WriteToUDP(resp, addr)
}

func (s *Server) validateChallenge(addr *net.UDPAddr, token int32) bool {
	val, ok := s.challenges.Load(addr.String())
	if !ok {
		return false
	}
	ce, ok := val.(*challengeEntry)
	if !ok {
		return false
	}
	return ce.token == token
}

func (s *Server) handleBasicStat(sessionID int32, addr *net.UDPAddr) {
	info := s.cfg.Info
	if info == nil {
		return
	}

	var buf []byte
	buf = append(buf, packetTypeStat)
	buf = appendUint32BE(buf, uint32(sessionID))
	buf = appendNullString(buf, info.MOTD())
	buf = appendNullString(buf, info.GameType())
	buf = appendNullString(buf, info.Map())
	buf = appendNullString(buf, strconv.Itoa(info.NumPlayers()))
	buf = appendNullString(buf, strconv.Itoa(info.MaxPlayers()))

	portBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(portBytes, uint16(info.HostPort()))
	buf = append(buf, portBytes...)
	buf = appendNullString(buf, info.HostIP())

	_, _ = s.conn.WriteToUDP(buf, addr)
}

func (s *Server) handleFullStat(sessionID int32, addr *net.UDPAddr) {
	info := s.cfg.Info
	if info == nil {
		return
	}

	var buf []byte
	buf = append(buf, packetTypeStat)
	buf = appendUint32BE(buf, uint32(sessionID))

	buf = append(buf, "splitnum\x00\x80\x00"...)

	kvPairs := [][2]string{
		{"hostname", info.MOTD()},
		{"gametype", info.GameType()},
		{"game_id", "MINECRAFT"},
		{"version", info.Version()},
		{"plugins", info.Plugins()},
		{"map", info.Map()},
		{"numplayers", strconv.Itoa(info.NumPlayers())},
		{"maxplayers", strconv.Itoa(info.MaxPlayers())},
		{"hostport", strconv.Itoa(info.HostPort())},
		{"hostip", info.HostIP()},
	}

	for _, kv := range kvPairs {
		buf = appendNullString(buf, kv[0])
		buf = appendNullString(buf, kv[1])
	}
	buf = append(buf, 0x00)

	buf = append(buf, 0x01)
	buf = appendNullString(buf, "player_")
	buf = append(buf, 0x00)

	players := info.OnlinePlayerNames()
	for _, name := range players {
		buf = appendNullString(buf, name)
	}
	buf = append(buf, 0x00)

	_, _ = s.conn.WriteToUDP(buf, addr)
}

func appendNullString(buf []byte, s string) []byte {
	buf = append(buf, []byte(s)...)
	buf = append(buf, 0x00)
	return buf
}

func appendUint32BE(buf []byte, v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return append(buf, b...)
}

// StaticServerInfo is a simple implementation of ServerInfo for testing or static configuration.
type StaticServerInfo struct {
	MotdValue       string
	GameTypeValue   string
	MapValue        string
	NumPlayersValue int
	MaxPlayersValue int
	HostPortValue   int
	HostIPValue     string
	PlayersValue    []string
	PluginsValue    string
	VersionValue    string
}

func (i *StaticServerInfo) MOTD() string                { return i.MotdValue }
func (i *StaticServerInfo) GameType() string            { return i.GameTypeValue }
func (i *StaticServerInfo) Map() string                 { return i.MapValue }
func (i *StaticServerInfo) NumPlayers() int             { return i.NumPlayersValue }
func (i *StaticServerInfo) MaxPlayers() int             { return i.MaxPlayersValue }
func (i *StaticServerInfo) HostPort() int               { return i.HostPortValue }
func (i *StaticServerInfo) HostIP() string              { return i.HostIPValue }
func (i *StaticServerInfo) OnlinePlayerNames() []string { return i.PlayersValue }
func (i *StaticServerInfo) Plugins() string             { return i.PluginsValue }
func (i *StaticServerInfo) Version() string             { return i.VersionValue }

// DynamicServerInfo wraps live server data behind the ServerInfo interface.
type DynamicServerInfo struct {
	MotdValue    string
	VersionValue string
	MaxPlayersV  int
	Port         int
	IP           string
	PlayerList   func() []string
}

func (d *DynamicServerInfo) MOTD() string     { return d.MotdValue }
func (d *DynamicServerInfo) GameType() string { return "SMP" }
func (d *DynamicServerInfo) Map() string      { return "world" }
func (d *DynamicServerInfo) NumPlayers() int {
	if d.PlayerList != nil {
		return len(d.PlayerList())
	}
	return 0
}
func (d *DynamicServerInfo) MaxPlayers() int { return d.MaxPlayersV }
func (d *DynamicServerInfo) HostPort() int   { return d.Port }
func (d *DynamicServerInfo) HostIP() string  { return d.IP }
func (d *DynamicServerInfo) OnlinePlayerNames() []string {
	if d.PlayerList != nil {
		return d.PlayerList()
	}
	return nil
}
func (d *DynamicServerInfo) Plugins() string {
	return "Vitis on " + d.VersionValue
}
func (d *DynamicServerInfo) Version() string { return d.VersionValue }
