package worldgen

import (
	"slices"
	"testing"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

// TestStructureCandidateSourcesMatchesExhaustiveScan covers spread modes and negative regions.
func TestStructureCandidateSourcesMatchesExhaustiveScan(t *testing.T) {
	for _, placement := range []structurePlacement{
		{kind: "minecraft:random_spread", spacing: 2, separation: 1, salt: 14357921},
		{kind: "minecraft:random_spread", spacing: 5, separation: 2, salt: 7},
		{kind: "minecraft:random_spread", spacing: 7, separation: 2, salt: 11, triangular: true},
	} {
		resources := &ResourceSet{structureSets: map[string]*structureSet{"test:one": {placement: placement}}}
		planner := &structurePlanner{generator: &TerrainGenerator{seed: 163}, snapshot: &terrainSnapshot{resources: resources}, sets: []string{"test:one"}, reach: 2}
		for _, coordinate := range [][2]int32{{0, 0}, {-11, -15}, {3, 7}, {20, -21}} {
			var want [][2]int32
			for x := coordinate[0] - 2; x <= coordinate[0]+2; x++ {
				for z := coordinate[1] - 2; z <= coordinate[1]+2; z++ {
					cx, cz, _ := placement.potentialChunk(163, x, z)
					if cx == x && cz == z {
						want = append(want, [2]int32{x, z})
					}
				}
			}
			if got := planner.candidateSources(coordinate[0], coordinate[1]); !slices.Equal(got, want) {
				t.Errorf("placement=%+v coordinate=%v candidates=%v exhaustive=%v", placement, coordinate, got, want)
			}
		}
	}
}

// TestSimpleSpreadReferencesVisitOnlyCandidates validates the allocation-free reference walk.
func TestSimpleSpreadReferencesVisitOnlyCandidates(t *testing.T) {
	placement := structurePlacement{kind: "minecraft:random_spread", spacing: 2, separation: 1, salt: 14357921, frequency: 1}
	for _, coordinate := range [][2]int32{{0, 0}, {-11, -15}, {3, 7}, {20, -21}} {
		resources := &ResourceSet{structureSets: map[string]*structureSet{"test:one": {placement: placement}}}
		planner := &structurePlanner{generator: &TerrainGenerator{seed: 163}, snapshot: &terrainSnapshot{resources: resources}, sets: []string{"test:one"}, reach: 2, entries: make(map[[2]int32]*structurePlanEntry)}
		if _, err := planner.references(coordinate[0], coordinate[1]); err != nil {
			t.Fatal(err)
		}
		visited := make([][2]int32, 0, len(planner.entries))
		for key := range planner.entries {
			visited = append(visited, key)
		}
		slices.SortFunc(visited, func(a, b [2]int32) int {
			if a[0] != b[0] {
				return int(a[0] - b[0])
			}
			return int(a[1] - b[1])
		})
		if want := planner.candidateSources(coordinate[0], coordinate[1]); !slices.Equal(visited, want) {
			t.Errorf("coordinate=%v visited=%v want=%v", coordinate, visited, want)
		}
	}
}

// TestStructureReferenceNBTKeepsExistingStart checks negative packed coordinates and copy ownership.
func TestStructureReferenceNBTKeepsExistingStart(t *testing.T) {
	chunk := section.NewChunkWithBounds(-2, 3, 0, 128, 0)
	start := &structureStartPlan{id: "minecraft:nether_fossil", x: -2, z: 3, step: 7, adjustment: terrainAdjustmentThin,
		box:    structureBox{low: blockPosition{-31, 50, 49}, high: blockPosition{-26, 54, 53}},
		pieces: []structurePiecePlan{{box: structureBox{low: blockPosition{-31, 50, 49}, high: blockPosition{-26, 54, 53}}, nbt: structurePieceTag("minecraft:nefos", structureBox{low: blockPosition{-31, 50, 49}, high: blockPosition{-26, 54, 53}})}},
	}
	recordStructurePlans(chunk, []*structureStartPlan{start})
	structures, ok := chunk.PersistentNBT().GetCompound("structures")
	if !ok {
		t.Fatal("missing structure compound")
	}
	owned, ok := structures.GetCompound("starts")
	if !ok {
		t.Fatal("missing starts")
	}
	if _, ok := owned.GetCompound(start.id); !ok {
		t.Fatal("missing fossil start")
	}
	refs, ok := structures.GetCompound("References")
	if !ok {
		t.Fatal("missing references")
	}
	value, _, ok := refs.Get(start.id)
	if !ok {
		t.Fatal("missing fossil references")
	}
	packed, ok := value.([]int64)
	if !ok || len(packed) != 1 || uint64(packed[0]) != uint64(uint32(start.x))|uint64(uint32(start.z))<<32 {
		t.Fatalf("packed references = %v", value)
	}
	start.pieces[0].nbt.PutString("mutated", "yes")
	stored, _ := owned.GetCompound(start.id)
	children, ok := stored.GetList("Children")
	if !ok || children.Len() != 1 {
		t.Fatal("missing owned piece")
	}
	copy, ok := children.Elements()[0].(*nbt.Compound)
	if !ok {
		t.Fatal("stored child has wrong NBT type")
	}
	if _, ok := copy.GetString("mutated"); ok {
		t.Fatal("stored child aliases planner NBT")
	}
}
