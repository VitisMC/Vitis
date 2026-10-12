package session

import (
	"context"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
)

// oceanSpawnFixture builds an ocean over a dry cave within the loaded search area.
func oceanSpawnFixture(t *testing.T) (*PlayerManager, *OnlinePlayer, *DefaultWorldAccessor) {
	t.Helper()
	players, online, world, _ := lifecycleFixture(t)
	world.SetBlock(1, 64, 1, 0)
	world.SetBlock(1, 64, 2, 0)
	stone, water := block.DefaultStateID("minecraft:stone"), block.DefaultStateID("minecraft:water")
	for x := 1; x <= 9; x++ {
		for z := 1; z <= 9; z++ {
			world.SetBlock(x, 0, z, stone)
			world.SetBlock(x, 40, z, stone)
			for y := 41; y <= 62; y++ {
				world.SetBlock(x, y, z, water)
			}
		}
	}
	return players, online, world
}

// TestWorldSpawnStaysAtOceanSurface rejects the dry cave beneath the seabed.
func TestWorldSpawnStaysAtOceanSurface(t *testing.T) {
	_, _, world := oceanSpawnFixture(t)
	x, y, z, valid := safeWorldRespawn(world, 5, 65, 5)
	if !valid || x != 5.5 || y != 63 || z != 5.5 || !spawnPositionClear(world, x, y, z) {
		t.Fatalf("ocean spawn = %v,%v,%v (%v)", x, y, z, valid)
	}
	if !block.IsWater(world.GetBlock(5, 62, 5)) {
		t.Fatal("spawn search changed the ocean surface")
	}
}

// TestWorldSpawnPrefersNearbyDryShore checks land before the ocean fallback.
func TestWorldSpawnPrefersNearbyDryShore(t *testing.T) {
	_, _, world := oceanSpawnFixture(t)
	world.SetBlock(7, 62, 5, block.DefaultStateID("minecraft:stone"))
	x, y, z, valid := safeWorldRespawn(world, 5, 65, 5)
	if !valid || x != 7.5 || y != 63 || z != 5.5 {
		t.Fatalf("shore spawn = %v,%v,%v (%v)", x, y, z, valid)
	}
}

// TestOceanPlantsAndWaterloggedBlocksKeepSurfaceSpawn covers non-pure fluid states.
func TestOceanPlantsAndWaterloggedBlocksKeepSurfaceSpawn(t *testing.T) {
	for _, test := range []struct {
		name       string
		properties map[string]string
	}{
		{"kelp", nil}, {"kelp_plant", nil}, {"seagrass", nil},
		{"tall_seagrass", map[string]string{"half": "upper"}},
		{"bubble_column", nil},
		{"stone_slab", map[string]string{"type": "bottom", "waterlogged": "true"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, world := oceanSpawnFixture(t)
			state := block.StateID("minecraft:"+test.name, test.properties)
			if state < 0 {
				t.Fatal("aquatic block state missing")
			}
			world.SetBlock(5, 62, 5, state)
			x, y, z, valid := safeWorldRespawn(world, 5, 65, 5)
			if !valid || x != 5.5 || y != 63 || z != 5.5 {
				t.Fatalf("aquatic spawn = %v,%v,%v (%v)", x, y, z, valid)
			}
		})
	}
}

// TestOceanLoginAndRespawnUseSurface exercises both authoritative transition paths.
func TestOceanLoginAndRespawnUseSurface(t *testing.T) {
	players, online, world := oceanSpawnFixture(t)
	result := make(chan spawnPreparation, 1)
	(&spawnPlayerCommand{ctx: context.Background(), world: world, position: entity.Vec3{X: 5.5, Y: 65, Z: 5.5}, result: result}).Execute()
	if resolved := <-result; resolved.err != nil || resolved.position.Y != 63 {
		t.Fatalf("new-player ocean spawn = %+v", resolved)
	}
	player := online.Session.Player().(*entity.Player)
	player.Living().SetHealth(0)
	player.Living().MarkDeathHandled()
	command := &respawnPlayerCommand{session: online.Session, players: players, world: world, config: PlayBootstrapConfig{SpawnX: 5.5, SpawnY: 65, SpawnZ: 5.5}}
	command.Execute()
	if player.Living().IsDead() || player.Position().Y != 63 {
		t.Fatalf("ocean respawn failed: %+v", player.Position())
	}
	position := player.Position()
	command.Execute()
	if player.Position() != position {
		t.Fatal("repeated respawn changed the surface position")
	}
}

// TestUndergroundSavedAndForcedSpawnsRemainLocal preserves explicit cave positions.
func TestUndergroundSavedAndForcedSpawnsRemainLocal(t *testing.T) {
	_, online, world := oceanSpawnFixture(t)
	position := entity.Vec3{X: 5.5, Y: 1, Z: 5.5}
	result := make(chan spawnPreparation, 1)
	(&spawnPlayerCommand{ctx: context.Background(), world: world, position: position, saved: true, result: result}).Execute()
	if resolved := <-result; resolved.err != nil || resolved.position != position {
		t.Fatalf("saved cave position changed: %+v", resolved)
	}
	online.SpawnX, online.SpawnY, online.SpawnZ = 5, 1, 5
	online.SpawnForced = true
	x, y, z, valid := playerRespawnPoint(world, online)
	if !valid || x != position.X || y != position.Y || z != position.Z {
		t.Fatalf("forced cave spawn = %v,%v,%v (%v)", x, y, z, valid)
	}
}

// TestWorldSpawnNeverDescendsThroughLava rejects hazardous surface fallback.
func TestWorldSpawnNeverDescendsThroughLava(t *testing.T) {
	_, _, world := oceanSpawnFixture(t)
	lava := block.DefaultStateID("minecraft:lava")
	for x := 1; x <= 9; x++ {
		for z := 1; z <= 9; z++ {
			world.SetBlock(x, 62, z, lava)
		}
	}
	if _, _, _, valid := safeWorldRespawn(world, 5, 65, 5); valid {
		t.Fatal("lava surface led to a cave or unsafe fallback")
	}
}
