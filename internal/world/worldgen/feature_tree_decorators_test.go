package worldgen

import (
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// TestUniqueTreePositionsPreservesInput checks ordering, duplicates, and source ownership.
func TestUniqueTreePositionsPreservesInput(t *testing.T) {
	positions := []blockPosition{{x: 2, y: 5, z: 1}, {x: -1, y: 3, z: 7}, {x: 2, y: 5, z: 1}, {x: 0, y: 3, z: -1}}
	original := append([]blockPosition(nil), positions...)
	want := []blockPosition{{x: -1, y: 3, z: 7}, {x: 0, y: 3, z: -1}, {x: 2, y: 5, z: 1}}
	if got := uniqueTreePositions(positions); !reflect.DeepEqual(got, want) {
		t.Fatalf("unique positions = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(positions, original) {
		t.Fatalf("source positions were changed: %v", positions)
	}
}

// TestTreeDecoratorDecoding rejects malformed settings and unsupported kinds explicitly.
func TestTreeDecoratorDecoding(t *testing.T) {
	for _, raw := range []string{
		`{"type":"minecraft:beehive","probability":2}`,
		`{"type":"minecraft:attached_to_leaves","directions":["down"],"required_empty_blocks":0}`,
		`{"type":"minecraft:attached_to_leaves","directions":["sideways"],"required_empty_blocks":1}`,
		`{"type":"minecraft:attached_to_leaves","directions":[],"required_empty_blocks":1}`,
		`{"type":"minecraft:alter_ground","provider":{}}`,
	} {
		if _, _, err := decodeTreeDecorator(nil, []byte(raw)); err == nil {
			t.Fatalf("invalid decorator accepted: %s", raw)
		}
	}
	if _, supported, err := decodeTreeDecorator(nil, []byte(`{"type":"custom:unknown"}`)); err != nil || supported {
		t.Fatalf("unsupported decorator: supported=%t error=%v", supported, err)
	}
}

// TestAlterGroundProducesPodzol preserves rock while preparing old-growth taiga soil.
func TestAlterGroundProducesPodzol(t *testing.T) {
	f := loadSmallFeature(t, "mega_spruce").(*treeFeature)
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	grass, stone, podzol := block.DefaultStateID("minecraft:grass_block"), block.DefaultStateID("minecraft:stone"), block.DefaultStateID("minecraft:podzol")
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			target.SetBlock(x, 64, z, grass)
		}
	}
	target.SetBlock(6, 64, 6, stone)
	f.placeDecorators(nil, target, worldrandom.NewLegacy(42), []blockPosition{{8, 65, 8}, {8, 66, 8}}, nil)
	count := 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			if target.GetBlock(x, 64, z) == podzol {
				count++
			}
		}
	}
	if count < 50 || target.GetBlock(6, 64, 6) != stone || target.GetBlock(8, 64, 8) != podzol {
		t.Fatalf("incorrect podzol patch: area=%d", count)
	}
}

// TestLeafAttachmentsRespectClearanceAndExclusion checks hanging mangrove propagules.
func TestLeafAttachmentsRespectClearanceAndExclusion(t *testing.T) {
	f := loadSmallFeature(t, "mangrove").(*treeFeature)
	decorator := f.decorators[1]
	decorator.probability = 1
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	leaves := []blockPosition{{4, 70, 8}, {5, 70, 8}, {8, 70, 8}, {9, 70, 8}, {12, 70, 8}}
	for _, p := range leaves {
		setTreeBlock(target, p, block.DefaultStateID("minecraft:mangrove_leaves"))
	}
	target.SetBlock(12, 68, 8, block.DefaultStateID("minecraft:stone"))
	f.placeAttachedToLeaves(nil, target, worldrandom.NewLegacy(42), decorator, leaves)
	var placed []int
	for _, p := range leaves {
		state := target.GetBlock(p.x, 69, p.z)
		if block.NameFromState(state) != "minecraft:mangrove_propagule" {
			continue
		}
		propagule, err := block.BlockStateFromID(state)
		if err != nil {
			t.Fatal(err)
		}
		if hanging, _ := propagule.Property("hanging"); hanging != "true" {
			t.Fatal("propagule is not hanging")
		}
		for _, x := range placed {
			if absInt(x-p.x) <= 1 {
				t.Fatal("leaf exclusion radius ignored")
			}
		}
		placed = append(placed, p.x)
	}
	if len(placed) != 2 || !block.IsAir(target.GetBlock(12, 69, 8)) {
		t.Fatalf("propagules=%v; clearance ignored", placed)
	}
}

// TestNaturalCreakingHeartRequiresSixLogNeighbors checks enclosure and persistent identity.
func TestNaturalCreakingHeartRequiresSixLogNeighbors(t *testing.T) {
	f := loadSmallFeature(t, "pale_oak").(*treeFeature)
	center := blockPosition{8, 70, 8}
	for _, enclosed := range []bool{false, true} {
		target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
		log := block.DefaultStateID("minecraft:pale_oak_log")
		setTreeBlock(target, center, log)
		for _, offset := range [...][3]int{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}} {
			if !enclosed && offset[1] == 1 {
				continue
			}
			setTreeBlock(target, blockPosition{center.x + offset[0], center.y + offset[1], center.z + offset[2]}, log)
		}
		f.placeCreakingHeart(target, worldrandom.NewLegacy(42), 1, []blockPosition{center})
		if !enclosed {
			if getTreeBlock(target, center) != log || len(target.BlockEntities) != 0 {
				t.Fatal("exposed heart generated")
			}
			continue
		}
		state, err := block.BlockStateFromID(getTreeBlock(target, center))
		if err != nil {
			t.Fatal(err)
		}
		if state.Name() != "minecraft:creaking_heart" {
			t.Fatal("heart missing")
		}
		for _, name := range []string{"active", "natural"} {
			if value, _ := state.Property(name); value != "true" {
				t.Fatalf("heart %s=%s", name, value)
			}
		}
		if id, _ := smallFeatureEntityNBT(t, target, center).GetString("id"); id != "minecraft:creaking_heart" {
			t.Fatal("heart entity missing")
		}
	}
}

