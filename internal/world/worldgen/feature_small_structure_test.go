package worldgen

import (
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// loadSmallFeature compiles the shipped definition against its block tags.
func loadSmallFeature(t *testing.T, name string) configuredFeature {
	t.Helper()
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	raw, ok := manager.Current().Get("data/minecraft/worldgen/configured_feature/" + name + ".json")
	if !ok {
		t.Fatalf("missing configured feature %s", name)
	}
	f, supported, err := decodeConfiguredFeature(manager.Current(), raw)
	if err != nil || !supported {
		t.Fatalf("decode %s: supported=%t error=%v", name, supported, err)
	}
	return f
}

// TestDesertWellGeometryAndArchaeology checks support, geometry, loot, and RNG consumption.
func TestDesertWellGeometryAndArchaeology(t *testing.T) {
	f := loadSmallFeature(t, "desert_well").(*desertWellFeature)
	for _, coords := range [][2]int32{{0, 0}, {-2, -3}} {
		target := section.NewChunkWithBounds(coords[0], coords[1], -64, 384, 0)
		origin := blockPosition{int(coords[0])*16 + 8, 60, int(coords[1])*16 + 8}
		for dx := -3; dx <= 3; dx++ {
			for dz := -3; dz <= 3; dz++ {
				for y := 57; y <= 60; y++ {
					setChunkBlock(target, blockPosition{origin.x + dx, y, origin.z + dz}, f.sand)
				}
			}
		}
		random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		first, second := expected.NextIntBound(5), expected.NextIntBound(5)
		if !f.placeFeature(nil, target, random, origin) {
			t.Fatal("supported well was rejected")
		}
		if random.NextLong() != expected.NextLong() {
			t.Fatal("well consumed extra RNG values")
		}
		cross := [...]blockPosition{origin, {origin.x + 1, origin.y, origin.z}, {origin.x, origin.y, origin.z + 1}, {origin.x - 1, origin.y, origin.z}, {origin.x, origin.y, origin.z - 1}}
		for _, p := range cross {
			if getChunkBlock(target, p) != f.water {
				t.Fatalf("missing basin water at %v", p)
			}
		}
		for dx := -2; dx <= 2; dx++ {
			for dz := -2; dz <= 2; dz++ {
				if absInt(dx) == 2 || absInt(dz) == 2 {
					want := f.stone
					if dx == 0 || dz == 0 {
						want = f.slab
					}
					if getChunkBlock(target, blockPosition{origin.x + dx, 61, origin.z + dz}) != want {
						t.Fatal("incorrect basin rim")
					}
				}
			}
		}
		if getChunkBlock(target, blockPosition{origin.x, 64, origin.z}) != f.stone || getChunkBlock(target, blockPosition{origin.x + 1, 64, origin.z}) != f.slab {
			t.Fatal("incorrect roof")
		}
		if len(target.BlockEntities) != 2 {
			t.Fatalf("brushable entities=%d", len(target.BlockEntities))
		}
		for depth, selection := range []int32{first, second} {
			p := cross[selection]
			p.y -= depth + 1
			if getChunkBlock(target, p) != f.suspicious {
				t.Fatalf("missing suspicious sand at %v", p)
			}
			root := smallFeatureEntityNBT(t, target, p)
			if table, _ := root.GetString("LootTable"); table != "minecraft:archaeology/desert_well" {
				t.Fatalf("unexpected archaeology loot %q", table)
			}
			if seed, _ := root.GetLong("LootTableSeed"); seed != packedBlockPosition(p) {
				t.Fatal("archaeology seed lost position")
			}
		}
	}
}

// TestDesertWellRejectsUnsupportedGround ensures rejected sites remain untouched.
func TestDesertWellRejectsUnsupportedGround(t *testing.T) {
	f := loadSmallFeature(t, "desert_well").(*desertWellFeature)
	for _, ground := range []int32{block.DefaultStateID("minecraft:stone"), f.sand} {
		target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
		origin := blockPosition{8, 60, 8}
		setChunkBlock(target, origin, ground)
		random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		if f.placeFeature(nil, target, random, origin) {
			t.Fatal("unsupported well was accepted")
		}
		if getChunkBlock(target, origin) != ground || len(target.BlockEntities) != 0 || random.NextLong() != expected.NextLong() {
			t.Fatal("rejected well modified the world or RNG")
		}
	}
}

// makeMonsterRoomSite provides rock support with a seed-dependent two-block entrance.
func makeMonsterRoomSite(seed int64) *section.Chunk {
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	stone := block.DefaultStateID("minecraft:stone")
	for x := 3; x <= 13; x++ {
		for z := 3; z <= 13; z++ {
			for y := 27; y <= 35; y++ {
				target.SetBlock(x, y, z, stone)
			}
		}
	}
	random := worldrandom.NewLegacy(seed)
	radiusX := int(random.NextIntBound(2)) + 2
	target.SetBlock(8-radiusX-1, 30, 8, block.DefaultStateID("minecraft:cave_air"))
	target.SetBlock(8-radiusX-1, 31, 8, block.DefaultStateID("minecraft:cave_air"))
	return target
}

// TestMonsterRoomGeometryLootAndSpawner checks complete deterministic rooms and persistent NBT.
func TestMonsterRoomGeometryLootAndSpawner(t *testing.T) {
	f := loadSmallFeature(t, "monster_room").(*monsterRoomFeature)
	chests := 0
	for seed := int64(0); seed < 12; seed++ {
		first, second := makeMonsterRoomSite(seed), makeMonsterRoomSite(seed)
		origin := blockPosition{8, 30, 8}
		for _, target := range []*section.Chunk{first, second} {
			if !f.placeFeature(nil, target, worldrandom.NewLegacy(seed), origin) {
				t.Fatalf("room seed %d was rejected", seed)
			}
		}
		if !reflect.DeepEqual(first.BlockEntities, second.BlockEntities) {
			t.Fatal("block entities are nondeterministic")
		}
		for x := 3; x <= 13; x++ {
			for z := 3; z <= 13; z++ {
				for y := 27; y <= 35; y++ {
					if first.GetBlock(x, y, z) != second.GetBlock(x, y, z) {
						t.Fatalf("nondeterministic room at %d,%d,%d", x, y, z)
					}
				}
			}
		}
		if getChunkBlock(first, origin) != f.spawner || first.GetBlock(8, 34, 8) != block.DefaultStateID("minecraft:stone") || !block.IsAir(first.GetBlock(8, 31, 8)) {
			t.Fatal("incorrect spawner, ceiling, or room interior")
		}
		root := smallFeatureEntityNBT(t, first, origin)
		data, _ := root.GetCompound("SpawnData")
		entity, ok := data.GetCompound("entity")
		if !ok {
			t.Fatal("spawner lost spawn data")
		}
		if mob, _ := entity.GetString("id"); mob != "minecraft:zombie" && mob != "minecraft:skeleton" && mob != "minecraft:spider" {
			t.Fatalf("unexpected room mob %q", mob)
		}
		if delay, kind, _ := root.Get("Delay"); kind != nbt.TagShort || delay != int16(20) {
			t.Fatal("spawner defaults lost")
		}
		for _, be := range first.BlockEntities {
			if be.TypeName() != "minecraft:chest" {
				continue
			}
			chests++
			x, y, z := be.Position()
			root := smallFeatureEntityNBT(t, first, blockPosition{int(x), int(y), int(z)})
			if table, _ := root.GetString("LootTable"); table != "minecraft:chests/simple_dungeon" {
				t.Fatal("dungeon chest lost loot table")
			}
			if block.NameFromState(first.GetBlock(int(x), int(y), int(z))) != "minecraft:chest" {
				t.Fatal("orphan dungeon chest NBT")
			}
		}
	}
	if chests < 1 {
		t.Fatal("no dungeon chests generated")
	}
}

// TestMonsterRoomValidationAndProtection covers absent support, entrances, and protected blocks.
func TestMonsterRoomValidationAndProtection(t *testing.T) {
	f := loadSmallFeature(t, "monster_room").(*monsterRoomFeature)
	origin := blockPosition{8, 30, 8}
	for _, p := range []blockPosition{{8, 29, 8}, {8, 34, 8}} {
		target := makeMonsterRoomSite(42)
		setChunkBlock(target, p, 0)
		random, expected := worldrandom.NewLegacy(42), worldrandom.NewLegacy(42)
		expected.NextIntBound(2)
		expected.NextIntBound(2)
		if f.placeFeature(nil, target, random, origin) || len(target.BlockEntities) != 0 || random.NextLong() != expected.NextLong() {
			t.Fatal("invalid room accepted or modified RNG")
		}
	}
	target := makeMonsterRoomSite(42)
	protected := block.DefaultStateID("minecraft:reinforced_deepslate")
	target.SetBlock(9, 31, 8, protected)
	if !f.placeFeature(nil, target, worldrandom.NewLegacy(42), origin) || target.GetBlock(9, 31, 8) != protected {
		t.Fatal("protected room interior overwritten")
	}
	if f.safeSet(target, origin, 0) {
		t.Fatal("spawner must be protected")
	}
}

// smallFeatureEntityNBT verifies named NBT encoding of one generated block entity.
func smallFeatureEntityNBT(t *testing.T, target *section.Chunk, position blockPosition) *nbt.Compound {
	t.Helper()
	be := target.GetBlockEntity(int32(position.x), int32(position.y), int32(position.z))
	if be == nil {
		t.Fatalf("missing block entity at %v", position)
	}
	persistent, ok := be.(interface{ PersistentNBT() *nbt.Compound })
	if !ok || persistent.PersistentNBT() == nil {
		t.Fatal("missing persistent block entity NBT")
	}
	encoder := nbt.NewEncoder(256)
	if err := encoder.WriteNamedRootCompound("", persistent.PersistentNBT()); err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(encoder.Bytes()).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
