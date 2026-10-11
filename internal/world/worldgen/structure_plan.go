package worldgen

import (
	"container/list"
	"fmt"
	"sort"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

const structurePlanCacheLimit = 4096

// structureBox stores inclusive piece bounds independently of mutable chunk storage.
type structureBox struct{ low, high blockPosition }

// intersectsChunk checks horizontal ownership, including negative chunk coordinates.
func (b structureBox) intersectsChunk(x, z int32) bool {
	return b.low.x <= int(x)*16+15 && b.high.x >= int(x)*16 && b.low.z <= int(z)*16+15 && b.high.z >= int(z)*16
}

// inflated extends references around terrain-adjusting structures as in the versioned generator.
func (b structureBox) inflated(radius int) structureBox {
	return structureBox{blockPosition{b.low.x - radius, b.low.y - radius, b.low.z - radius}, blockPosition{b.high.x + radius, b.high.y + radius, b.high.z + radius}}
}

// structurePiecePlan owns transformed geometry and a persistent piece description.
type structurePiecePlan struct {
	box    structureBox
	blocks []templateBlock
	nbt    *nbt.Compound
}

// structureStartPlan is immutable after publication and can be shared by neighboring workers.
type structureStartPlan struct {
	id         string
	x, z       int32
	step       int
	adjustment terrainAdjustment
	box        structureBox
	pieces     []structurePiecePlan
}

// referenceBox includes the twelve-block terrain adaptation margin in structure references.
func (s *structureStartPlan) referenceBox() structureBox {
	if s.adjustment != terrainAdjustmentNone {
		return s.box.inflated(12)
	}
	return s.box
}

// structurePlanEntry coalesces one chunk's starts without depending on terrain or decoration jobs.
type structurePlanEntry struct {
	ready  chan struct{}
	starts []*structureStartPlan
	err    error
	lru    *list.Element
}

// structurePlanner owns snapshot-specific starts and completed-only eviction.
type structurePlanner struct {
	generator *TerrainGenerator
	snapshot  *terrainSnapshot
	sets      []string
	reach     int
	stages    uint16
	mu        sync.Mutex
	entries   map[[2]int32]*structurePlanEntry
	lru       list.List
}

// newStructurePlanner restricts supported sets to biomes reachable by this dimension's source.
func newStructurePlanner(g *TerrainGenerator, snapshot *terrainSnapshot) *structurePlanner {
	if snapshot.resources == nil || len(snapshot.resources.netherFossils) == 0 {
		return nil
	}
	possible := make(map[string]bool)
	for _, id := range possibleBiomeNames(snapshot.biomes) {
		possible[id] = true
	}
	p := &structurePlanner{generator: g, snapshot: snapshot, entries: make(map[[2]int32]*structurePlanEntry)}
	for id, set := range snapshot.resources.structureSets {
		reachable := false
		for _, entry := range set.entries {
			definition := snapshot.resources.netherFossils[entry.id]
			if definition == nil {
				continue
			}
			valid := false
			for biome := range definition.biomeNames {
				if possible[biome] {
					valid = true
					break
				}
			}
			if valid {
				reachable = true
				p.reach = max(p.reach, definition.reach)
				p.stages |= 1 << definition.step
			}
		}
		if reachable {
			p.sets = append(p.sets, id)
		}
	}
	if len(p.sets) == 0 {
		return nil
	}
	sort.Strings(p.sets)
	return p
}

// starts coalesces candidates; planning reads unmodified noise columns and never requests terrain bases.
func (p *structurePlanner) starts(x, z int32) *structurePlanEntry {
	key := [2]int32{x, z}
	p.mu.Lock()
	if entry := p.entries[key]; entry != nil {
		if entry.lru != nil {
			p.lru.MoveToBack(entry.lru)
		}
		p.mu.Unlock()
		<-entry.ready
		return entry
	}
	entry := &structurePlanEntry{ready: make(chan struct{})}
	p.entries[key] = entry
	p.mu.Unlock()
	entry.starts, entry.err = p.plan(x, z)
	p.mu.Lock()
	close(entry.ready)
	entry.lru = p.lru.PushBack(key)
	for p.lru.Len() > structurePlanCacheLimit {
		front := p.lru.Front()
		old := front.Value.([2]int32)
		delete(p.entries, old)
		p.lru.Remove(front)
	}
	p.mu.Unlock()
	return entry
}

// plan preserves weighted retry order for supported structure definitions.
func (p *structurePlanner) plan(x, z int32) ([]*structureStartPlan, error) {
	var starts []*structureStartPlan
	for _, id := range p.sets {
		resources := p.snapshot.resources
		if !resources.isStructureChunk(id, p.generator.seed, x, z) {
			continue
		}
		remaining := append([]structureEntry(nil), resources.structureSets[id].entries...)
		random := worldrandom.NewWorldgen(worldrandom.NewLegacy(0))
		random.SetLargeFeatureSeed(p.generator.seed, x, z)
		for len(remaining) > 0 {
			index := 0
			if len(remaining) > 1 {
				total := 0
				for _, item := range remaining {
					total += item.weight
				}
				value := int(random.NextIntBound(int32(total)))
				for index < len(remaining)-1 && value >= remaining[index].weight {
					value -= remaining[index].weight
					index++
				}
			}
			definition := resources.netherFossils[remaining[index].id]
			if definition != nil {
				start, err := definition.plan(p.generator, p.snapshot, x, z)
				if err != nil {
					return nil, err
				}
				if start != nil {
					starts = append(starts, start)
					break
				}
			}
			remaining = append(remaining[:index], remaining[index+1:]...)
		}
	}
	return starts, nil
}

// references selects only starts whose adjusted bounds intersect this chunk.
func (p *structurePlanner) references(x, z int32) ([]*structureStartPlan, error) {
	if p == nil {
		return nil, nil
	}
	var result []*structureStartPlan
	if len(p.sets) == 1 {
		placement := p.snapshot.resources.structureSets[p.sets[0]].placement
		if placement.kind == "minecraft:random_spread" && placement.spacing-placement.separation == 1 {
			lowX, highX := int(x)-p.reach, int(x)+p.reach
			lowZ, highZ := int(z)-p.reach, int(z)+p.reach
			for regionX := floorDiv(lowX, placement.spacing); regionX <= floorDiv(highX, placement.spacing); regionX++ {
				sourceX := regionX * placement.spacing
				if sourceX < lowX || sourceX > highX {
					continue
				}
				for regionZ := floorDiv(lowZ, placement.spacing); regionZ <= floorDiv(highZ, placement.spacing); regionZ++ {
					sourceZ := regionZ * placement.spacing
					if sourceZ < lowZ || sourceZ > highZ {
						continue
					}
					entry := p.starts(int32(sourceX), int32(sourceZ))
					if entry.err != nil {
						return nil, entry.err
					}
					for _, start := range entry.starts {
						if start.referenceBox().intersectsChunk(x, z) {
							result = append(result, start)
						}
					}
				}
			}
			return result, nil
		}
	}
	for _, key := range p.candidateSources(x, z) {
		entry := p.starts(key[0], key[1])
		if entry.err != nil {
			return nil, entry.err
		}
		for _, start := range entry.starts {
			if start.referenceBox().intersectsChunk(x, z) {
				result = append(result, start)
			}
		}
	}
	return result, nil
}

// candidateSources enumerates only possible starts while retaining ascending chunk order.
func (p *structurePlanner) candidateSources(x, z int32) [][2]int32 {
	candidates := make(map[[2]int32]struct{})
	lowX, highX := int(x)-p.reach, int(x)+p.reach
	lowZ, highZ := int(z)-p.reach, int(z)+p.reach
	for _, id := range p.sets {
		placement := p.snapshot.resources.structureSets[id].placement
		if placement.kind == "minecraft:random_spread" {
			for regionX := floorDiv(lowX, placement.spacing); regionX <= floorDiv(highX, placement.spacing); regionX++ {
				for regionZ := floorDiv(lowZ, placement.spacing); regionZ <= floorDiv(highZ, placement.spacing); regionZ++ {
					candidateX, candidateZ := int32(regionX*placement.spacing), int32(regionZ*placement.spacing)
					if placement.spacing-placement.separation > 1 {
						candidateX, candidateZ, _ = placement.potentialChunk(p.generator.seed, candidateX, candidateZ)
					}
					if int(candidateX) >= lowX && int(candidateX) <= highX && int(candidateZ) >= lowZ && int(candidateZ) <= highZ {
						candidates[[2]int32{candidateX, candidateZ}] = struct{}{}
					}
				}
			}
			continue
		}
		for sourceX := lowX; sourceX <= highX; sourceX++ {
			for sourceZ := lowZ; sourceZ <= highZ; sourceZ++ {
				candidates[[2]int32{int32(sourceX), int32(sourceZ)}] = struct{}{}
			}
		}
	}
	ordered := make([][2]int32, 0, len(candidates))
	for key := range candidates {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i][0] != ordered[j][0] {
			return ordered[i][0] < ordered[j][0]
		}
		return ordered[i][1] < ordered[j][1]
	})
	return ordered
}