// TestBeeNestDecorationPersistsOccupants checks orientation and the 1.21.4 occupant schema.
func TestBeeNestDecorationPersistsOccupants(t *testing.T) {
	f := loadSmallFeature(t, "oak").(*treeFeature)
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	logs := []blockPosition{{8, 65, 8}, {8, 66, 8}, {8, 67, 8}, {8, 68, 8}}
	for _, p := range logs {
		setTreeBlock(target, p, block.DefaultStateID("minecraft:oak_log"))
	}
	f.placeBeeNest(target, worldrandom.NewLegacy(42), 1, logs, []blockPosition{{8, 68, 8}})
	if len(target.BlockEntities) != 1 {
		t.Fatal("bee nest entity missing")
	}
	for _, be := range target.BlockEntities {
		x, y, z := be.Position()
		state, _ := block.BlockStateFromID(target.GetBlock(int(x), int(y), int(z)))
		if facing, _ := state.Property("facing"); facing != "south" || y != 67 {
			t.Fatal("incorrect nest placement")
		}
		bees, ok := smallFeatureEntityNBT(t, target, blockPosition{int(x), int(y), int(z)}).GetList("bees")
		if !ok || bees.Len() < 2 || bees.Len() > 3 {
			t.Fatal("nest occupants missing")
		}
		for _, value := range bees.Elements() {
			entry := value.(*nbt.Compound)
			data, _ := entry.GetCompound("entity_data")
			if id, _ := data.GetString("id"); id != "minecraft:bee" {
				t.Fatal("invalid nest occupant")
			}
			if duration, _ := entry.GetInt("min_ticks_in_hive"); duration != 600 {
				t.Fatal("invalid nest duration")
			}
			if ticks, _ := entry.GetInt("ticks_in_hive"); ticks < 0 || ticks >= 599 {
				t.Fatal("invalid nest age")
			}
		}
	}
}

// TestPaleMossDecorationAddsGroundAndHangers checks that both nested and direct decorations run.
func TestPaleMossDecorationAddsGroundAndHangers(t *testing.T) {
	f := loadSmallFeature(t, "pale_oak").(*treeFeature)
	decorator := f.decorators[0]
	decorator.groundChance, decorator.leavesChance, decorator.trunkChance = 1, 1, 1
	target := section.NewChunkWithBounds(0, 0, -64, 384, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			target.SetBlock(x, 64, z, block.DefaultStateID("minecraft:dirt"))
		}
	}
	logs, leaves := []blockPosition{{8, 65, 8}, {8, 70, 8}}, []blockPosition{{5, 72, 8}}
	for _, p := range logs {
		setTreeBlock(target, p, block.DefaultStateID("minecraft:pale_oak_log"))
	}
	for _, p := range leaves {
		setTreeBlock(target, p, block.DefaultStateID("minecraft:pale_oak_leaves"))
	}
	system := &featureSystem{resources: &featureSystemResources{configured: map[string]configuredFeature{"minecraft:pale_moss_patch": loadSmallFeature(t, "pale_moss_patch")}}}
	f.placePaleMoss(system, target, worldrandom.NewLegacy(42), decorator, logs, leaves)
	if block.NameFromState(target.GetBlock(5, 71, 8)) != "minecraft:pale_hanging_moss" || block.NameFromState(target.GetBlock(8, 69, 8)) != "minecraft:pale_hanging_moss" {
		t.Fatal("moss hangers missing")
	}
	ground := 0
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			if block.NameFromState(target.GetBlock(x, 64, z)) == "minecraft:pale_moss_block" {
				ground++
			}
		}
	}
	if ground == 0 {
		t.Fatal("nested pale moss ground patch did not run")
	}
}

// TestFeatureBorderEntitiesFollowAcceptedBlocks prevents orphan NBT and shared mutable roots.
func TestFeatureBorderEntitiesFollowAcceptedBlocks(t *testing.T) {
	position := section.BorderPosition{X: 16, Y: 70, Z: 8}
	state := block.StateID("minecraft:bee_nest", map[string]string{"facing": "south"})
	root := featureBlockEntityNBT("minecraft:beehive", blockPosition{16, 70, 8}).PutList("bees", nbt.NewList(nbt.TagCompound))
	border := map[section.BorderPosition]int32{position: state}
	entities := map[section.BorderPosition]*nbt.Compound{position: root}
	for _, accepted := range []bool{false, true} {
		target := section.NewChunkWithBounds(1, 0, -64, 384, 0)
		if !accepted {
			target.SetBlock(16, 70, 8, block.DefaultStateID("minecraft:stone"))
		}
		mergeFeatureBorders(target, border, entities)
		if !accepted {
			if len(target.BlockEntities) != 0 {
				t.Fatal("rejected border has orphan NBT")
			}
			continue
		}
		root.PutString("test_mutation", "source")
		stored := smallFeatureEntityNBT(t, target, blockPosition{16, 70, 8})
		if _, _, present := stored.Get("test_mutation"); present {
			t.Fatal("border entity root is shared")
		}
		if target.GetBlock(16, 70, 8) != state {
			t.Fatal("border block missing")
		}
	}
}
