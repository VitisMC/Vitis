package spawning

import (
	"math"
	"math/rand"

	"github.com/vitismc/vitis/internal/block"
	genbiome "github.com/vitismc/vitis/internal/data/generated/biome"
	"github.com/vitismc/vitis/internal/entity"
)

const (
	defaultIntervalTicks       = 20
	defaultAttemptsPerCategory = 3
	defaultMinSpawnDistance    = 24.0
	defaultMaxSpawnDistance    = 128.0
	defaultRandomDespawnRange  = 32.0
	defaultHardDespawnRange    = 128.0
	defaultRandomDespawnChance = 800
	seaLevel                   = 63
)

// MobCap defines the default maximum number of mobs per spawn category.
var MobCap = map[entity.MobCategory]int{
	entity.MobCategoryHostile:       70,
	entity.MobCategoryPassive:       10,
	entity.MobCategoryAmbient:       15,
	entity.MobCategoryWaterCreature: 5,
}

var spawnCategoryOrder = []entity.MobCategory{
	entity.MobCategoryHostile,
	entity.MobCategoryPassive,
	entity.MobCategoryAmbient,
	entity.MobCategoryWaterCreature,
}

// Config controls natural mob spawning behavior.
type Config struct {
	Seed                  int64
	IntervalTicks         int
	AttemptsPerCategory   int
	MinSpawnDistance      float64
	MaxSpawnDistance      float64
	RandomDespawnDistance float64
	HardDespawnDistance   float64
	RandomDespawnChance   int
	MobCaps               map[entity.MobCategory]int
	SpawnTable            []SpawnEntry
}

// SpawnEntry defines a mob type eligible for natural spawning.
type SpawnEntry struct {
	TypeName        string
	Weight          int
	MinGroup        int
	MaxGroup        int
	Category        entity.MobCategory
	BiomeCategories []string
}

// DefaultOverworldSpawns provides the default overworld spawn table.
var DefaultOverworldSpawns = []SpawnEntry{
	{TypeName: "zombie", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "skeleton", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "creeper", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "spider", Weight: 100, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "enderman", Weight: 10, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "witch", Weight: 5, MinGroup: 1, MaxGroup: 1, Category: entity.MobCategoryHostile},
	{TypeName: "slime", Weight: 10, MinGroup: 1, MaxGroup: 4, Category: entity.MobCategoryHostile},
	{TypeName: "cow", Weight: 8, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive, BiomeCategories: landBiomeCategories()},
	{TypeName: "pig", Weight: 10, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive, BiomeCategories: landBiomeCategories()},
	{TypeName: "sheep", Weight: 12, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive, BiomeCategories: landBiomeCategories()},
	{TypeName: "chicken", Weight: 10, MinGroup: 2, MaxGroup: 4, Category: entity.MobCategoryPassive, BiomeCategories: landBiomeCategories()},
	{TypeName: "bat", Weight: 10, MinGroup: 1, MaxGroup: 8, Category: entity.MobCategoryAmbient},
}

// PlayerInfo represents a player eligible for spawning calculations.
type PlayerInfo struct {
	X, Y, Z  float64
	GameMode int32
	Dead     bool
}

// WorldAccess provides world state required by natural spawning rules.
type WorldAccess interface {
	GetBlock(x, y, z int) int32
	GetBiome(x, y, z int) int32
	GetBlockLight(x, y, z int) uint8
	GetSkyLight(x, y, z int) uint8
	IsChunkLoaded(x, z int32) bool
	MinY() int
	MaxY() int
	TimeOfDay() int64
}

// SpawnRequest describes a mob spawn the world should execute.
type SpawnRequest struct {
	TypeName string
	X, Y, Z  float64
}

// SpawnResult contains world mutations selected by the spawner.
type SpawnResult struct {
	Requests   []SpawnRequest
	DespawnIDs []int32
}

// Spawner handles natural mob spawning and despawn decisions.
type Spawner struct {
	spawnTable []SpawnEntry
	mobCaps    map[entity.MobCategory]int

	spawnCounter          int
	interval              int
	attemptsPerCategory   int
	minSpawnDistance      float64
	minSpawnDistanceSq    float64
	maxSpawnDistance      float64
	maxSpawnDistanceSq    float64
	randomDespawnDistance float64
	randomDespawnSq       float64
	hardDespawnSq         float64
	randomDespawnChance   int

	rng *rand.Rand
}

