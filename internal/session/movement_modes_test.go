package session

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	pose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// modeTestWorld supplies controllable fluids and a low ceiling above the resident floor.
type modeTestWorld struct {
	*movementTestWorld
	water   bool
	ceiling int32
}

// fluidMovementWorld supplies exact fluid states for body and surface checks.
type fluidMovementWorld map[[3]int]int32

// GetBlockStateAt reads one resident state from the fluid fixture.
func (w fluidMovementWorld) GetBlockStateAt(x, y, z int) int32 { return w[[3]int{x, y, z}] }

// TestMovementFluidChecksBodyAndSurface verifies edge contact, levels and waterlogged states.
func TestMovementFluidChecksBodyAndSurface(t *testing.T) {
	water := block.DefaultStateID("minecraft:water")
	flowing := block.StateID("minecraft:water", map[string]string{"level": "7"})
	logged := block.StateID("minecraft:oak_slab", map[string]string{"type": "bottom", "waterlogged": "true"})
	for _, test := range []struct {
		name     string
		world    fluidMovementWorld
		position entity.Vec3
		want     bool
	}{
		{"body_edge", fluidMovementWorld{{1, 64, 0}: water}, entity.Vec3{X: .9, Y: 64, Z: .5}, true},
		{"above_source", fluidMovementWorld{{0, 64, 0}: water}, entity.Vec3{X: .5, Y: 64.95, Z: .5}, false},
		{"above_flow", fluidMovementWorld{{0, 64, 0}: flowing}, entity.Vec3{X: .5, Y: 64.2, Z: .5}, false},
		{"inside_flow", fluidMovementWorld{{0, 64, 0}: flowing}, entity.Vec3{X: .5, Y: 64.05, Z: .5}, true},
		{"waterlogged", fluidMovementWorld{{0, 64, 0}: logged}, entity.Vec3{X: .5, Y: 64.5, Z: .5}, true},
		{"stacked_water", fluidMovementWorld{{0, 64, 0}: water, {0, 65, 0}: water}, entity.Vec3{X: .5, Y: 64.95, Z: .5}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := movementFluid(test.world, test.position, 1.8); got != test.want {
				t.Fatalf("fluid contact = %v, want %v", got, test.want)
			}
		})
	}
}

// TestGliderWearUsesTicksAndComponentDurability verifies equipment wear and safe exhaustion.
func TestGliderWearUsesTicksAndComponentDurability(t *testing.T) {
	for _, scenario := range []string{"ordinary", "nearly_broken", "unbreakable", "creative"} {
		t.Run(scenario, func(t *testing.T) {
			player, online, movement := movementTestPlayer(t)
			world := movement.world.(*movementTestWorld)
			player.SetPosition(entity.Vec3{X: .5, Y: 70, Z: .5})
			online.Y = 70
			online.Windows = inventory.NewWindowManager()
			wings := inventory.NewSlot(item.IDByName("minecraft:elytra"), 1)
			if scenario == "nearly_broken" {
				wings.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(4))
				wings.SetDamage(2)
			}
			if scenario == "unbreakable" {
				wings.SetComponent(dc.ComponentUnbreakable, inventory.BoolComponent(false))
			}
			if scenario == "creative" {
				player.SetGameMode(1)
				online.GameMode = 1
			}
			online.Windows.Inventory().Set(inventory.ArmorStart+1, wings)
			(&playerModeCommand{session: online.Session, players: movement.players, world: world, request: play.EntityAction{EntityID: player.ID(), ActionID: 8}}).Execute()
			if online.Flags&0x80 == 0 {
				t.Fatal("usable glider rejected")
			}
			for i := 0; i < 20; i++ {
				world.tick++
				movement.x, movement.y, movement.z = .5, 70, .5
				for range 3 {
					movement.Execute()
				}
				TickPlayerMovement(movement.players, world)
				if i < 19 && online.Windows.Inventory().Get(inventory.ArmorStart+1).Damage != wings.Damage {
					t.Fatal("packets accelerated durability wear")
				}
			}
			expected := wings.Damage
			if scenario == "ordinary" || scenario == "nearly_broken" {
				expected++
			}
			saved := online.Windows.Inventory().Get(inventory.ArmorStart + 1)
			if saved.Damage != expected {
				t.Fatalf("damage = %d, want %d", saved.Damage, expected)
			}
			if scenario == "nearly_broken" && (online.Flags&0x80 != 0 || online.Pose != pose.PoseStanding || saved.Empty()) {
				t.Fatal("exhausted glider did not stop safely")
			}
			encoded, err := inventory.ItemStackToNBT(saved)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := inventory.ItemStackFromNBT(encoded)
			if err != nil || restored.Damage != expected {
				t.Fatalf("saved glider wear: %v", err)
			}
		})
	}
}

