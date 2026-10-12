package session

import (
	"context"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/world/chunk"
)

// TestStandingSpawnUsesCollisionSurfaces checks fractional floors and clearance.
func TestStandingSpawnUsesCollisionSurfaces(t *testing.T) {
	for _, test := range []struct {
		name       string
		block      string
		properties map[string]string
		wantY      float64
	}{
		{"bottom_slab", "stone_slab", map[string]string{"type": "bottom", "waterlogged": "false"}, 64.5},
		{"top_slab", "stone_slab", map[string]string{"type": "top", "waterlogged": "false"}, 65},
		{"stairs", "stone_stairs", map[string]string{"half": "bottom", "facing": "north", "shape": "straight", "waterlogged": "false"}, 65},
		{"snow", "snow", map[string]string{"layers": "4"}, 64.375},
		{"fence", "oak_fence", map[string]string{"north": "false", "south": "false", "east": "false", "west": "false", "waterlogged": "false"}, 65.5},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, world, _ := lifecycleFixture(t)
			world.SetBlock(5, 64, 5, block.StateID("minecraft:"+test.block, test.properties))
			x, y, z, ok := safeWorldRespawn(world, 5, 65, 5)
			if !ok || x != 5.5 || y != test.wantY || z != 5.5 {
				t.Fatalf("spawn = %v,%v,%v (%v), want Y=%v", x, y, z, ok, test.wantY)
			}
			box := physics.PlayerDimensions().MakeBoundingBox(x, y, z)
			if physics.IsCollidingAt(spawnBlockAccess{world}, box) || !physics.CheckOnGround(spawnBlockAccess{world}, box) {
				t.Fatal("resolved spawn penetrates a shape or lacks support")
			}
		})
	}
}

// TestStandingSpawnRejectsLowCeilingAndHazards checks the complete player height.
func TestStandingSpawnRejectsLowCeilingAndHazards(t *testing.T) {
	_, _, world, _ := lifecycleFixture(t)
	world.SetBlock(5, 64, 5, block.StateID("minecraft:stone_slab", map[string]string{"type": "bottom", "waterlogged": "false"}))
	world.SetBlock(5, 66, 5, block.StateID("minecraft:stone", nil))
	if _, _, _, ok := standingSpawnAt(world, 5, 65, 5); ok {
		t.Fatal("1.5-block clearance accepted a standing player")
	}
	world.SetBlock(5, 66, 5, 0)
	for _, name := range []string{"lava", "water", "fire", "sweet_berry_bush", "powder_snow", "wither_rose"} {
		world.SetBlock(5, 65, 5, block.StateID("minecraft:"+name, nil))
		if _, _, _, ok := standingSpawnAt(world, 5, 65, 5); ok {
			t.Fatalf("spawn touching %s was accepted", name)
		}
	}
	world.SetBlock(5, 65, 5, 0)
	world.SetBlock(5, 64, 5, block.StateID("minecraft:magma_block", nil))
	if _, _, _, ok := standingSpawnAt(world, 5, 65, 5); ok {
		t.Fatal("damaging support was accepted")
	}
}

// TestSavedSpawnRetainsFractionalPosition checks the login command's shape query.
func TestSavedSpawnRetainsFractionalPosition(t *testing.T) {
	_, _, world, _ := lifecycleFixture(t)
	world.SetBlock(5, 64, 5, block.StateID("minecraft:stone_slab", map[string]string{"type": "bottom", "waterlogged": "false"}))
	position := entity.Vec3{X: 5.4, Y: 64.5, Z: 5.6}
	result := make(chan spawnPreparation, 1)
	(&spawnPlayerCommand{ctx: context.Background(), world: world, position: position, saved: true, result: result}).Execute()
	resolved := <-result
	if resolved.err != nil || resolved.position != position {
		t.Fatalf("saved position changed: %+v", resolved)
	}
	world.SetBlock(5, 64, 5, block.StateID("minecraft:oak_fence", map[string]string{
		"north": "false", "south": "false", "east": "false", "west": "false", "waterlogged": "false",
	}))
	position.Y = 65.1
	(&spawnPlayerCommand{ctx: context.Background(), world: world, position: position, saved: true, result: result}).Execute()
	resolved = <-result
	if resolved.err != nil || resolved.position.Y != 65.5 {
		t.Fatalf("saved position inside a tall shape was retained: %+v", resolved)
	}
}

// TestBedRespawnPreservesFractionalSurface exercises the actual lifecycle handler.
func TestBedRespawnPreservesFractionalSurface(t *testing.T) {
	pm, online, world, _ := lifecycleFixture(t)
	online.HasSpawnPoint = true
	online.SpawnX, online.SpawnY, online.SpawnZ = 1, 64, 1
	player := online.Session.Player().(*entity.Player)
	player.Living().SetHealth(0)
	if err := handleRespawn(online.Session, PlayBootstrapConfig{SpawnX: 1.5, SpawnY: 65, SpawnZ: 1.5}, pm, world); err != nil {
		t.Fatal(err)
	}
	if position := player.Position(); position.Y != 64.5625 || !spawnPositionClear(world, position.X, position.Y, position.Z) {
		t.Fatalf("bed surface was rounded or obstructed: %+v", position)
	}
}

// TestSpawnRequiresCollisionNeighborChunks covers positive and negative borders.
func TestSpawnRequiresCollisionNeighborChunks(t *testing.T) {
	storage := chunk.NewStorage(1)
	storage.Set(chunk.New(0, 0))
	manager := chunk.NewManager(chunk.ManagerConfig{Storage: storage})
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	world := &DefaultWorldAccessor{Chunks: manager}
	for _, x := range []float64{0.3, 15.7} {
		if spawnPositionClear(world, x, 65, 5.5) {
			t.Fatalf("missing collision neighbor treated as air at X=%v", x)
		}
	}
	storage.Set(chunk.New(-1, 0))
	storage.Set(chunk.New(1, 0))
	for _, x := range []float64{0.3, 15.7, -0.3} {
		if !spawnPositionClear(world, x, 65, 5.5) {
			t.Fatalf("loaded boundary position was rejected at X=%v", x)
		}
	}
}