// NewSpawner creates a mob spawner using default-compatible settings.
func NewSpawner(table []SpawnEntry, interval int) *Spawner {
	return NewSpawnerWithConfig(Config{SpawnTable: table, IntervalTicks: interval})
}

// NewSpawnerWithConfig creates a mob spawner with explicit production settings.
func NewSpawnerWithConfig(config Config) *Spawner {
	interval := config.IntervalTicks
	if interval <= 0 {
		interval = defaultIntervalTicks
	}

	attempts := config.AttemptsPerCategory
	if attempts <= 0 {
		attempts = defaultAttemptsPerCategory
	}

	minDistance := config.MinSpawnDistance
	if minDistance <= 0 {
		minDistance = defaultMinSpawnDistance
	}

	maxDistance := config.MaxSpawnDistance
	if maxDistance <= minDistance {
		maxDistance = defaultMaxSpawnDistance
	}

	randomDespawnDistance := config.RandomDespawnDistance
	if randomDespawnDistance <= 0 {
		randomDespawnDistance = defaultRandomDespawnRange
	}

	hardDespawnDistance := config.HardDespawnDistance
	if hardDespawnDistance <= randomDespawnDistance {
		hardDespawnDistance = defaultHardDespawnRange
	}

	randomDespawnChance := config.RandomDespawnChance
	if randomDespawnChance <= 0 {
		randomDespawnChance = defaultRandomDespawnChance
	}

	table := config.SpawnTable
	if len(table) == 0 {
		table = DefaultOverworldSpawns
	}

	return &Spawner{
		spawnTable:            append([]SpawnEntry(nil), table...),
		mobCaps:               copyMobCaps(config.MobCaps),
		interval:              interval,
		attemptsPerCategory:   attempts,
		minSpawnDistance:      minDistance,
		minSpawnDistanceSq:    minDistance * minDistance,
		maxSpawnDistance:      maxDistance,
		maxSpawnDistanceSq:    maxDistance * maxDistance,
		randomDespawnDistance: randomDespawnDistance,
		randomDespawnSq:       randomDespawnDistance * randomDespawnDistance,
		hardDespawnSq:         hardDespawnDistance * hardDespawnDistance,
		randomDespawnChance:   randomDespawnChance,
		rng:                   rand.New(rand.NewSource(config.Seed)),
	}
}

// Tick runs one natural spawn cycle and returns world mutations to apply.
func (s *Spawner) Tick(mobs *entity.MobManager, players []PlayerInfo, world WorldAccess) SpawnResult {
	if s == nil || mobs == nil || world == nil {
		return SpawnResult{}
	}

	eligiblePlayers := filterEligiblePlayers(players)
	if len(eligiblePlayers) == 0 {
		return SpawnResult{}
	}

	result := SpawnResult{
		DespawnIDs: s.collectDespawns(mobs, eligiblePlayers),
	}

	s.spawnCounter++
	if s.spawnCounter < s.interval {
		return result
	}
	s.spawnCounter = 0

	for _, category := range spawnCategoryOrder {
		capacity := s.mobCaps[category]
		if capacity <= 0 {
			continue
		}

		current := s.countMobsBySpawnCategory(mobs, category)
		if current >= capacity {
			continue
		}

		entries := s.entriesForCategory(category)
		if len(entries) == 0 {
			continue
		}

		for attempts := 0; attempts < s.attemptsPerCategory && current < capacity; attempts++ {
			entry := weightedRandom(s.rng, entries)
			if entry == nil {
				continue
			}

			player := eligiblePlayers[s.rng.Intn(len(eligiblePlayers))]
			spawnX, spawnZ := s.randomSpawnXZ(player)
			x := int(math.Floor(spawnX))
			z := int(math.Floor(spawnZ))
			y, ok := findSpawnY(world, x, z)
			if !ok {
				continue
			}

			if !s.canSpawnAt(*entry, x, y, z, player, world) {
				continue
			}

			groupSize := s.groupSize(*entry)
			for i := 0; i < groupSize && current < capacity; i++ {
				gx := x + s.rng.Intn(5) - 2
				gz := z + s.rng.Intn(5) - 2
				gy, ok := findSpawnY(world, gx, gz)
				if !ok || !s.canSpawnAt(*entry, gx, gy, gz, player, world) {
					continue
				}

				result.Requests = append(result.Requests, SpawnRequest{
					TypeName: entry.TypeName,
					X:        float64(gx) + 0.5,
					Y:        float64(gy),
					Z:        float64(gz) + 0.5,
				})
				current++
			}
		}
	}

	return result
}

