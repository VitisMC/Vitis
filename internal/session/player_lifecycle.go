package session

import (
	"errors"
	"math"
	"strconv"

	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/logger"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// ProcessPlayerDeath drops inventory contents and returns the experience to spawn.
func ProcessPlayerDeath(player *entity.Player, players *PlayerManager, world WorldAccessor, keepInventory bool) int32 {
	if player == nil || players == nil || world == nil {
		return 0
	}
	session, ok := player.Session().(Session)
	if !ok {
		return 0
	}
	online := players.GetBySession(session)
	if online == nil || online.Windows == nil || online.DeathProcessed {
		return 0
	}
	online.DeathProcessed = true
	CancelPlayerActions(session)
	online.DeathKeepInventory = keepInventory
	if online.Sleeping {
		wakePlayer(players, online, world)
	}
	if active := online.Windows.ActiveWindow(); active != nil {
		_ = session.Send(&playpacket.ClientboundCloseContainer{WindowID: active.ID})
	}
	if keepInventory {
		for _, stack := range online.Windows.CloseTemporaryCrafting() {
			if !stack.Empty() {
				position := player.Position()
				spawnInventoryItem(players, world, position.X, position.Y+0.5, position.Z, stack)
			}
		}
		_ = sendWindowResync(session, online.Windows, 0)
		return 0
	}
	position := player.Position()
	for _, stack := range online.Windows.DropAllOnDeath() {
		if !stack.Empty() {
			spawnInventoryItem(players, world, position.X, position.Y+0.5, position.Z, stack)
		}
	}
	online.Windows.Inventory().Set(inventory.CraftOutputSlot, inventory.EmptySlot())
	_ = sendWindowResync(session, online.Windows, 0)
	level := player.Living().XPLevel()
	player.Living().SetXP(0, 0, 0)
	_ = session.Send(&playpacket.SetExperience{})
	return min(int32(100), level*7)
}

// bedUseCommand validates a bed use after the packet enters the world queue.
type bedUseCommand struct {
	session  Session
	players  *PlayerManager
	world    WorldAccessor
	x, y, z  int
	sequence int32
}

// Execute validates and applies a bed interaction on the world tick.
func (c *bedUseCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	player, ok := c.session.Player().(*entity.Player)
	if !ok || player == nil || !blockWithinReach(player, playpacket.BlockPos{X: int32(c.x), Y: int32(c.y), Z: int32(c.z)}, 6) {
		_ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
		return
	}
	state := c.world.GetBlock(c.x, c.y, c.z)
	if !behavior.IsBed(state) {
		_ = c.session.Send(&playpacket.AcknowledgeBlockChange{Sequence: c.sequence})
		return
	}
	if err := handleBedUse(c.session, c.players, c.world, state, c.x, c.y, c.z, c.sequence); err != nil {
		logger.Warn("bed use failed", "session", c.session.ID(), "error", err)
	}
}

// wakeBedCommand clears sleeping state on the authoritative tick.
type wakeBedCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
}

// Execute wakes the player and releases their bed on the world tick.
func (c *wakeBedCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.players == nil {
		return
	}
	if player := c.players.GetBySession(c.session); player != nil {
		wakePlayer(c.players, player, c.world)
	}
}

// respawnPlayerCommand restores a dead player on the authoritative tick.
type respawnPlayerCommand struct {
	session Session
	players *PlayerManager
	world   WorldAccessor
	config  PlayBootstrapConfig
}

// Execute respawns a dead player on the world tick.
func (c *respawnPlayerCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.players == nil {
		return
	}
	online := c.players.GetBySession(c.session)
	player, ok := c.session.Player().(*entity.Player)
	if online == nil || !ok || player == nil || !player.Living().IsDead() || online.RespawnPending {
		return
	}
	online.RespawnPending = true
	if !player.Living().DeathHandled() {
		return
	}
	if err := handleRespawn(c.session, c.config, c.players, c.world); err != nil {
		if errors.Is(err, errRespawnChunksPending) || errors.Is(err, errRespawnObstructed) {
			online.RespawnRetryTicks = 20
			return
		}
		online.RespawnPending = false
		logger.Warn("respawn failed", "session", c.session.ID(), "error", err)
	}
	if !player.Living().IsDead() {
		online.RespawnPending = false
	}
}

var errRespawnChunksPending = errors.New("respawn chunks pending")
var errRespawnObstructed = errors.New("respawn area obstructed")

// prepareRespawnChunks starts bounded asynchronous loads before a transition.
func prepareRespawnChunks(world WorldAccessor, x, z float64) error {
	accessor, ok := world.(*DefaultWorldAccessor)
	if !ok || accessor.Chunks == nil {
		return nil
	}
	cx, cz := int32(math.Floor(x/16)), int32(math.Floor(z/16))
	ready := true
	for dx := int32(-1); dx <= 1; dx++ {
		for dz := int32(-1); dz <= 1; dz++ {
			if cx+dx < -1875000 || cx+dx >= 1875000 || cz+dz < -1875000 || cz+dz >= 1875000 {
				continue
			}
			if _, loaded := accessor.GetChunk(cx+dx, cz+dz); !loaded {
				accessor.Chunks.RequestLoad(cx+dx, cz+dz)
				ready = false
			}
		}
	}
	if !ready {
		return errRespawnChunksPending
	}
	return nil
}

