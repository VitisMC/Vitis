package session

import (
	"strings"

	"github.com/vitismc/vitis/internal/chat"
	"github.com/vitismc/vitis/internal/command"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/experience"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// playerLookupAdapter adapts PlayerManager to command.PlayerLookup.
type playerLookupAdapter struct {
	pm  *PlayerManager
	ops *operator.List
}

// NewPlayerLookup creates a command.PlayerLookup backed by a PlayerManager.
func NewPlayerLookup(pm *PlayerManager, ops *operator.List) command.PlayerLookup {
	return &playerLookupAdapter{pm: pm, ops: ops}
}

func (a *playerLookupAdapter) FindPlayerByName(name string) command.PlayerSender {
	if a.pm == nil {
		return nil
	}

	a.pm.mu.RLock()
	defer a.pm.mu.RUnlock()

	for _, p := range a.pm.players {
		if p.Name == name {
			return &sessionCommandSender{session: p.Session, pm: a.pm, ops: a.ops}
		}
	}
	return nil
}

func (a *playerLookupAdapter) OnlinePlayers() []string {
	if a.pm == nil {
		return nil
	}

	a.pm.mu.RLock()
	defer a.pm.mu.RUnlock()

	names := make([]string, 0, len(a.pm.players))
	for _, p := range a.pm.players {
		names = append(names, p.Name)
	}
	return names
}

// WorldTimeAccessor provides world time access for the command bridge.
type WorldTimeAccessor interface {
	TimeOfDay() int64
	SetTimeOfDay(t int64)
	WorldAge() int64
}

// ServerControlAdapter adapts session-layer components to the command.ServerControl interface.
type ServerControlAdapter struct {
	PM        *PlayerManager
	OpList    *operator.List
	StopFunc  func()
	SeedValue int64
	World     WorldTimeAccessor
}

// NewServerControl creates a command.ServerControl backed by session-layer components.
func NewServerControl(pm *PlayerManager, stopFunc func(), opList *operator.List) command.ServerControl {
	return &ServerControlAdapter{
		PM:       pm,
		OpList:   opList,
		StopFunc: stopFunc,
	}
}

func (s *ServerControlAdapter) Stop() {
	if s.StopFunc != nil {
		s.StopFunc()
	}
}

func (s *ServerControlAdapter) Seed() int64 {
	return s.SeedValue
}

func (s *ServerControlAdapter) SetTime(time int64) {
	if s.World != nil {
		s.World.SetTimeOfDay(time)
	}
	if s.PM != nil {
		age := int64(0)
		tod := time
		if s.World != nil {
			age = s.World.WorldAge()
			tod = s.World.TimeOfDay()
		}
		s.PM.Broadcast(&playpacket.UpdateTime{WorldAge: age, TimeOfDay: tod, TickDayTime: true})
	}
}

func (s *ServerControlAdapter) GetTime() int64 {
	if s.World != nil {
		return s.World.TimeOfDay()
	}
	return 0
}

func (s *ServerControlAdapter) SetWeather(_ string, _ int) {
	// TODO: broadcast weather change via GameEvent packet
}

func (s *ServerControlAdapter) SetGameMode(entityID int32, mode int32) error {
	if s.PM == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()

	for _, p := range s.PM.players {
		if p.EntityID == entityID {
			p.GameMode = mode
			_ = p.Session.Send(&playpacket.GameEvent{
				Event: 3,
				Value: float32(mode),
			})
			if player, ok := p.Session.Player().(*entity.Player); ok && player != nil {
				player.Living().SetGameMode(mode)
			}
			switch mode {
			case 1:
				_ = p.Session.Send(playpacket.CreativeAbilities())
			default:
				_ = p.Session.Send(playpacket.SurvivalAbilities())
			}
			return nil
		}
	}
	return nil
}

func (s *ServerControlAdapter) TeleportPlayer(entityID int32, x, y, z float64) error {
	if s.PM == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()

	for _, p := range s.PM.players {
		if p.EntityID == entityID {
			p.X, p.Y, p.Z = x, y, z
			teleportID := nextTeleportID()
			_ = p.Session.Send(&playpacket.SyncPlayerPosition{
				TeleportID: teleportID,
				X:          x,
				Y:          y,
				Z:          z,
			})
			return nil
		}
	}
	return nil
}

func (s *ServerControlAdapter) GiveItem(_ int32, _ string, _ int) error {
	// TODO: implement when inventory system is available
	return nil
}

func (s *ServerControlAdapter) KillEntity(entityID int32) error {
	if s.PM == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()

	for _, p := range s.PM.players {
		if p.EntityID == entityID {
			if player, ok := p.Session.Player().(*entity.Player); ok && player != nil {
				player.Living().Damage(1000, "kill_command")
			}
			_ = p.Session.Send(&playpacket.UpdateHealth{
				Health:         0,
				Food:           20,
				FoodSaturation: 5.0,
			})
			_ = p.Session.Send(&playpacket.DeathCombatEvent{
				PlayerID: entityID,
				Message:  p.Name + " was killed",
			})
			return nil
		}
	}
	return nil
}

func (s *ServerControlAdapter) SetDifficulty(_ int) error {
	// TODO: broadcast difficulty change packet
	return nil
}

func (s *ServerControlAdapter) SetOp(name string, level int) error {
	if s.PM == nil || s.OpList == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()
	for _, p := range s.PM.players {
		if p.Name == name {
			return s.OpList.Add(operator.Operator{
				UUID:  p.UUID,
				Name:  p.Name,
				Level: level,
			})
		}
	}
	return nil
}

func (s *ServerControlAdapter) RemoveOp(name string) error {
	if s.PM == nil || s.OpList == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()
	for _, p := range s.PM.players {
		if p.Name == name {
			return s.OpList.Remove(p.UUID)
		}
	}
	return nil
}

func (s *ServerControlAdapter) KickPlayer(name string, reason string) error {
	if s.PM == nil {
		return nil
	}
	s.PM.mu.RLock()
	var target *OnlinePlayer
	for _, p := range s.PM.players {
		if p.Name == name {
			target = p
			break
		}
	}
	s.PM.mu.RUnlock()

	if target == nil {
		return nil
	}

	_ = target.Session.Send(&playpacket.Disconnect{Text: reason})
	_ = target.Session.ForceClose(nil)
	return nil
}

func (s *ServerControlAdapter) BroadcastMessage(message string) {
	if s.PM == nil {
		return
	}
	if strings.Contains(message, "§") {
		comp := chat.FromLegacy(message)
		s.PM.Broadcast(playpacket.NewSystemChatNBT(comp.EncodeNBT()))
	} else {
		s.PM.Broadcast(playpacket.NewSystemChatText(message))
	}
}

func (s *ServerControlAdapter) AddXP(entityID int32, amount int32) error {
	if s.PM == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()

	for _, p := range s.PM.players {
		if p.EntityID == entityID {
			player, ok := p.Session.Player().(*entity.Player)
			if !ok || player == nil {
				return nil
			}
			living := player.Living()
			result := experience.AddXP(living.XPLevel(), living.XPTotal(), amount)
			living.SetXP(result.Level, result.Total, result.Bar)
			_ = p.Session.Send(&playpacket.SetExperience{
				ExperienceBar:   result.Bar,
				ExperienceLevel: result.Level,
				TotalExperience: result.Total,
			})
			return nil
		}
	}
	return nil
}

func (s *ServerControlAdapter) SetXPLevel(entityID int32, level int32) error {
	if s.PM == nil {
		return nil
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()

	for _, p := range s.PM.players {
		if p.EntityID == entityID {
			player, ok := p.Session.Player().(*entity.Player)
			if !ok || player == nil {
				return nil
			}
			living := player.Living()
			result := experience.SetLevel(level)
			living.SetXP(result.Level, result.Total, result.Bar)
			_ = p.Session.Send(&playpacket.SetExperience{
				ExperienceBar:   result.Bar,
				ExperienceLevel: result.Level,
				TotalExperience: result.Total,
			})
			return nil
		}
	}
	return nil
}

// Verify interface compliance.
var _ command.ServerControl = (*ServerControlAdapter)(nil)
var _ command.PlayerLookup = (*playerLookupAdapter)(nil)
var _ protocol.Packet = (*playpacket.GameEvent)(nil) // suppress unused import