// recordStructurePlans preserves starts and references without sharing mutable NBT across chunks.
func recordStructurePlans(c *section.Chunk, starts []*structureStartPlan) {
	if len(starts) == 0 {
		return
	}
	root := c.PersistentNBT()
	if root == nil {
		root = nbt.NewCompound()
	}
	structures, ok := root.GetCompound("structures")
	if !ok {
		structures = nbt.NewCompound()
	}
	owned, ok := structures.GetCompound("starts")
	if !ok {
		owned = nbt.NewCompound()
	}
	references, ok := structures.GetCompound("References")
	if !ok {
		references = nbt.NewCompound()
	}
	packed := make(map[string][]int64)
	for _, start := range starts {
		packed[start.id] = append(packed[start.id], int64(uint64(uint32(start.x))|uint64(uint32(start.z))<<32))
		if start.x == c.X && start.z == c.Z {
			children := nbt.NewList(nbt.TagCompound)
			for _, piece := range start.pieces {
				children.Add(piece.nbt.Clone())
			}
			owned.PutCompound(start.id, nbt.NewCompound().PutString("id", start.id).PutInt("ChunkX", start.x).PutInt("ChunkZ", start.z).PutInt("references", 0).PutList("Children", children))
		}
	}
	for id, values := range packed {
		references.PutLongArray(id, values)
	}
	structures.PutCompound("starts", owned).PutCompound("References", references)
	root.PutCompound("structures", structures)
	c.SetPersistentNBT(root)
}

