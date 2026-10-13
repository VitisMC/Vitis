package worldgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	generatedbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// compileBuriedTreasureBiomes resolves the active biome selection once per reload.
func compileBuriedTreasureBiomes(snapshot *datapack.Snapshot, resources *ResourceSet) error {
	raw := resources.resources["structure"]["minecraft:buried_treasure"]
	if len(raw) == 0 {
		return nil
	}
	var definition struct {
		Type   string          `json:"type"`
		Biomes json.RawMessage `json:"biomes"`
	}
	if err := json.Unmarshal(raw, &definition); err != nil {
		return fmt.Errorf("buried treasure structure: %w", err)
	}
	if definition.Type != "minecraft:buried_treasure" {
		return nil
	}
	if len(definition.Biomes) == 0 {
		return fmt.Errorf("buried treasure: missing biomes")
	}
	var entries []string
	if err := json.Unmarshal(definition.Biomes, &entries); err != nil {
		var single string
		if err := json.Unmarshal(definition.Biomes, &single); err != nil {
			return fmt.Errorf("buried treasure: invalid biomes: %w", err)
		}
		entries = []string{single}
	}
	if len(entries) == 0 {
		return fmt.Errorf("buried treasure: empty biomes")
	}
	allowed := make(map[int32]bool)
	for _, entry := range entries {
		if entry == "" {
			return fmt.Errorf("buried treasure: empty biome reference")
		}
		names := []string{entry}
		if strings.HasPrefix(entry, "#") {
			var err error
			names, err = snapshot.ResolveTag("worldgen/biome", strings.TrimPrefix(entry, "#"))
			if err != nil {
				return fmt.Errorf("buried treasure biomes: %w", err)
			}
		}
		for _, name := range names {
			id := generatedbiome.BiomeIDByName(normalizeResourceID(name))
			if id < 0 {
				return fmt.Errorf("buried treasure: unknown biome %s", name)
			}
			allowed[id] = true
		}
	}
	resources.buriedTreasureBiomes = allowed
	return nil
}

// placeBuriedTreasure places a chest in a qualifying shore chunk.
func placeBuriedTreasure(resources *ResourceSet, result *section.Chunk, biomes biomeSource, biomeHeight int, seed int64, chunkX, chunkZ int32) bool {
	set := resources.structureSets["minecraft:buried_treasures"]
	if set == nil || len(set.entries) != 1 || set.entries[0].id != "minecraft:buried_treasure" || len(resources.buriedTreasureBiomes) == 0 {
		return false
	}
	if !resources.isStructureChunk("minecraft:buried_treasures", seed, chunkX, chunkZ) {
		return false
	}
	if !resources.buriedTreasureBiomes[biomes.sample((int(chunkX)*16+8)>>2, biomeHeight>>2, (int(chunkZ)*16+8)>>2)] {
		return false
	}
	height, ok := result.Heightmap(9, 9, section.HeightmapOceanFloor)
	if !ok {
		recordBuriedTreasureStart(result, chunkX, chunkZ, 90)
		return false
	}
	stone := map[string]bool{"minecraft:sandstone": true, "minecraft:stone": true, "minecraft:andesite": true, "minecraft:granite": true, "minecraft:diorite": true}
	for y := height; y > result.MinY(); y-- {
		below := result.GetBlock(9, y-1, 9)
		if !stone[block.NameFromState(below)] {
			continue
		}
		fill := result.GetBlock(9, y, 9)
		if block.IsAir(fill) || block.IsWater(fill) || block.IsLava(fill) {
			fill = block.DefaultStateID("minecraft:sand")
		}
		for _, direction := range [][3]int{{0, 1, 0}, {0, -1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}} {
			nx, ny, nz := 9+direction[0], y+direction[1], 9+direction[2]
			if nx < 0 || nx > 15 || nz < 0 || nz > 15 || ny < result.MinY() || ny > result.MaxY() {
				continue
			}
			current := result.GetBlock(nx, ny, nz)
			if !block.IsAir(current) && !block.IsWater(current) && !block.IsLava(current) {
				continue
			}
			state := fill
			if direction[1] != 1 {
				under := result.GetBlock(nx, ny-1, nz)
				if block.IsAir(under) || block.IsWater(under) || block.IsLava(under) {
					state = below
				}
			}
			result.SetBlock(nx, ny, nz, state)
		}
		chest := block.DefaultStateID("minecraft:chest")
		if chest < 0 {
			return false
		}
		result.SetBlock(9, y, 9, chest)
		random := worldrandom.NewWorldgen(worldrandom.NewLegacy(0))
		random.SetLargeFeatureSeed(seed, chunkX, chunkZ)
		worldX, worldZ := chunkX*16+9, chunkZ*16+9
		entity := nbt.NewCompound().
			PutString("id", "minecraft:chest").
			PutInt("x", worldX).
			PutInt("y", int32(y)).
			PutInt("z", worldZ).
			PutString("LootTable", "minecraft:chests/buried_treasure").
			PutLong("LootTableSeed", random.NextLong())
		result.AddBlockEntity(blockentity.NewRawBlockEntity(entity))
		recordBuriedTreasureStart(result, chunkX, chunkZ, int32(y))
		return true
	}
	recordBuriedTreasureStart(result, chunkX, chunkZ, 90)
	return false
}

// recordBuriedTreasureStart stores the piece and local reference in chunk NBT.
func recordBuriedTreasureStart(chunk *section.Chunk, chunkX, chunkZ, y int32) {
	worldX, worldZ := chunkX*16+9, chunkZ*16+9
	piece := nbt.NewCompound().
		PutString("id", "minecraft:btp").
		PutIntArray("BB", []int32{worldX, y, worldZ, worldX, y, worldZ}).
		PutInt("O", -1).
		PutInt("GD", 0)
	children := nbt.NewList(nbt.TagCompound)
	children.Add(piece)
	start := nbt.NewCompound().
		PutString("id", "minecraft:buried_treasure").
		PutInt("ChunkX", chunkX).
		PutInt("ChunkZ", chunkZ).
		PutInt("references", 0).
		PutList("Children", children)
	root := chunk.PersistentNBT()
	if root == nil {
		root = nbt.NewCompound()
	}
	structures, _ := root.GetCompound("structures")
	if structures == nil {
		structures = nbt.NewCompound()
	}
	starts, _ := structures.GetCompound("starts")
	if starts == nil {
		starts = nbt.NewCompound()
	}
	starts.PutCompound("minecraft:buried_treasure", start)
	structures.PutCompound("starts", starts)
	references, _ := structures.GetCompound("References")
	if references == nil {
		references = nbt.NewCompound()
	}
	packed := int64(uint64(uint32(chunkX)) | uint64(uint32(chunkZ))<<32)
	references.PutLongArray("minecraft:buried_treasure", []int64{packed})
	structures.PutCompound("References", references)
	root.PutCompound("structures", structures)
	chunk.SetPersistentNBT(root)
}
