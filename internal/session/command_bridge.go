package session

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/chat"
	"github.com/vitismc/vitis/internal/command"
	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	enchdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	gamerules "github.com/vitismc/vitis/internal/data/generated/game_rules"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/experience"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world"
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

// DifficultyAccessor provides authoritative world difficulty control.
type DifficultyAccessor interface {
	Difficulty() int32
	SetDifficulty(difficulty int32)
}

type worldGameRules interface {
	SetDaylightCycle(bool)
	SetWeatherCycle(bool)
	SetFireTick(bool)
}

// MobSpawner provides mob spawning for the command bridge.
type MobSpawner interface {
	SummonMob(typeName string, x, y, z float64)
}

type worldCommandSubmitter interface {
	Submit(task func()) bool
}

const (
	defaultWalkingBlocksPerSecond = 4.317
	defaultFlyingBlocksPerSecond  = 10.92
	maximumPlayerSpeed            = 200
)

// playerAbilitiesWithSpeed converts a steady movement rate to client ability values.
func playerAbilitiesWithSpeed(mode int32, speed float64, flying bool) *playpacket.PlayerAbilitiesClientbound {
	abilities := playpacket.SurvivalAbilities()
	if mode == 1 {
		abilities = playpacket.CreativeAbilities()
		if flying {
			abilities.Flags |= playpacket.AbilityFlying
		}
	} else if mode == 3 {
		abilities.Flags = playpacket.AbilityInvulnerable | playpacket.AbilityAllowFlying | playpacket.AbilityFlying
	}
	if speed > 0 {
		abilities.FlySpeed = float32(0.05 * speed / defaultFlyingBlocksPerSecond)
		abilities.FOVModifier = float32(0.1 * speed / defaultWalkingBlocksPerSecond)
	}
	return abilities
}

// ServerControlAdapter adapts session-layer components to the command.ServerControl interface.
type ServerControlAdapter struct {
	PM                        *PlayerManager
	OpList                    *operator.List
	StopFunc                  func()
	SeedValue                 int64
	World                     WorldTimeAccessor
	WeatherWorld              WeatherAccessor
	WorldAccess               WorldAccessor
	ResolveWorldAccess        func(Session) WorldAccessor
	MobSpawn                  MobSpawner
	ResourceReload            func() error
	PregenerationWorlds       *world.Manager
	DefaultPregenerationWorld string
	GameRules                 map[string]string
	rulesMu                   sync.RWMutex
	DefaultGM                 int32
	SpawnX, SpawnY, SpawnZ    int
}

// ReloadResources atomically reloads server datapacks.
func (s *ServerControlAdapter) ReloadResources() error {
	if s == nil || s.ResourceReload == nil {
		return fmt.Errorf("resource reload is unavailable")
	}
	return s.ResourceReload()
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
	apply := func() {
		if s.World != nil {
			s.World.SetTimeOfDay(time)
		}
		if s.PM == nil {
			return
		}
		age := int64(0)
		tod := time
		if s.World != nil {
			age = s.World.WorldAge()
			tod = s.World.TimeOfDay()
		}
		s.PM.Broadcast(&playpacket.UpdateTime{WorldAge: age, TimeOfDay: tod, TickDayTime: true})
	}
	if submitter, ok := s.World.(worldCommandSubmitter); ok {
		_ = submitter.Submit(apply)
		return
	}
	apply()
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
	apply := func() { s.WeatherWorld.SetWeather(state, int32(duration)) }
	if submitter, ok := s.WeatherWorld.(worldCommandSubmitter); ok {
		_ = submitter.Submit(apply)
		return
	}
	apply()
}