// TestGliderComponentsControlEligibility verifies component removals and equipment slot matching.
func TestGliderComponentsControlEligibility(t *testing.T) {
	wm := inventory.NewWindowManager()
	wings := inventory.NewSlot(item.IDByName("minecraft:elytra"), 1)
	wings.ComponentPatch.Removed = []int32{dc.ComponentGlider}
	wm.Inventory().Set(inventory.ArmorStart+1, wings)
	if len(equippedGliders(wm)) != 0 {
		t.Fatal("removed glider component ignored")
	}
	helm := inventory.NewSlot(item.IDByName("minecraft:diamond_helmet"), 1)
	helm.SetComponent(dc.ComponentGlider, inventory.UnitComponent{})
	wm.Inventory().Set(inventory.ArmorStart, helm)
	if indices := equippedGliders(wm); len(indices) != 1 || indices[0] != inventory.ArmorStart {
		t.Fatalf("custom equipped glider = %v", indices)
	}
	wm.Inventory().Clear(inventory.ArmorStart)
	wm.Inventory().Set(inventory.HotbarStart, helm)
	if len(equippedGliders(wm)) != 0 {
		t.Fatal("equippable slot mismatch admitted glide")
	}
}

// TestFallDamageUsesCurrentEffects verifies slow falling, levitation and jump boost protection.
func TestFallDamageUsesCurrentEffects(t *testing.T) {
	for _, test := range []struct {
		name      string
		effect    int32
		amplifier int32
		health    float32
	}{
		{"ordinary", -1, 0, 12},
		{"slow_falling", effectdata.EffectSlowFalling, 0, 20},
		{"levitation", effectdata.EffectLevitation, 0, 20},
		{"jump_boost", effectdata.EffectJumpBoost, 1, 14},
	} {
		t.Run(test.name, func(t *testing.T) {
			player, _, movement := movementTestPlayer(t)
			player.SetFallDistance(10)
			if test.effect >= 0 {
				player.Effects().Add(effect.Instance{ID: test.effect, Amplifier: test.amplifier, Duration: 100})
			}
			checkFallDamage(movement.session, 65, 64, true)
			if player.Health() != test.health || player.FallDistance() != 0 {
				t.Fatalf("health=%v, fall distance=%v", player.Health(), player.FallDistance())
			}
		})
	}
}

// GetBlockStateAt adds pose-specific obstacles to the shared test world.
func (w *modeTestWorld) GetBlockStateAt(x, y, z int) int32 {
	if w.water && y >= 64 && y <= 66 {
		return block.DefaultStateID("minecraft:water")
	}
	if w.ceiling != 0 && y == 65 {
		return w.ceiling
	}
	return w.movementTestWorld.GetBlockStateAt(x, y, z)
}

// TestFlightRequestsAreTickOwnedAndModeRestricted rejects forged survival capabilities.
func TestFlightRequestsAreTickOwnedAndModeRestricted(t *testing.T) {
	for _, mode := range []int32{0, 1, 2, 3} {
		player, online, movement := movementTestPlayer(t)
		player.SetGameMode(mode)
		online.GameMode = mode
		var queued []func()
		world := &DefaultWorldAccessor{SubmitTask: func(task func()) bool { queued = append(queued, task); return true }}
		router := NewPacketRouter()
		if err := RegisterPlayHandlers(router, PlayBootstrapConfig{}, movement.players, world); err != nil {
			t.Fatal(err)
		}
		request := &play.PlayerAbilitiesServerbound{Flags: 0xFF}
		if err := router.Handle(movement.session, protocol.StatePlay, request); err != nil {
			t.Fatal(err)
		}
		concrete := movement.session.(*DefaultSession)
		if _, found := concrete.SessionData().Load("player_flying"); found || len(queued) != 1 {
			t.Fatal("flight changed outside the world tick")
		}
		queued[0]()
		value, _ := concrete.SessionData().Load("player_flying")
		if value != (mode == 1 || mode == 3) {
			t.Fatalf("mode %d accepted forged flight: %v", mode, value)
		}
		abilities := playerAbilitiesWithSpeed(mode, 0, false)
		if mode == 3 && abilities.Flags != play.AbilityFlying|play.AbilityAllowFlying|play.AbilityInvulnerable {
			t.Fatal("spectator did not retain mandatory flight")
		}
	}
}

