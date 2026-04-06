package session

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/chat"
	"github.com/vitismc/vitis/internal/command"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	enchdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
	gamerules "github.com/vitismc/vitis/internal/data/generated/game_rules"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/experience"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/weather"
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

// WeatherAccessor provides weather control for the command bridge.
type WeatherAccessor interface {
	SetWeather(state weather.State, duration int32)
}

// MobSpawner provides mob spawning for the command bridge.
type MobSpawner interface {
	SummonMob(typeName string, x, y, z float64)
}

// ServerControlAdapter adapts session-layer components to the command.ServerControl interface.
type ServerControlAdapter struct {
	PM                     *PlayerManager
	OpList                 *operator.List
	StopFunc               func()
	SeedValue              int64
	World                  WorldTimeAccessor
	WeatherWorld           WeatherAccessor
	WorldAccess            WorldAccessor
	MobSpawn               MobSpawner
	GameRules              map[string]string
	DefaultGM              int32
	SpawnX, SpawnY, SpawnZ int
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

func (s *ServerControlAdapter) SetWeather(w string, duration int) {
	if s.WeatherWorld == nil {
		return
	}
	state := weather.ParseState(w)
	s.WeatherWorld.SetWeather(state, int32(duration))
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

func (s *ServerControlAdapter) EnchantItem(entityID int32, enchantName string, level int) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	info := enchdata.EnchantmentByID(enchdata.EnchantmentIDByName(enchantName))
	if info == nil {
		return fmt.Errorf("unknown enchantment: %s", enchantName)
	}

	op := s.PM.GetByEntityID(entityID)
	if op == nil || op.Windows == nil {
		return fmt.Errorf("player not found")
	}
	heldIdx := inventory.HotbarStart + int(op.Windows.HeldSlot())
	held := op.Windows.Inventory().Get(heldIdx)
	if held.Empty() {
		return fmt.Errorf("player is not holding an item")
	}
	held.SetEnchant(info.ID, int32(level))
	op.Windows.Inventory().Set(heldIdx, held)
	return nil
}

func (s *ServerControlAdapter) SetBlockAt(x, y, z int, blockName string) (int32, error) {
	if s.WorldAccess == nil {
		return 0, fmt.Errorf("no world access")
	}
	stateID := block.DefaultStateID(blockName)
	if stateID < 0 {
		return 0, fmt.Errorf("unknown block: %s", blockName)
	}
	s.WorldAccess.SetBlock(x, y, z, stateID)
	s.WorldAccess.NotifyNeighbors(x, y, z)
	if s.PM != nil {
		s.PM.Broadcast(&playpacket.BlockUpdate{
			Position: playpacket.BlockPos{X: int32(x), Y: int32(y), Z: int32(z)},
			BlockID:  stateID,
		})
	}
	return stateID, nil
}

func (s *ServerControlAdapter) FillBlocks(x1, y1, z1, x2, y2, z2 int, blockName string) (int, error) {
	if s.WorldAccess == nil {
		return 0, fmt.Errorf("no world access")
	}
	stateID := block.DefaultStateID(blockName)
	if stateID < 0 {
		return 0, fmt.Errorf("unknown block: %s", blockName)
	}
	minX, maxX := x1, x2
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := y1, y2
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	minZ, maxZ := z1, z2
	if minZ > maxZ {
		minZ, maxZ = maxZ, minZ
	}
	volume := (maxX - minX + 1) * (maxY - minY + 1) * (maxZ - minZ + 1)
	if volume > 32768 {
		return 0, fmt.Errorf("fill volume %d exceeds limit of 32768", volume)
	}
	count := 0
	for bx := minX; bx <= maxX; bx++ {
		for by := minY; by <= maxY; by++ {
			for bz := minZ; bz <= maxZ; bz++ {
				old := s.WorldAccess.GetBlock(bx, by, bz)
				if old != stateID {
					s.WorldAccess.SetBlock(bx, by, bz, stateID)
					count++
				}
			}
		}
	}
	if s.PM != nil {
		for bx := minX; bx <= maxX; bx++ {
			for by := minY; by <= maxY; by++ {
				for bz := minZ; bz <= maxZ; bz++ {
					s.PM.Broadcast(&playpacket.BlockUpdate{
						Position: playpacket.BlockPos{X: int32(bx), Y: int32(by), Z: int32(bz)},
						BlockID:  stateID,
					})
				}
			}
		}
	}
	return count, nil
}

func (s *ServerControlAdapter) ClearInventory(entityID int32, itemName string, maxCount int) (int, error) {
	if s.PM == nil {
		return 0, fmt.Errorf("no player manager")
	}
	op := s.PM.GetByEntityID(entityID)
	if op == nil || op.Windows == nil {
		return 0, fmt.Errorf("player not found")
	}
	var filterID int32 = -1
	if itemName != "" {
		filterID = item.IDByName(itemName)
		if filterID <= 0 {
			return 0, fmt.Errorf("unknown item: %s", itemName)
		}
	}
	inv := op.Windows.Inventory()
	removed := 0
	remaining := maxCount
	if maxCount < 0 {
		remaining = int(^uint(0) >> 1)
	}
	for i := 0; i < inv.Size(); i++ {
		if remaining <= 0 {
			break
		}
		sl := inv.Get(i)
		if sl.Empty() {
			continue
		}
		if filterID > 0 && sl.ItemID != filterID {
			continue
		}
		take := int(sl.ItemCount)
		if take > remaining {
			take = remaining
		}
		sl.ItemCount -= int32(take)
		if sl.ItemCount <= 0 {
			inv.Set(i, inventory.EmptySlot())
		} else {
			inv.Set(i, sl)
		}
		removed += take
		remaining -= take
	}
	return removed, nil
}

func (s *ServerControlAdapter) GetGameRule(name string) (string, error) {
	info := gamerules.GameRuleByName(name)
	if info == nil {
		return "", fmt.Errorf("unknown game rule: %s", name)
	}
	if s.GameRules != nil {
		if v, ok := s.GameRules[name]; ok {
			return v, nil
		}
	}
	if info.Type == "boolean" {
		return "true", nil
	}
	return "0", nil
}

func (s *ServerControlAdapter) SetGameRule(name, value string) error {
	info := gamerules.GameRuleByName(name)
	if info == nil {
		return fmt.Errorf("unknown game rule: %s", name)
	}
	if s.GameRules == nil {
		s.GameRules = make(map[string]string)
	}
	s.GameRules[name] = value
	return nil
}

func (s *ServerControlAdapter) SetDefaultGameMode(mode int32) error {
	if mode < 0 || mode > 3 {
		return fmt.Errorf("invalid game mode: %d", mode)
	}
	s.DefaultGM = mode
	return nil
}

func (s *ServerControlAdapter) SetWorldSpawn(x, y, z int) error {
	s.SpawnX, s.SpawnY, s.SpawnZ = x, y, z
	if s.PM != nil {
		s.PM.Broadcast(&playpacket.SetDefaultSpawnPosition{
			X: int32(x), Y: int32(y), Z: int32(z),
		})
	}
	return nil
}

func (s *ServerControlAdapter) SetSpawnPoint(entityID int32, x, y, z int) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	op := s.PM.GetByEntityID(entityID)
	if op == nil {
		return fmt.Errorf("player not found")
	}
	_ = op.Session.Send(&playpacket.SetDefaultSpawnPosition{
		X: int32(x), Y: int32(y), Z: int32(z),
	})
	return nil
}