// playerRespawnPoint resolves a complete bed or a forced command spawn.
func playerRespawnPoint(world WorldAccessor, player *OnlinePlayer) (float64, float64, float64, bool) {
	return playerRespawnPointInDimension(world, player, SessionDimension(player.Session))
}

// playerRespawnPointInDimension resolves a spawn before publishing destination membership.
func playerRespawnPointInDimension(world WorldAccessor, player *OnlinePlayer, dimension string) (float64, float64, float64, bool) {
	if player.SpawnDimension != "" && player.SpawnDimension != dimension {
		return 0, 0, 0, false
	}
	if player.SpawnForced {
		return safeLocalRespawn(world, player.SpawnX, player.SpawnY, player.SpawnZ)
	}
	return safeBedRespawn(world, player.SpawnX, player.SpawnY, player.SpawnZ)
}

// safeLocalRespawn searches near an explicit position, including underground spawns.
func safeLocalRespawn(world WorldAccessor, x, y, z int) (float64, float64, float64, bool) {
	if world == nil {
		return 0, 0, 0, false
	}
	y = max(-63, min(y, 318))
	for radius := 0; radius <= 4; radius++ {
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if radius > 0 && absInt(dx) != radius && absInt(dz) != radius {
					continue
				}
				px, pz := x+dx, z+dz
				if _, ok := world.GetChunk(int32(math.Floor(float64(px)/16)), int32(math.Floor(float64(pz)/16))); !ok {
					continue
				}
				for step := 0; step < 768; step++ {
					py := y + step/2
					if step%2 == 1 {
						py = y - (step+1)/2
					}
					if py < -63 || py > 318 {
						continue
					}
					if sx, sy, sz, valid := standingSpawnAt(world, px, py, pz); valid {
						return sx, sy, sz, true
					}
				}
			}
		}
	}
	return 0, 0, 0, false
}

// absInt returns the magnitude of a bounded coordinate offset.
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// sleepThunder checks the weather dependency used by bed interaction.
func sleepThunder(world WorldAccessor) bool {
	weather, ok := world.(interface{ Thundering() bool })
	return ok && weather.Thundering()
}

// TickPlayerLifecycle advances sleep and pending respawns on the world tick.
func TickPlayerLifecycle(players *PlayerManager, world WorldAccessor, config PlayBootstrapConfig) {
	if players == nil {
		return
	}
	rule := func(name, fallback string) string {
		if config.GameRule != nil {
			return config.GameRule(name)
		}
		return fallback
	}
	eligible, deepSleeping := 0, 0
	for _, online := range players.Players() {
		if online.Session == nil || online.Session.Context().Err() != nil {
			continue
		}
		player, ok := online.Session.Player().(*entity.Player)
		if !ok || player == nil {
			continue
		}
		if online.Sleeping {
			head := world.GetBlock(online.SleepX, online.SleepY, online.SleepZ)
			_, _, _, valid := safeBedRespawn(world, online.SleepX, online.SleepY, online.SleepZ)
			if !behavior.IsBed(head) || !valid || player.Living().IsDead() {
				wakePlayer(players, online, world)
			} else {
				online.SleepTicks++
			}
		}
		if online.GameMode != 3 {
			eligible++
			if online.Sleeping && online.SleepTicks >= 100 {
				deepSleeping++
			}
		}
		if online.RespawnRetryTicks > 0 {
			online.RespawnRetryTicks--
		}
		if online.RespawnRetryTicks == 0 && player.Living().IsDead() && player.Living().DeathHandled() && (online.RespawnPending || rule("doImmediateRespawn", "false") == "true") {
			online.RespawnPending = true
			err := handleRespawn(online.Session, config, players, world)
			if err == nil {
				online.RespawnPending = false
			} else if errors.Is(err, errRespawnChunksPending) || errors.Is(err, errRespawnObstructed) {
				online.RespawnRetryTicks = 20
			} else {
				online.RespawnPending = false
				logger.Warn("complete respawn failed", "player", online.Name, "error", err)
			}
		}
	}
	percentage, err := strconv.Atoi(rule("playersSleepingPercentage", "100"))
	if err != nil {
		percentage = 100
	}
	required := max(1, int(math.Ceil(float64(eligible)*float64(percentage)/100)))
	if eligible > 0 && deepSleeping >= required {
		if rule("doDaylightCycle", "true") == "true" {
			skipToDawn(players, world)
		} else {
			if weather, ok := world.(interface{ ClearSleepWeather() }); ok {
				weather.ClearSleepWeather()
			}
			for _, online := range players.Players() {
				wakePlayer(players, online, world)
			}
		}
	}
}

// PreparePlayerDisconnect releases the occupied bed and transient inventory.
func PreparePlayerDisconnect(players *PlayerManager, online *OnlinePlayer, world WorldAccessor) {
	if online == nil {
		return
	}
	CancelPlayerActions(online.Session)
	wakePlayer(players, online, world)
	if online.Windows == nil {
		return
	}
	for _, stack := range online.Windows.CloseTemporaryCrafting() {
		spawnInventoryItem(players, world, online.X, online.Y+0.5, online.Z, stack)
	}
}