// placeStructureStage applies canonical immutable geometry before features of the same stage.
func (r *featureRegion) placeStructureStage(c *section.Chunk, step int) {
	if r.snapshot.structures == nil || r.err != nil || r.snapshot.structures.stages&(1<<step) == 0 {
		return
	}
	entry := r.neighbors[4]
	if entry == nil || entry.err != nil {
		return
	}
	var starts []*structureStartPlan
	for _, start := range entry.structures {
		if start.step == step {
			starts = append(starts, start)
		}
	}
	sort.Slice(starts, func(i, j int) bool {
		a, b := starts[i], starts[j]
		if a.id != b.id {
			return a.id < b.id
		}
		if a.x != b.x {
			return a.x < b.x
		}
		return a.z < b.z
	})
	c.SetStructureStep(step)
	for index, start := range starts {
		random := worldrandom.NewWorldgen(worldrandom.NewXoroshiro(0))
		seed := random.SetDecorationSeed(r.generator.seed, r.sourceX*16, r.sourceZ*16)
		random.SetFeatureSeed(seed, int32(index), int32(step))
		for _, piece := range start.pieces {
			for _, item := range piece.blocks {
				p := item.position
				if p.x>>4 != int(c.X) || p.z>>4 != int(c.Z) || !c.FeatureWriteAllowed(p.x, p.y, p.z) {
					continue
				}
				state := item.state
				if generationBlockTraits(getChunkBlock(c, p))&featureWaterTrait != 0 {
					value, _ := block.BlockStateFromID(state)
					if _, ok := value.Property("waterlogged"); ok {
						if wet, err := value.WithProperty("waterlogged", "true"); err == nil {
							state = wet.ID()
						}
					}
				}
				c.RemoveBlockEntity(int32(p.x), int32(p.y), int32(p.z))
				setChunkBlock(c, p, state)
				if item.data != nil {
					data := item.data.Clone().PutInt("x", int32(p.x)).PutInt("y", int32(p.y)).PutInt("z", int32(p.z))
					if templateRandomizableContainer(block.NameFromState(state)) {
						seed := random.NextLong()
						if _, ok := data.GetString("LootTable"); ok {
							data.PutLong("LootTableSeed", seed)
						}
					}
					c.AddBlockEntity(blockentity.NewRawBlockEntity(data))
				}
			}
		}
	}
}

// structurePieceTag serializes the common inclusive bounds and piece metadata.
func structurePieceTag(kind string, box structureBox) *nbt.Compound {
	return nbt.NewCompound().PutString("id", kind).PutIntArray("BB", []int32{int32(box.low.x), int32(box.low.y), int32(box.low.z), int32(box.high.x), int32(box.high.y), int32(box.high.z)}).PutInt("O", -1).PutInt("GD", 0)
}

// structureStep resolves the versioned decoration stage name.
func structureStep(name string) (int, error) {
	for index, value := range []string{"raw_generation", "lakes", "local_modifications", "underground_structures", "surface_structures", "strongholds", "underground_ores", "underground_decoration", "fluid_springs", "vegetal_decoration", "top_layer_modification"} {
		if name == value {
			return index, nil
		}
	}
	return 0, fmt.Errorf("unknown structure generation step %q", name)
}
