package session

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block/fluid"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	effect "github.com/vitismc/vitis/internal/data/generated/effect"
	pose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// abilitiesCommand validates flight requests against the current server game mode.
type abilitiesCommand struct {
	session Session
	players *PlayerManager
	flying  bool
}

// OwnerSession binds flight requests to the dimension that accepted them.
func (c *abilitiesCommand) OwnerSession() Session { return c.session }

// Execute changes only the capability that clients may request.
func (c *abilitiesCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil || c.players == nil {
		return
	}
	online := c.players.GetBySession(c.session)
	player, ok := c.session.Player().(*entity.Player)
	if online == nil || !ok || player == nil || player.IsDead() {
		return
	}
	mode := player.GameMode()
	flying := mode == 3 || mode == 1 && c.flying
	if concrete, ok := c.session.(*DefaultSession); ok {
		concrete.SessionData().Store("player_flying", flying)
	}
	if flying {
		player.SetFallDistance(0)
		online.movement.floatingTicks = 0
	}
	if err := c.session.Send(playerAbilitiesWithSpeed(mode, online.SpeedBlocksPerSecond, flying)); err != nil {
		logger.Warn("sync player abilities failed", "session", c.session.ID(), "error", err)
	}
}

// playerFlying reads an accepted flight capability on the authoritative tick.
func playerFlying(s Session, player *entity.Player) bool {
	if player.GameMode() == 3 {
		return true
	}
	if player.GameMode() != 1 {
		return false
	}
	if concrete, ok := s.(*DefaultSession); ok {
		value, _ := concrete.SessionData().Load("player_flying")
		flying, _ := value.(bool)
		return flying
	}
	return false
}

// movementFluid detects fluid surfaces intersecting the player's current body.
func movementFluid(world physics.BlockAccess, pos entity.Vec3, height float64) bool {
	box := physics.NewAABB(pos.X-.3, pos.Y, pos.Z-.3, pos.X+.3, pos.Y+height, pos.Z+.3).Contract(.001)
	for x := int(math.Floor(box.MinX)); x <= int(math.Floor(box.MaxX)); x++ {
		for z := int(math.Floor(box.MinZ)); z <= int(math.Floor(box.MaxZ)); z++ {
			for y := int(math.Floor(box.MinY)); y <= int(math.Floor(box.MaxY)); y++ {
				kind, surface := movementFluidSurface(world, x, y, z)
				if kind != fluid.FluidTypeNone && box.MinY < float64(y)+surface {
					return true
				}
			}
		}
	}
	return false
}

// movementFluidSurface returns the fluid type and its occupied height in one block.
func movementFluidSurface(world physics.BlockAccess, x, y, z int) (fluid.FluidType, float64) {
	state := world.GetBlockStateAt(x, y, z)
	kind := fluid.ContainedFluidType(state)
	if kind == fluid.FluidTypeNone {
		return kind, 0
	}
	if fluid.ContainedFluidType(world.GetBlockStateAt(x, y+1, z)) == kind {
		return kind, 1
	}
	level := 8
	if fluid.GetFluidType(state) != fluid.FluidTypeNone {
		level = fluid.EffectiveLevel(state)
	}
	return kind, float64(level) / 9
}

// movementEyesInWater checks water depth at the current pose's eye position.
func movementEyesInWater(world physics.BlockAccess, pos entity.Vec3, online *OnlinePlayer) bool {
	height := 1.62
	switch online.Pose {
	case pose.PoseCrouching:
		height = 1.27
	case pose.PoseSwimming, pose.PoseFallFlying:
		height = .4
	}
	eyes := pos.Y + height
	x, y, z := int(math.Floor(pos.X)), int(math.Floor(eyes)), int(math.Floor(pos.Z))
	kind, surface := movementFluidSurface(world, x, y, z)
	return kind == fluid.FluidTypeWater && eyes < float64(y)+surface
}

// applySwimmingExhaustion charges accepted three-dimensional fluid movement.
func applySwimmingExhaustion(s Session, oldX, oldY, oldZ, x, y, z float64) {
	player, ok := s.Player().(*entity.Player)
	if !ok || player.IsDead() || player.GameMode() == 1 || player.GameMode() == 3 {
		return
	}
	dx, dy, dz := x-oldX, y-oldY, z-oldZ
	player.AddExhaustion(float32(math.Sqrt(dx*dx+dy*dy+dz*dz) * .01))
}

// canGlide verifies equipped wings and conditions that permit starting glide.
func canGlide(s Session, online *OnlinePlayer, player *entity.Player, world physics.BlockAccess) bool {
	if online.Windows == nil || online.Sleeping || !player.HasClientLoaded() || player.OnGround() || playerFlying(s, player) || player.Effects().Has(effect.EffectLevitation) || movementFluid(world, player.Position(), movementDimensions(online).Height) {
		return false
	}
	return len(equippedGliders(online.Windows)) != 0
}

// equippedGliders resolves usable glider and equippable components in occupied slots.
func equippedGliders(windows *inventory.WindowManager) []int {
	indices := []int{inventory.HotbarStart + int(windows.HeldSlot()), inventory.OffhandSlot, inventory.ArmorStart, inventory.ArmorStart + 1, inventory.ArmorStart + 2, inventory.ArmorEnd}
	names := []string{"mainhand", "offhand", "head", "chest", "legs", "feet"}
	var usable []int
	for i, index := range indices {
		stack := windows.Inventory().Get(index)
		if stack.Empty() {
			continue
		}
		if _, ok := stack.Component(dc.ComponentGlider); !ok {
			continue
		}
		equippable, ok := stack.Component(dc.ComponentEquippable)
		if !ok {
			continue
		}
		structured, ok := equippable.(inventory.StructuredComponent)
		if !ok {
			continue
		}
		fields, ok := structured.Data.(map[string]any)
		if !ok || fields["slot"] != names[i] {
			continue
		}
		maximum, damageable := stack.Component(dc.ComponentMaxDamage)
		_, unbreakable := stack.Component(dc.ComponentUnbreakable)
		if limit, ok := maximum.(inventory.VarIntComponent); damageable && ok && !unbreakable && stack.Damage >= int32(limit)-1 {
			continue
		}
		usable = append(usable, index)
	}
	return usable
}