func (s *ServerControlAdapter) SendTitle(entityID int32, title, subtitle string, fadeIn, stay, fadeOut int) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	op := s.PM.GetByEntityID(entityID)
	if op == nil {
		return fmt.Errorf("player not found")
	}
	sess := op.Session
	if fadeIn > 0 || stay > 0 || fadeOut > 0 {
		_ = sess.Send(&playpacket.SetTitleTime{
			FadeIn: int32(fadeIn), Stay: int32(stay), FadeOut: int32(fadeOut),
		})
	}
	if subtitle != "" {
		_ = sess.Send(&playpacket.SetTitleSubtitle{Text: subtitle})
	}
	if title != "" {
		_ = sess.Send(&playpacket.SetTitleText{Text: title})
	}
	return nil
}

func (s *ServerControlAdapter) SendActionBar(entityID int32, text string) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	op := s.PM.GetByEntityID(entityID)
	if op == nil {
		return fmt.Errorf("player not found")
	}
	return op.Session.Send(&playpacket.ActionBar{Text: text})
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

func (s *ServerControlAdapter) SummonMob(entityType string, x, y, z float64) error {
	if s.MobSpawn == nil {
		return fmt.Errorf("mob spawning not available")
	}
	s.MobSpawn.SummonMob(entityType, x, y, z)
	return nil
}

func (s *ServerControlAdapter) ApplyEffect(entityID int32, effectName string, durationTicks int32, amplifier int32) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	info := effectdata.EffectByName(effectName)
	if info == nil {
		return fmt.Errorf("unknown effect: %s", effectName)
	}

	op := s.PM.GetByEntityID(entityID)
	if op == nil {
		return fmt.Errorf("player not found")
	}
	player, ok := op.Session.Player().(*entity.Player)
	if !ok || player == nil {
		return fmt.Errorf("player not found")
	}

	inst := effect.Instance{
		ID:        info.ID,
		Amplifier: amplifier,
		Duration:  durationTicks,
		Flags:     effect.FlagParticles | effect.FlagIcon,
	}
	player.Living().Effects().Add(inst)

	_ = op.Session.Send(&playpacket.EntityEffect{
		EntityID:  entityID,
		EffectID:  info.ID,
		Amplifier: amplifier,
		Duration:  durationTicks,
		Flags:     inst.Flags,
	})
	return nil
}

func (s *ServerControlAdapter) ClearEffects(entityID int32, effectName string) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}

	op := s.PM.GetByEntityID(entityID)
	if op == nil {
		return fmt.Errorf("player not found")
	}
	player, ok := op.Session.Player().(*entity.Player)
	if !ok || player == nil {
		return fmt.Errorf("player not found")
	}

	mgr := player.Living().Effects()
	if effectName == "" {
		ids := mgr.Clear()
		for _, id := range ids {
			_ = op.Session.Send(&playpacket.RemoveEntityEffect{
				EntityID: entityID,
				EffectID: id,
			})
		}
	} else {
		info := effectdata.EffectByName(effectName)
		if info == nil {
			return fmt.Errorf("unknown effect: %s", effectName)
		}
		if mgr.Remove(info.ID) {
			_ = op.Session.Send(&playpacket.RemoveEntityEffect{
				EntityID: entityID,
				EffectID: info.ID,
			})
		}
	}
	return nil
}

// Verify interface compliance.
var _ command.ServerControl = (*ServerControlAdapter)(nil)
var _ command.PlayerLookup = (*playerLookupAdapter)(nil)
var _ protocol.Packet = (*playpacket.GameEvent)(nil) // suppress unused import
