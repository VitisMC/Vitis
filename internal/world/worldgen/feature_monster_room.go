package worldgen

import (
	"fmt"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// monsterRoomFeature builds underground rooms and persists their loot and spawners.
type monsterRoomFeature struct {
	air, cobble, mossy, chest, spawner int32
	protected                          map[int32]struct{}
}

// decodeMonsterRoomFeature resolves the resource snapshot's replacement protection.
func decodeMonsterRoomFeature(snapshot *datapack.Snapshot) (configuredFeature, bool, error) {
	if snapshot == nil {
		return nil, true, fmt.Errorf("monster room requires block tags")
	}
	states, err := resolveFeatureBlocks("minecraft:cave_air", "minecraft:cobblestone", "minecraft:mossy_cobblestone", "minecraft:chest", "minecraft:spawner")
	if err != nil {
		return nil, true, err
	}
	names, err := snapshot.ResolveTag("block", "minecraft:features_cannot_replace")
	if err != nil {
		return nil, true, err
	}
	f := &monsterRoomFeature{air: states[0], cobble: states[1], mossy: states[2], chest: states[3], spawner: states[4], protected: make(map[int32]struct{})}
	for _, name := range names {
		info := block.InfoByName(normalizeResourceID(name))
		if info == nil {
			return nil, true, fmt.Errorf("monster room protection references unknown block %s", name)
		}
		for state := info.MinStateID; state <= info.MaxStateID; state++ {
			f.protected[state] = struct{}{}
		}
	}
	return f, true, nil
}

// placeFeature preserves vanilla room validation and the geometry/chest/spawner RNG order.
func (f *monsterRoomFeature) placeFeature(_ *featureSystem, target *section.Chunk, random worldrandom.Source, origin blockPosition) bool {
	radiusX := int(random.NextIntBound(2)) + 2
	radiusZ := int(random.NextIntBound(2)) + 2
	openings := 0
	for dx := -radiusX - 1; dx <= radiusX+1; dx++ {
		for dy := -1; dy <= 4; dy++ {
			for dz := -radiusZ - 1; dz <= radiusZ+1; dz++ {
				position := blockPosition{origin.x + dx, origin.y + dy, origin.z + dz}
				state := getChunkBlock(target, position)
				if (dy == -1 || dy == 4) && !block.IsSolid(state) {
					return false
				}
				if dy == 0 && (absInt(dx) == radiusX+1 || absInt(dz) == radiusZ+1) && block.IsAir(state) && isAirAt(target, blockPosition{position.x, position.y + 1, position.z}) {
					openings++
				}
			}
		}
	}
	if openings < 1 || openings > 5 {
		return false
	}
	for dx := -radiusX - 1; dx <= radiusX+1; dx++ {
		for dy := 3; dy >= -1; dy-- {
			for dz := -radiusZ - 1; dz <= radiusZ+1; dz++ {
				position := blockPosition{origin.x + dx, origin.y + dy, origin.z + dz}
				state := getChunkBlock(target, position)
				if absInt(dx) == radiusX+1 || dy == -1 || absInt(dz) == radiusZ+1 {
					if position.y >= target.MinY() && !block.IsSolid(getChunkBlock(target, blockPosition{position.x, position.y - 1, position.z})) {
						setChunkBlock(target, position, f.air)
					} else if block.IsSolid(state) && block.NameFromState(state) != "minecraft:chest" {
						replacement := f.cobble
						if dy == -1 && random.NextIntBound(4) != 0 {
							replacement = f.mossy
						}
						f.safeSet(target, position, replacement)
					}
				} else if name := block.NameFromState(state); name != "minecraft:chest" && name != "minecraft:spawner" {
					f.safeSet(target, position, f.air)
				}
			}
		}
	}
	for chestIndex := 0; chestIndex < 2; chestIndex++ {
		for attempt := 0; attempt < 3; attempt++ {
			position := blockPosition{origin.x + int(random.NextIntBound(int32(radiusX*2+1))) - radiusX, origin.y, origin.z + int(random.NextIntBound(int32(radiusZ*2+1))) - radiusZ}
			if !isAirAt(target, position) {
				continue
			}
			walls := 0
			for _, offset := range roomHorizontalOffsets {
				if block.IsSolid(getChunkBlock(target, blockPosition{position.x + offset[0], position.y, position.z + offset[1]})) {
					walls++
				}
			}
			if walls != 1 {
				continue
			}
			f.safeSet(target, position, f.orientChest(target, position))
			root := featureBlockEntityNBT("minecraft:chest", position).PutString("LootTable", "minecraft:chests/simple_dungeon").PutLong("LootTableSeed", random.NextLong())
			target.AddBlockEntity(blockentity.NewRawBlockEntity(root))
			break
		}
	}
	f.safeSet(target, origin, f.spawner)
	if getChunkBlock(target, origin) == f.spawner {
		mobs := [...]string{"minecraft:skeleton", "minecraft:zombie", "minecraft:zombie", "minecraft:spider"}
		spawnData := nbt.NewCompound().PutCompound("entity", nbt.NewCompound().PutString("id", mobs[random.NextIntBound(4)]))
		root := featureBlockEntityNBT("minecraft:mob_spawner", origin).
			PutShort("Delay", 20).PutShort("MinSpawnDelay", 200).PutShort("MaxSpawnDelay", 800).
			PutShort("SpawnCount", 4).PutShort("MaxNearbyEntities", 6).PutShort("RequiredPlayerRange", 16).PutShort("SpawnRange", 4).
			PutCompound("SpawnData", spawnData).PutList("SpawnPotentials", nbt.NewList(nbt.TagCompound))
		if previous := target.GetBlockEntity(int32(origin.x), int32(origin.y), int32(origin.z)); previous != nil {
			if persistent, ok := previous.(interface{ PersistentNBT() *nbt.Compound }); ok {
				if stored := persistent.PersistentNBT(); stored != nil {
					root = stored.PutCompound("SpawnData", spawnData)
				}
			}
		}
		target.AddBlockEntity(blockentity.NewRawBlockEntity(root))
	}
	return true
}

// roomHorizontalOffsets follows the horizontal direction order used by room placement.
var roomHorizontalOffsets = [...][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// safeSet preserves blocks excluded by the current datapack's replacement tag.
func (f *monsterRoomFeature) safeSet(target *section.Chunk, position blockPosition, state int32) bool {
	if _, protected := f.protected[getChunkBlock(target, position)]; protected {
		return false
	}
	setChunkBlock(target, position, state)
	return true
}

// orientChest faces a generated chest away from opaque neighboring walls.
func (f *monsterRoomFeature) orientChest(target *section.Chunk, position blockPosition) int32 {
	opaque := func(direction int) bool {
		offset := roomHorizontalOffsets[direction]
		state := getChunkBlock(target, blockPosition{position.x + offset[0], position.y, position.z + offset[1]})
		info := block.Info(block.BlockIDFromState(state))
		return info != nil && info.Solid && !info.Transparent
	}
	wall := -1
	for direction, offset := range roomHorizontalOffsets {
		neighbor := getChunkBlock(target, blockPosition{position.x + offset[0], position.y, position.z + offset[1]})
		if block.NameFromState(neighbor) == "minecraft:chest" {
			return f.chest
		}
		if opaque(direction) {
			if wall >= 0 {
				wall = -1
				break
			}
			wall = direction
		}
	}
	facing := 0
	if wall >= 0 {
		facing = (wall + 2) % 4
	} else {
		if opaque(facing) {
			facing = (facing + 2) % 4
		}
		if opaque(facing) {
			facing = (facing + 1) % 4
		}
		if opaque(facing) {
			facing = (facing + 2) % 4
		}
	}
	return block.StateID("minecraft:chest", map[string]string{"facing": [...]string{"north", "east", "south", "west"}[facing]})
}