// tickGliding charges equipment wear every twenty authoritative glide ticks.
func tickGliding(players *PlayerManager, online *OnlinePlayer, player *entity.Player, world physics.BlockAccess) {
	if online.Flags&0x80 == 0 {
		online.movement.glidingTicks = 0
		return
	}
	online.movement.glidingTicks++
	if online.movement.glidingTicks%20 != 0 || player.GameMode() == 1 || player.GameMode() == 3 {
		return
	}
	indices := equippedGliders(online.Windows)
	if len(indices) == 0 {
		return
	}
	index := indices[rand.Intn(len(indices))]
	stack := online.Windows.Inventory().Get(index)
	if _, ok := stack.Component(dc.ComponentMaxDamage); !ok {
		return
	}
	if _, ok := stack.Component(dc.ComponentUnbreakable); ok {
		return
	}
	if !item.ShouldConsumeDurability(stack.EnchantLevel(item.EnchantUnbreakingID), false) {
		return
	}
	stack.SetDamage(stack.Damage + 1)
	online.Windows.Inventory().Set(index, stack)
	if err := sendWindowResync(online.Session, online.Windows, 0); err != nil {
		logger.Warn("sync glider wear failed", "session", online.Session.ID(), "error", err)
	}
	if packet := BuildFullEquipmentPacket(player.ID(), online.Windows); packet != nil {
		players.BroadcastExcept(online.UUID, packet)
	}
	updateMovementPose(online.Session, players, online, player, world)
}

// updateMovementPose derives a fitting pose and publishes server corrections.
func updateMovementPose(s Session, players *PlayerManager, online *OnlinePlayer, player *entity.Player, world physics.BlockAccess) {
	if online.Sleeping || player.IsDead() {
		return
	}
	previousPose, previousFlags := online.Pose, online.Flags
	wanted := pose.PoseStanding
	if online.Flags&0x80 != 0 {
		if canGlide(s, online, player, world) {
			wanted = pose.PoseFallFlying
		} else {
			online.Flags &^= 0x80
		}
	}
	pos := player.Position()
	if wanted == pose.PoseStanding {
		if online.Flags&0x08 != 0 && movementEyesInWater(world, pos, online) && !playerFlying(s, player) {
			wanted = pose.PoseSwimming
		} else if online.Flags&0x02 != 0 {
			wanted = pose.PoseCrouching
		}
	}
	if player.GameMode() != 3 {
		for _, candidate := range []int32{wanted, pose.PoseCrouching, pose.PoseSwimming} {
			copy := *online
			copy.Pose = candidate
			box := movementDimensions(&copy).MakeBoundingBox(pos.X, pos.Y, pos.Z).Contract(1e-7)
			if len(physics.CollectBlockCollisions(world, box)) == 0 {
				wanted = candidate
				break
			}
		}
	}
	online.Pose = wanted
	if previousPose != wanted || previousFlags != online.Flags {
		data := metadata.New()
		data.SetByte(metadata.IndexBase, int8(online.Flags))
		data.SetPose(metadata.IndexPose, wanted)
		packet := &play.SetEntityMetadata{EntityID: player.ID(), Metadata: data.Encode()}
		players.BroadcastExcept(online.UUID, packet)
		if err := s.Send(packet); err != nil {
			logger.Warn("sync player pose failed", "session", s.ID(), "error", err)
		}
	}
}

// TickPlayerMovement checks sustained floating once per world tick, including stationary clients.
func TickPlayerMovement(players *PlayerManager, world WorldAccessor) {
	geometry, ok := world.(movementGeometry)
	if players == nil || !ok || !geometry.HasMovementGeometry() {
		return
	}
	for _, online := range players.Players() {
		player, ok := online.Session.Player().(*entity.Player)
		if !ok || player == nil || player.IsDead() || !player.HasClientLoaded() || online.Session.Context().Err() != nil {
			online.movement.floatingTicks = 0
			continue
		}
		if concrete, ok := online.Session.(*DefaultSession); ok && concrete.hasPendingTeleport() {
			online.movement.floatingTicks = 0
			continue
		}
		updateMovementPose(online.Session, players, online, player, geometry)
		tickGliding(players, online, player, geometry)
		pos := player.Position()
		box := movementDimensions(online).MakeBoundingBox(pos.X, pos.Y, pos.Z).Grow(.0625).Expand(0, -.55, 0)
		descending := online.movement.tick == geometry.MovementTick() && online.movement.descending
		if player.GameMode() == 1 || player.GameMode() == 3 || online.Sleeping || online.Flags&0x80 != 0 || player.Effects().Has(effect.EffectLevitation) || player.Effects().Has(effect.EffectSlowFalling) || descending || movementFluid(geometry, pos, movementDimensions(online).Height) || len(physics.CollectBlockCollisions(geometry, box)) != 0 {
			online.movement.floatingTicks = 0
			continue
		}
		online.movement.floatingTicks++
		if online.movement.floatingTicks > 80 {
			if err := online.Session.ForceClose(fmt.Errorf("flying is not enabled on this server")); err != nil {
				logger.Debug("close floating player failed", "session", online.Session.ID(), "error", err)
			}
		}
	}
}