func (s *ServerControlAdapter) SetGameMode(entityID int32, mode int32) error {
	if s.PM == nil {
		return fmt.Errorf("player manager is unavailable")
	}
	if mode < 0 || mode > 3 {
		return fmt.Errorf("invalid game mode: %d", mode)
	}
	if s.PM.GetByEntityID(entityID) == nil {
		return fmt.Errorf("player not found")
	}
	submitter, ok := s.World.(worldCommandSubmitter)
	if !ok || !submitter.Submit(func() {
		s.PM.mu.Lock()
		defer s.PM.mu.Unlock()
		for _, p := range s.PM.players {
			if p.EntityID != entityID {
				continue
			}
			p.GameMode = mode
			_ = p.Session.Send(&playpacket.GameEvent{Event: 3, Value: float32(mode)})
			if player, ok := p.Session.Player().(*entity.Player); ok && player != nil {
				player.Living().SetGameMode(mode)
			}
			flying := mode == 3
			if mode == 1 {
				if ds, ok := p.Session.(*DefaultSession); ok {
					value, _ := ds.SessionData().Load("player_flying")
					flying, _ = value.(bool)
				}
			} else if ds, ok := p.Session.(*DefaultSession); ok {
				ds.SessionData().Store("player_flying", flying)
			}
			_ = p.Session.Send(playerAbilitiesWithSpeed(mode, p.SpeedBlocksPerSecond, flying))
			return
		}
	}) {
		return fmt.Errorf("world command queue is unavailable or full")
	}
	return nil
}

// SetPlayerSpeed updates a player's movement speed on the authoritative world tick.
// A zero value restores the ordinary walking and flying speeds.
func (s *ServerControlAdapter) SetPlayerSpeed(entityID int32, blocksPerSecond float64) error {
	if math.IsNaN(blocksPerSecond) || math.IsInf(blocksPerSecond, 0) || blocksPerSecond < 0 || blocksPerSecond > maximumPlayerSpeed {
		return fmt.Errorf("invalid movement speed")
	}
	if s.PM == nil || s.PM.GetByEntityID(entityID) == nil {
		return fmt.Errorf("player not found")
	}
	submitter, ok := s.World.(worldCommandSubmitter)
	if !ok || !submitter.Submit(func() {
		s.PM.mu.Lock()
		defer s.PM.mu.Unlock()
		for _, online := range s.PM.players {
			if online.EntityID != entityID {
				continue
			}
			player, ok := online.Session.Player().(*entity.Player)
			if !ok || player == nil {
				return
			}
			attributeSpeed := 0.1
			if blocksPerSecond > 0 {
				attributeSpeed *= blocksPerSecond / defaultWalkingBlocksPerSecond
			}
			player.Attributes().SetBase(genattr.AttrMovementSpeed, attributeSpeed)
			online.SpeedBlocksPerSecond = blocksPerSecond
			flying := false
			if ds, ok := online.Session.(*DefaultSession); ok {
				value, _ := ds.SessionData().Load("player_flying")
				flying, _ = value.(bool)
			}
			_ = online.Session.Send(playerAbilitiesWithSpeed(online.GameMode, blocksPerSecond, flying))
			_ = online.Session.Send(&playpacket.EntityUpdateAttributes{
				EntityID: entityID, Properties: player.Attributes().ToProperties(),
			})
			return
		}
	}) {
		return fmt.Errorf("world command queue is unavailable or full")
	}
	return nil
}

func (s *ServerControlAdapter) TeleportPlayer(entityID int32, x, y, z float64) error {
	if s.PM == nil {
		return fmt.Errorf("player manager is unavailable")
	}
	if !finiteWorldPosition(x) || !finiteWorldPosition(y) || !finiteWorldPosition(z) {
		return fmt.Errorf("invalid teleport position")
	}
	if s.PM.GetByEntityID(entityID) == nil {
		return fmt.Errorf("player not found")
	}
	submitter, ok := s.World.(worldCommandSubmitter)
	if !ok || !submitter.Submit(func() {
		s.PM.mu.Lock()
		defer s.PM.mu.Unlock()
		for _, p := range s.PM.players {
			if p.EntityID != entityID {
				continue
			}
			p.X, p.Y, p.Z = x, y, z
			if player, ok := p.Session.Player().(MovablePlayer); ok && player != nil {
				player.SetPositionXYZ(x, y, z)
			}
			teleportID := nextTeleportID()
			ds, hasSession := p.Session.(*DefaultSession)
			if hasSession {
				ds.setPendingTeleport(teleportID)
			}
			if err := p.Session.Send(&playpacket.SyncPlayerPosition{
				TeleportID: teleportID,
				X:          x,
				Y:          y,
				Z:          z,
			}); err != nil && hasSession {
				ds.pendingTeleport.CompareAndSwap(teleportID, 0)
			}
			return
		}
	}) {
		return fmt.Errorf("world command queue is unavailable or full")
	}
	return nil
}