func (s *Spawner) collectDespawns(mobs *entity.MobManager, players []PlayerInfo) []int32 {
	var ids []int32
	for _, mob := range mobs.Mobs() {
		if mob == nil || mob.IsDead() || mob.Entity.Removed() {
			continue
		}

		distSq := nearestPlayerDistSq(mob.Position(), players)
		if distSq > s.hardDespawnSq {
			ids = append(ids, mob.ID())
			continue
		}
		if distSq > s.randomDespawnSq && s.rng.Intn(s.randomDespawnChance) == 0 {
			ids = append(ids, mob.ID())
		}
	}
	return ids
}

func (s *Spawner) canSpawnAt(entry SpawnEntry, x, y, z int, player PlayerInfo, world WorldAccess) bool {
	if !world.IsChunkLoaded(blockToChunk(x), blockToChunk(z)) {
		return false
	}
	if y < world.MinY() || y+1 > world.MaxY() {
		return false
	}

	dx := (float64(x) + 0.5) - player.X
	dz := (float64(z) + 0.5) - player.Z
	distSq := dx*dx + dz*dz
	if distSq < s.minSpawnDistanceSq || distSq > s.maxSpawnDistanceSq {
		return false
	}

	if !isEmptySpawnVolume(world, x, y, z) {
		return false
	}

	biomeID := world.GetBiome(x, y, z)
	if !isBiomeAllowed(entry, biomeID) {
		return false
	}

	switch entry.Category {
	case entity.MobCategoryHostile:
		return isSolidGround(world, x, y-1, z) &&
			world.GetBlockLight(x, y, z) == 0 &&
			(world.GetSkyLight(x, y, z) <= 7 || isNight(world.TimeOfDay()))
	case entity.MobCategoryPassive:
		return isPassiveGround(world.GetBlock(x, y-1, z)) && world.GetSkyLight(x, y, z) >= 9
	case entity.MobCategoryAmbient:
		return y < seaLevel && maxLight(world.GetBlockLight(x, y, z), world.GetSkyLight(x, y, z)) <= 4
	case entity.MobCategoryWaterCreature:
		return block.IsWater(world.GetBlock(x, y, z))
	default:
		return false
	}
}

func (s *Spawner) randomSpawnXZ(player PlayerInfo) (float64, float64) {
	angle := s.rng.Float64() * 2 * math.Pi
	distance := s.minSpawnDistance + s.rng.Float64()*(s.maxSpawnDistance-s.minSpawnDistance)
	return player.X + math.Cos(angle)*distance, player.Z + math.Sin(angle)*distance
}

func (s *Spawner) groupSize(entry SpawnEntry) int {
	groupSize := entry.MinGroup
	if groupSize <= 0 {
		groupSize = 1
	}
	if entry.MaxGroup > groupSize {
		groupSize += s.rng.Intn(entry.MaxGroup - groupSize + 1)
	}
	return groupSize
}

func (s *Spawner) entriesForCategory(category entity.MobCategory) []SpawnEntry {
	result := make([]SpawnEntry, 0, len(s.spawnTable))
	for _, entry := range s.spawnTable {
		if entry.Category == category {
			result = append(result, entry)
		}
	}
	return result
}

func (s *Spawner) countMobsBySpawnCategory(mobs *entity.MobManager, category entity.MobCategory) int {
	count := 0
	for _, mob := range mobs.Mobs() {
		if mob == nil || mob.Entity.Removed() {
			continue
		}
		if s.spawnCategoryForMob(mob.TypeName()) == category {
			count++
		}
	}
	return count
}

