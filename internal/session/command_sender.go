package session

import (
	"strings"

	"github.com/vitismc/vitis/internal/chat"
	"github.com/vitismc/vitis/internal/command"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// sessionCommandSender adapts a Session to the command.Sender interface.
type sessionCommandSender struct {
	session Session
	pm      *PlayerManager
	ops     *operator.List
}

// newSessionCommandSender creates a command.Sender from a session.
func newSessionCommandSender(s Session, pm *PlayerManager, ops *operator.List) command.Sender {
	return &sessionCommandSender{session: s, pm: pm, ops: ops}
}

func (s *sessionCommandSender) Name() string {
	if ds, ok := s.session.(*DefaultSession); ok {
		if raw, found := ds.SessionData().Load("login_name"); found {
			if name, ok := raw.(string); ok {
				return name
			}
		}
	}
	return "Player"
}

func (s *sessionCommandSender) SendMessage(text string) {
	if strings.Contains(text, "§") {
		comp := chat.FromLegacy(text)
		_ = s.session.Send(playpacket.NewSystemChatNBT(comp.EncodeNBT()))
	} else {
		_ = s.session.Send(playpacket.NewSystemChatText(text))
	}
}

func (s *sessionCommandSender) HasPermission(level int) bool {
	if s.ops == nil {
		return true
	}
	uuid := s.UUID()
	return s.ops.GetLevel(uuid) >= level
}

func (s *sessionCommandSender) IsPlayer() bool {
	return true
}

// Implement PlayerSender interface for full command integration.

func (s *sessionCommandSender) UUID() protocol.UUID {
	if ds, ok := s.session.(*DefaultSession); ok {
		if raw, found := ds.SessionData().Load("login_uuid"); found {
			if uuid, ok := raw.(protocol.UUID); ok {
				return uuid
			}
		}
	}
	return protocol.UUID{}
}

func (s *sessionCommandSender) EntityID() int32 {
	if p := s.session.Player(); p != nil {
		return p.ID()
	}
	return 0
}

func (s *sessionCommandSender) Position() (x, y, z float64) {
	if s.pm == nil {
		return 0, 0, 0
	}
	if op := s.pm.GetBySession(s.session); op != nil {
		return op.X, op.Y, op.Z
	}
	return 0, 0, 0
}

func (s *sessionCommandSender) GameMode() int32 {
	if s.pm == nil {
		return 1
	}
	if op := s.pm.GetBySession(s.session); op != nil {
		return op.GameMode
	}
	return 1
}

// Ensure sessionCommandSender implements command.PlayerSender.
var _ command.PlayerSender = (*sessionCommandSender)(nil)