func finiteWorldPosition(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value) <= 30_000_000
}

func (s *ServerControlAdapter) GiveItem(entityID int32, itemName string, count int) error {
	if s.PM == nil || (s.WorldAccess == nil && s.ResolveWorldAccess == nil) {
		return fmt.Errorf("world inventory service is unavailable")
	}
	itemID := item.IDByName(itemName)
	if itemID <= 0 {
		return fmt.Errorf("unknown item: %s", itemName)
	}
	if count <= 0 || count > 6400 {
		return fmt.Errorf("item count out of range: %d", count)
	}
	online := s.PM.GetByEntityID(entityID)
	if online == nil {
		return fmt.Errorf("player not found")
	}
	accessor := s.WorldAccess
	players := s.PM
	if s.ResolveWorldAccess != nil {
		accessor = s.ResolveWorldAccess(online.Session)
		players = s.PM.ForDimension(SessionDimension(online.Session))
	}
	if accessor == nil || !accessor.Submit(&giveItemCommand{
		session:  online.Session,
		players:  players,
		world:    accessor,
		entityID: entityID,
		itemID:   itemID,
		count:    int32(count),
	}) {
		return fmt.Errorf("world command queue is full")
	}
	return nil
}

type giveItemCommand struct {
	session  Session
	players  *PlayerManager
	world    WorldAccessor
	entityID int32
	itemID   int32
	count    int32
}

// Execute inserts the item on the world tick and drops any inventory overflow.
func (c *giveItemCommand) Execute() {
	op := c.players.GetByEntityID(c.entityID)
	if op == nil || op.Windows == nil || op.Session == nil {
		return
	}
	maxStack := item.StackSize(c.itemID)
	if maxStack <= 0 {
		return
	}
	indices := make([]int, 0, inventory.MainInventoryEnd-inventory.MainInventoryStart+10)
	for index := inventory.HotbarStart; index <= inventory.HotbarEnd; index++ {
		indices = append(indices, index)
	}
	for index := inventory.MainInventoryStart; index <= inventory.MainInventoryEnd; index++ {
		indices = append(indices, index)
	}

	inv := op.Windows.Inventory()
	remainder, changed := inv.Insert(inventory.NewSlot(c.itemID, c.count), maxStack, indices)
	if len(changed) > 0 {
		_ = sendWindowResync(op.Session, op.Windows, 0)
	}
	for !remainder.Empty() {
		dropCount := min(remainder.ItemCount, maxStack)
		spawnCommandItem(c.players, c.world, op.X, op.Y, op.Z, c.itemID, dropCount)
		remainder.ItemCount -= dropCount
	}
}

func spawnCommandItem(players *PlayerManager, world WorldAccessor, x, y, z float64, itemID, count int32) {
	spawnInventoryItem(players, world, x, y, z, inventory.NewSlot(itemID, count))
}

func spawnInventoryItem(players *PlayerManager, world WorldAccessor, x, y, z float64, stack inventory.Slot) {
	if world == nil || stack.Empty() {
		return
	}
	drop := world.SpawnItemDrop(x, y, z, stack.ItemID, stack.ItemCount)
	if drop == nil {
		return
	}
	drop.SetStack(stack.Clone())
	if players == nil || worldOwnsItemTracking(world) {
		return
	}
	position := drop.Position()
	velocity := drop.Velocity()
	players.Broadcast(&playpacket.SpawnEntity{
		EntityID:   drop.ID(),
		EntityUUID: drop.UUID(),
		Type:       genentity.EntityItem,
		X:          position.X,
		Y:          position.Y,
		Z:          position.Z,
		VelocityX:  velocityToProtocol(velocity.X),
		VelocityY:  velocityToProtocol(velocity.Y),
		VelocityZ:  velocityToProtocol(velocity.Z),
	})
	data := metadata.New()
	data.SetSlot(metadata.IndexItemEntityItem, drop.Stack())
	players.Broadcast(&playpacket.SetEntityMetadata{EntityID: drop.ID(), Metadata: data.Encode()})
}