func (s *Spawner) spawnCategoryForMob(typeName string) entity.MobCategory {
	for _, entry := range s.spawnTable {
		if entry.TypeName == typeName {
			return entry.Category
		}
	}
	if def := entity.GetMobTypeDef(typeName); def != nil {
		return def.Category
	}
	return entity.MobCategoryPassive
}

func filterEligiblePlayers(players []PlayerInfo) []PlayerInfo {
	result := make([]PlayerInfo, 0, len(players))
	for _, player := range players {
		if player.Dead || player.GameMode == 1 || player.GameMode == 3 {
			continue
		}
		result = append(result, player)
	}
	return result
}

func findSpawnY(world WorldAccess, x, z int) (int, bool) {
	if !world.IsChunkLoaded(blockToChunk(x), blockToChunk(z)) {
		return 0, false
	}
	for y := world.MaxY() - 1; y >= world.MinY()+1; y-- {
		if isSolidGround(world, x, y-1, z) && isEmptySpawnVolume(world, x, y, z) {
			return y, true
		}
	}
	return 0, false
}

func isEmptySpawnVolume(world WorldAccess, x, y, z int) bool {
	return isPassable(world.GetBlock(x, y, z)) && isPassable(world.GetBlock(x, y+1, z))
}

func isPassable(stateID int32) bool {
	return block.IsAir(stateID) || (!block.IsSolid(stateID) && !block.IsWater(stateID) && !block.IsLava(stateID))
}

func isSolidGround(world WorldAccess, x, y, z int) bool {
	stateID := world.GetBlock(x, y, z)
	return block.IsSolid(stateID) && !block.IsWater(stateID) && !block.IsLava(stateID)
}

func isPassiveGround(stateID int32) bool {
	switch block.NameFromState(stateID) {
	case "minecraft:grass_block", "minecraft:podzol", "minecraft:mycelium":
		return true
	default:
		return false
	}
}

func isBiomeAllowed(entry SpawnEntry, biomeID int32) bool {
	biome := genbiome.BiomeByID(biomeID)
	if biome == nil || biome.Category == "nether" || biome.Category == "the_end" || biome.Category == "none" {
		return false
	}
	if entry.Category != entity.MobCategoryWaterCreature && (biome.Category == "ocean" || biome.Category == "river") {
		return false
	}
	if len(entry.BiomeCategories) == 0 {
		return true
	}
	for _, category := range entry.BiomeCategories {
		if biome.Category == category {
			return true
		}
	}
	return false
}

func weightedRandom(rng *rand.Rand, entries []SpawnEntry) *SpawnEntry {
	if len(entries) == 0 {
		return nil
	}
	total := 0
	for _, entry := range entries {
		if entry.Weight > 0 {
			total += entry.Weight
		}
	}
	if total <= 0 {
		return &entries[0]
	}
	roll := rng.Intn(total)
	for i := range entries {
		if entries[i].Weight <= 0 {
			continue
		}
		roll -= entries[i].Weight
		if roll < 0 {
			return &entries[i]
		}
	}
	return &entries[len(entries)-1]
}

func nearestPlayerDistSq(pos entity.Vec3, players []PlayerInfo) float64 {
	minDist := math.MaxFloat64
	for _, player := range players {
		dx := pos.X - player.X
		dy := pos.Y - player.Y
		dz := pos.Z - player.Z
		dist := dx*dx + dy*dy + dz*dz
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

func isNight(timeOfDay int64) bool {
	dayTime := timeOfDay % 24000
	if dayTime < 0 {
		dayTime += 24000
	}
	return dayTime >= 13000 && dayTime <= 23000
}

func copyMobCaps(caps map[entity.MobCategory]int) map[entity.MobCategory]int {
	result := make(map[entity.MobCategory]int, len(MobCap))
	for category, cap := range MobCap {
		result[category] = cap
	}
	for category, cap := range caps {
		result[category] = cap
	}
	return result
}

func blockToChunk(blockCoord int) int32 {
	return int32(math.Floor(float64(blockCoord) / 16))
}

func maxLight(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

func landBiomeCategories() []string {
	return []string{"plains", "forest", "taiga", "savanna", "jungle", "mountain", "extreme_hills", "beach", "swamp", "mesa", "mushroom"}
}