// TestFloatingDetectionUsesWorldTicksAndExemptions covers stationary clients and legitimate support.
func TestFloatingDetectionUsesWorldTicksAndExemptions(t *testing.T) {
	for _, exempt := range []string{"none", "support", "creative", "levitation", "fluid", "teleport"} {
		t.Run(exempt, func(t *testing.T) {
			player, online, command := movementTestPlayer(t)
			world := &modeTestWorld{movementTestWorld: command.world.(*movementTestWorld)}
			player.SetPosition(entity.Vec3{X: .5, Y: 65, Z: .5})
			online.Y = 65
			switch exempt {
			case "support":
				player.SetPosition(entity.Vec3{X: .5, Y: 64, Z: .5})
			case "creative":
				player.SetGameMode(1)
			case "levitation":
				player.Effects().Add(effect.Instance{ID: effectdata.EffectLevitation, Duration: 100})
			case "fluid":
				world.water = true
			case "teleport":
				command.session.(*DefaultSession).setPendingTeleport(99)
			}
			for range 80 {
				world.tick++
				TickPlayerMovement(command.players, world)
			}
			if command.session.Lifecycle() != LifecycleActive {
				t.Fatal("floating grace period was shortened")
			}
			world.tick++
			TickPlayerMovement(command.players, world)
			if closed := command.session.Lifecycle() != LifecycleActive; closed != (exempt == "none") {
				t.Fatalf("exemption %s: closed=%v", exempt, closed)
			}
		})
	}
}

// TestSwimmingMovementUsesFluidPoseAndResetsFallState verifies accepted movement effects.
func TestSwimmingMovementUsesFluidPoseAndResetsFallState(t *testing.T) {
	player, online, command := movementTestPlayer(t)
	world := &modeTestWorld{movementTestWorld: command.world.(*movementTestWorld), water: true}
	command.world = world
	player.SetPosition(entity.Vec3{X: .5, Y: 64.5, Z: .5})
	online.Y, online.Flags = 64.5, 0x08
	player.SetFallDistance(8)
	command.x, command.y = 1.5, 64.2
	command.Execute()
	if player.Position().X != 1.5 || online.Pose != pose.PoseSwimming || player.FallDistance() != 0 || player.Health() != 20 {
		t.Fatal("swimming ignored authoritative pose or accumulated fall damage")
	}
	want := math.Sqrt(1+.3*.3) * .01
	if math.Abs(float64(player.FoodExhaustion())-want) > 1e-6 {
		t.Fatalf("swimming exhaustion=%v want=%v", player.FoodExhaustion(), want)
	}
}

// TestStandingPoseMustFitResidentShapes prevents standing inside a low ceiling.
func TestStandingPoseMustFitResidentShapes(t *testing.T) {
	player, online, command := movementTestPlayer(t)
	world := &modeTestWorld{movementTestWorld: command.world.(*movementTestWorld), ceiling: block.StateID("minecraft:stone_slab", map[string]string{"type": "top", "waterlogged": "false"})}
	online.Pose, online.Flags = pose.PoseCrouching, 0x02
	(&playerModeCommand{session: command.session, players: command.players, world: world, request: play.EntityAction{EntityID: player.ID(), ActionID: 1}}).Execute()
	if online.Pose != pose.PoseCrouching || online.Flags&0x02 != 0 {
		t.Fatal("standing pose intersected the ceiling")
	}
	world.ceiling = block.DefaultStateID("minecraft:stone")
	TickPlayerMovement(command.players, world)
	if online.Pose != pose.PoseSwimming {
		t.Fatal("one-block space did not select a fitting crawl pose")
	}
}

// TestGlideRequiresUsableEquippedWingsAndStopsOnLanding covers the queued glide request.
func TestGlideRequiresUsableEquippedWingsAndStopsOnLanding(t *testing.T) {
	player, online, movement := movementTestPlayer(t)
	online.Windows = inventory.NewWindowManager()
	player.SetPosition(entity.Vec3{X: .5, Y: 70, Z: .5})
	world := movement.world.(*movementTestWorld)
	command := &playerModeCommand{session: movement.session, players: movement.players, world: world, request: play.EntityAction{EntityID: player.ID(), ActionID: 8}}
	command.Execute()
	if online.Flags&0x80 != 0 {
		t.Fatal("player glided without equipped wings")
	}
	wings := inventory.NewSlot(item.IDByName("minecraft:elytra"), 1)
	online.Windows.Inventory().Set(inventory.ArmorStart+1, wings)
	command.Execute()
	if online.Flags&0x80 == 0 || online.Pose != pose.PoseFallFlying {
		t.Fatal("usable wings did not enable glide")
	}
	player.SetOnGround(true)
	TickPlayerMovement(movement.players, world)
	if online.Flags&0x80 != 0 || online.Pose != pose.PoseStanding {
		t.Fatal("landing retained glide")
	}
}