func (s *ServerControlAdapter) KillEntity(entityID int32) error {
	if s.PM == nil {
		return fmt.Errorf("player manager is unavailable")
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
	return fmt.Errorf("player not found")
}

func (s *ServerControlAdapter) SetDifficulty(difficulty int) error {
	if difficulty < 0 || difficulty > 3 {
		return fmt.Errorf("invalid difficulty: %d", difficulty)
	}
	world, ok := s.World.(DifficultyAccessor)
	if !ok || world == nil {
		return fmt.Errorf("difficulty service is unavailable")
	}
	apply := func() { world.SetDifficulty(int32(difficulty)) }
	if submitter, ok := s.World.(worldCommandSubmitter); ok {
		if !submitter.Submit(apply) {
			return fmt.Errorf("world command queue is full")
		}
		return nil
	}
	apply()
	return nil
}

func (s *ServerControlAdapter) SetOp(name string, level int) error {
	if s.PM == nil {
		return fmt.Errorf("player manager is unavailable")
	}
	if s.OpList == nil {
		return fmt.Errorf("operator list is unavailable")
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
	return fmt.Errorf("player not found: %s", name)
}

func (s *ServerControlAdapter) RemoveOp(name string) error {
	if s.PM == nil {
		return fmt.Errorf("player manager is unavailable")
	}
	if s.OpList == nil {
		return fmt.Errorf("operator list is unavailable")
	}
	s.PM.mu.RLock()
	defer s.PM.mu.RUnlock()
	for _, p := range s.PM.players {
		if p.Name == name {
			return s.OpList.Remove(p.UUID)
		}
	}
	return fmt.Errorf("player not found: %s", name)
}

func (s *ServerControlAdapter) KickPlayer(name string, reason string) error {
	if s.PM == nil {
		return fmt.Errorf("player manager is unavailable")
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
		return fmt.Errorf("player not found: %s", name)
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
	if world, ok := s.World.(interface{ GameRule(string) string }); ok {
		return world.GameRule(name), nil
	}
	s.rulesMu.RLock()
	defer s.rulesMu.RUnlock()
	if s.GameRules != nil {
		if v, ok := s.GameRules[name]; ok {
			return v, nil
		}
	}
	return gamerules.Default(name), nil
}

func (s *ServerControlAdapter) SetGameRule(name, value string) error {
	if err := gamerules.Validate(name, value); err != nil {
		return err
	}
	info := gamerules.GameRuleByName(name)
	if info == nil {
		return fmt.Errorf("unknown game rule: %s", name)
	}
	if info.Type == "boolean" {
		if value != "true" && value != "false" {
			return fmt.Errorf("game rule %s requires true or false", name)
		}
	} else if info.Type == "integer" {
		if _, err := strconv.ParseInt(value, 10, 32); err != nil {
			return fmt.Errorf("game rule %s requires a 32-bit integer: %w", name, err)
		}
	}
	if world, ok := s.World.(interface{ SetGameRule(string, string) error }); ok {
		apply := func() {
			if err := world.SetGameRule(name, value); err != nil {
				logger.Warn("set game rule failed", "rule", name, "error", err)
				return
			}
			if s.PM != nil && (name == "doImmediateRespawn" || name == "doLimitedCrafting") {
				event := byte(12)
				enabled := value == "true"
				if name == "doImmediateRespawn" {
					event = 11
					enabled = !enabled
				}
				number := float32(0)
				if enabled {
					number = 1
				}
				s.PM.Broadcast(&playpacket.GameEvent{Event: event, Value: number})
			}
		}
		if submitter, ok := s.World.(worldCommandSubmitter); ok {
			if !submitter.Submit(apply) {
				return fmt.Errorf("world command queue is full")
			}
			return nil
		}
		return world.SetGameRule(name, value)
	}
	if world, ok := s.World.(worldGameRules); ok {
		if name == "doDaylightCycle" || name == "doWeatherCycle" || name == "doFireTick" {
			enabled := value == "true"
			apply := func() {
				switch name {
				case "doDaylightCycle":
					world.SetDaylightCycle(enabled)
				case "doWeatherCycle":
					world.SetWeatherCycle(enabled)
				case "doFireTick":
					world.SetFireTick(enabled)
				}
			}
			if submitter, ok := s.World.(worldCommandSubmitter); ok {
				if !submitter.Submit(apply) {
					return fmt.Errorf("world command queue is full")
				}
			} else {
				apply()
			}
		}
	}
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
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
	return s.SetWorldSpawnAngle(x, y, z, 0)
}

// SetWorldSpawnAngle applies persistent spawn coordinates and yaw on the tick.
func (s *ServerControlAdapter) SetWorldSpawnAngle(x, y, z int, angle float32) error {
	if math.IsNaN(float64(angle)) || math.IsInf(float64(angle), 0) {
		return fmt.Errorf("invalid spawn angle")
	}
	if x < -30000000 || x > 30000000 || z < -30000000 || z > 30000000 || y < -64 || y > 319 {
		return fmt.Errorf("spawn position out of bounds")
	}
	apply := func() {
		if world, ok := s.World.(interface {
			SetWorldSpawn(int32, int32, int32, float32)
		}); ok {
			world.SetWorldSpawn(int32(x), int32(y), int32(z), angle)
		}
		s.SpawnX, s.SpawnY, s.SpawnZ = x, y, z
		if s.PM != nil {
			s.PM.Broadcast(&playpacket.SetDefaultSpawnPosition{
				X: int32(x), Y: int32(y), Z: int32(z), Angle: angle,
			})
		}
	}
	if submitter, ok := s.World.(worldCommandSubmitter); ok {
		if !submitter.Submit(apply) {
			return fmt.Errorf("world command queue is full")
		}
	} else {
		apply()
	}
	return nil
}

func (s *ServerControlAdapter) SetSpawnPoint(entityID int32, x, y, z int) error {
	return s.SetSpawnPointAngle(entityID, x, y, z, 0)
}

// SetSpawnPointAngle sets a forced player respawn position and yaw on the tick.
func (s *ServerControlAdapter) SetSpawnPointAngle(entityID int32, x, y, z int, angle float32) error {
	if math.IsNaN(float64(angle)) || math.IsInf(float64(angle), 0) {
		return fmt.Errorf("invalid spawn angle")
	}
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	op := s.PM.GetByEntityID(entityID)
	if op == nil {
		return fmt.Errorf("player not found")
	}
	if x < -30000000 || x > 30000000 || z < -30000000 || z > 30000000 || y < -64 || y > 319 {
		return fmt.Errorf("spawn position out of bounds")
	}
	apply := func() {
		op.SpawnX, op.SpawnY, op.SpawnZ = x, y, z
		op.HasSpawnPoint, op.SpawnForced = true, true
		op.SpawnDimension, op.SpawnAngle = SessionDimension(op.Session), angle
	}
	if submitter, ok := s.World.(worldCommandSubmitter); ok {
		if !submitter.Submit(apply) {
			return fmt.Errorf("world command queue is full")
		}
	} else {
		apply()
	}
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
	apply := func() { s.MobSpawn.SummonMob(entityType, x, y, z) }
	if submitter, ok := s.MobSpawn.(worldCommandSubmitter); ok {
		if !submitter.Submit(apply) {
			return fmt.Errorf("world command queue is full")
		}
		return nil
	}
	apply()
	return nil
}

func findEffectByName(name string) *effectdata.EffectInfo {
	if info := effectdata.EffectByName(name); info != nil {
		return info
	}
	lower := strings.ToLower(name)
	for i := range effectdata.Effects {
		if strings.ToLower(effectdata.Effects[i].Name) == lower {
			return &effectdata.Effects[i]
		}
	}
	return nil
}

func (s *ServerControlAdapter) ApplyEffect(entityID int32, effectName string, durationTicks int32, amplifier int32) error {
	if s.PM == nil {
		return fmt.Errorf("no player manager")
	}
	info := findEffectByName(effectName)
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
		info := findEffectByName(effectName)
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
