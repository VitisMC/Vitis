package world

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	blockentitystate "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/ai"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/experience"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/loot"
	"github.com/vitismc/vitis/internal/metrics"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/blockentity"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/spawning"
	"github.com/vitismc/vitis/internal/world/streaming"
	"github.com/vitismc/vitis/internal/world/tick"
	"github.com/vitismc/vitis/internal/world/weather"
	"github.com/vitismc/vitis/internal/world/worldgen/random"
)

// chunkBatchRateSource is implemented by sessions that accept chunk batch feedback.
type chunkBatchRateSource interface {
	ChunkBatchRate() (float32, bool)
	ChunkBatchAcknowledgements() uint64
}

// clientViewDistanceSource is implemented by sessions that expose client settings.
type clientViewDistanceSource interface {
	ClientViewDistance() (int32, bool)
}

const (
	defaultSchedulerQueueCapacity   = 8192
	defaultSchedulerDrainPerTick    = 1024
	defaultChunkCompletionsPerTick  = 256
	defaultChunkUnloadBatchPerTick  = 64
	defaultWorldChunkStorageInitial = 2048
	defaultChunksPerTick            = 32
)

// Config defines world construction settings.
type Config struct {
	ID uint64

	Name      string
	Dimension string

	ChunkManager     *chunk.Manager
	Generator        chunk.Generator
	ChunkStore       *persistence.ChunkStore
	ChunkActivity    *metrics.ChunkActivity
	EntityIDSequence *atomic.Int32
	LootRuntime      *loot.Runtime
	SpawningRuntime  *spawning.Runtime
	ReadSeaLevel     func() (int, bool)

	WorldDir               string
	PregenerationStatePath string
	Seed                   int64
	BiomeID                int32
	Difficulty             int32
	InitialWorldAge        int64
	InitialTimeOfDay       int64
	Metadata               *level.Level

	SchedulerQueueCapacity int
	SchedulerDrainPerTick  int

	ChunkCompletionsPerTick int
	ChunkUnloadBatchPerTick int

	ChunkLoadWorkers      int
	ChunkWorkerQueueSize  int
	ChunkRequestQueueSize int
	ChunkResultQueueSize  int
	ChunkRequestPumpBatch int
	ChunkUnloadScanBatch  int
	ChunkUnloadTTLTicks   int
	ChunkStorageInitCap   int
	ChunkUnloadQueueSize  int

	ChunkStreamChunksPerTick int
	ChunkStreamViewDistance  int32
	SimulationDistance       int32
	AutosaveIntervalTicks    int
	AutosaveBatchPerTick     int
	AutosaveQueueCapacity    int
}

// World stores world-owned simulation state and chunk subsystem entry points.
type World struct {
	id           uint64
	name         string
	dimension    string
	seed         int64
	readSeaLevel func() (int, bool)
	worldDir     string

	chunks             *chunk.Manager
	chunkStore         *persistence.ChunkStore
	chunkActivity      *metrics.ChunkActivity
	streaming          *streaming.Manager
	activity           *chunk.ActivityTracker
	simulationDistance int32
	playerTicketOwners map[uint64]struct{}

	scheduler        *Scheduler
	tickScheduler    *tick.Scheduler
	entities         *entity.Manager
	tracker          *entity.Tracker
	fallingBlocks    *entity.FallingBlockManager
	items            *entity.ItemEntityManager
	itemPickup       func(*entity.Player, inventory.Slot) int32
	playerDeath      func(*entity.Player)
	furnaceUpdate    func(x, y, z int32, slots []inventory.Slot, properties [4]int16)
	xpOrbs           *entity.XPOrbManager
	tnts             *entity.TNTManager
	projectiles      *projectile.Manager
	weather          *weather.Weather
	mobs             *entity.MobManager
	mobBrains        *ai.BrainRegistry
	spawner          *spawning.Spawner
	lootRuntime      *loot.Runtime
	equipmentRuntime *equipment.Runtime
	mobPickupCursor  int32
	mobPickupItems   map[int64]int32
	blockEntities    *blockentity.Store

	tick atomic.Uint64

	worldAge      atomic.Int64
	timeOfDay     atomic.Int64
	difficulty    atomic.Int32
	daylightCycle atomic.Bool
	fireTick      atomic.Bool

	schedulerDrainPerTick    int
	chunkCompletionsPerTick  int
	chunkUnloadBatchPerTick  int
	chunkStreamChunksPerTick int
	chunkStreamViewDistance  int32
	autosave                 *chunkAutosave
	randomTicks              *random.Legacy
	randomTickPosition       int32
	lightChanges             map[int64]struct{}
	entityChunks             map[*chunk.Chunk]*persistentEntityChunk
	pregen                   *pregeneration
	metadataMu               sync.RWMutex
	metadata                 *level.Level
	playerTick               func(uint64)
	entitySpawnState         func(*entity.Player, *entity.Entity)
	playerInventory          func(*entity.Player) *inventory.WindowManager
	neighborUpdates          []neighborUpdate
	neighborCursor           int
	neighborCount            int
	neighborWork             int
	neighborBorders          map[int64]int
	neighborBorderCursor     int64
}

// New creates a world with self-contained internal defaults.
func New(config Config) (*World, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("create world: empty world name")
	}
	if config.SimulationDistance < 0 || config.SimulationDistance > 96 {
		return nil, fmt.Errorf("create world: simulation distance must be between 0 and 96")
	}

	chunkManager := config.ChunkManager
	if chunkManager == nil {
		generator := config.Generator
		if generator == nil {
			generator = chunk.NewTerrainGenerator(config.Seed, config.BiomeID)
		}
		if config.ChunkActivity != nil {
			generator = chunk.ObservedGenerator{Generator: generator, Record: config.ChunkActivity.RecordGeneration}
		}
		if config.ChunkStore != nil {
			generator = chunk.NewPersistentGenerator(config.ChunkStore, generator, config.BiomeID)
		}

		loader := chunk.NewLoader(chunk.LoaderConfig{
			Generator:           generator,
			WorkerCount:         config.ChunkLoadWorkers,
			WorkerQueueCapacity: config.ChunkWorkerQueueSize,
			RequestQueueSize:    config.ChunkRequestQueueSize,
			ResultQueueSize:     config.ChunkResultQueueSize,
			PumpBatch:           config.ChunkRequestPumpBatch,
		})

		chunkManager = chunk.NewManager(chunk.ManagerConfig{
			Loader:                 loader,
			OnUnload:               makeChunkSaveFunc(config.ChunkStore),
			InitialStorageCapacity: maxInt(config.ChunkStorageInitCap, defaultWorldChunkStorageInitial),
			UnloadTTL:              saturatingIntToUint64(config.ChunkUnloadTTLTicks),
			UnloadBatch:            config.ChunkUnloadBatchPerTick,
			CompletionApplyBatch:   config.ChunkCompletionsPerTick,
			RequestPumpBatch:       config.ChunkRequestPumpBatch,
			UnloadScanBatch:        config.ChunkUnloadScanBatch,
			UnloadQueueCapacity:    config.ChunkUnloadQueueSize,
		})
	}

	schedulerCapacity := config.SchedulerQueueCapacity
	if schedulerCapacity <= 0 {
		schedulerCapacity = defaultSchedulerQueueCapacity
	}

	schedulerDrain := config.SchedulerDrainPerTick
	if schedulerDrain <= 0 {
		schedulerDrain = defaultSchedulerDrainPerTick
	}

	chunkCompletions := config.ChunkCompletionsPerTick
	if chunkCompletions <= 0 {
		chunkCompletions = defaultChunkCompletionsPerTick
	}

	chunkUnloadBatch := config.ChunkUnloadBatchPerTick
	if chunkUnloadBatch <= 0 {
		chunkUnloadBatch = defaultChunkUnloadBatchPerTick
	}

	chunkStreamPerTick := config.ChunkStreamChunksPerTick
	if chunkStreamPerTick <= 0 {
		chunkStreamPerTick = defaultChunksPerTick
	}
	chunkStreamViewDistance := config.ChunkStreamViewDistance
	if chunkStreamViewDistance <= 0 {
		chunkStreamViewDistance = 10
	}

	entMgr := entity.NewManagerWithIDSequence(config.EntityIDSequence)
	w := &World{
		id:                       config.ID,
		name:                     config.Name,
		dimension:                config.Dimension,
		seed:                     config.Seed,
		readSeaLevel:             config.ReadSeaLevel,
		worldDir:                 config.WorldDir,
		chunks:                   chunkManager,
		chunkStore:               config.ChunkStore,
		chunkActivity:            config.ChunkActivity,
		streaming:                streaming.NewManager(),
		scheduler:                NewScheduler(schedulerCapacity),
		tickScheduler:            tick.NewScheduler(),
		entities:                 entMgr,
		tracker:                  entity.NewTracker(),
		fallingBlocks:            entity.NewFallingBlockManager(),
		items:                    entity.NewItemEntityManager(),
		xpOrbs:                   entity.NewXPOrbManager(),
		tnts:                     entity.NewTNTManager(),
		projectiles:              projectile.NewManager(),
		weather:                  weather.New(config.Seed),
		schedulerDrainPerTick:    schedulerDrain,
		chunkCompletionsPerTick:  chunkCompletions,
		chunkUnloadBatchPerTick:  chunkUnloadBatch,
		chunkStreamChunksPerTick: chunkStreamPerTick,
		chunkStreamViewDistance:  chunkStreamViewDistance,
	}
	w.blockEntities = blockentity.NewStore(w.chunks.GetChunk)
	w.tracker.SetSpawnStateHandler(w.sendEntitySpawnState)
	w.blockEntities.SetLootRuntime(config.LootRuntime)
	w.worldAge.Store(config.InitialWorldAge)
	w.SetTimeOfDay(config.InitialTimeOfDay)
	w.difficulty.Store(clampDifficulty(config.Difficulty))
	w.daylightCycle.Store(true)
	w.fireTick.Store(true)
	w.initMetadata(config.Metadata)
	w.chunks.EnsureSaveHandler(makeChunkSaveFunc(config.ChunkStore))
	w.initActivity(config.SimulationDistance)
	w.initRandomTicks(config.Seed)
	w.mobs = entity.NewMobManager(entMgr, entMgr.AllocateID)
	w.lootRuntime = config.LootRuntime
	w.mobs.SetDeathLootHandler(w.mobDeathLoot)
	w.mobBrains = ai.NewBrainRegistry()
	w.mobs.SetSpawnHandler(w.mobBrains.RegisterMob)
	if config.SpawningRuntime != nil {
		w.spawner = spawning.NewSpawner(nil, 20)
		w.spawner.SetRuntime(config.SpawningRuntime)
	}
	if config.ChunkStore != nil {
		w.autosave = newChunkAutosave(config.ChunkStore, config.AutosaveIntervalTicks, config.AutosaveBatchPerTick, config.AutosaveQueueCapacity)
		w.chunks.SetUnloadPreparer(func(c *chunk.Chunk) (bool, error) {
			_, err := w.autosave.enqueue(c)
			if err != nil {
				logger.Error("unload chunk snapshot failed", "world", w.name, "x", c.X(), "z", c.Z(), "error", err)
			}
			return false, err
		})
	}
	w.pregen = newPregeneration(w, config.PregenerationStatePath)
	w.chunks.SetLoadObserver(w.pregen.onLoad)
	return w, nil
}

func clampDifficulty(difficulty int32) int32 {
	if difficulty < 0 || difficulty > 3 {
		return 2
	}
	return difficulty
}

// Difficulty returns the authoritative world difficulty ID.
func (w *World) Difficulty() int32 {
	if w == nil {
		return 2
	}
	return w.difficulty.Load()
}

// Seed returns the seed used by this world's generator and weather.
func (w *World) Seed() int64 {
	if w == nil {
		return 0
	}
	return w.seed
}

// SetDifficulty changes difficulty and notifies all tracked players.
func (w *World) SetDifficulty(difficulty int32) {
	if w == nil || difficulty < 0 || difficulty > 3 {
		return
	}
	w.difficulty.Store(difficulty)
	packet := &playpacket.Difficulty{DifficultyLevel: byte(difficulty), Locked: false}
	for _, player := range w.tracker.Players() {
		if !player.Removed() && player.Session() != nil {
			_ = player.Session().Send(packet)
		}
	}
}

// SetDaylightCycle enables or freezes the day clock.
func (w *World) SetDaylightCycle(enabled bool) { w.daylightCycle.Store(enabled) }

// DaylightCycle reports whether clients should advance their local day clock.
func (w *World) DaylightCycle() bool { return w.daylightCycle.Load() }

// SetWeatherCycle enables or disables natural weather changes.
func (w *World) SetWeatherCycle(enabled bool) { w.weather.SetCycleEnabled(enabled) }

// SetFireTick enables or disables scheduled fire updates.
func (w *World) SetFireTick(enabled bool) { w.fireTick.Store(enabled) }

// BlockEntities returns the world-owned block entity store.
func (w *World) BlockEntities() *blockentity.Store {
	if w == nil {
		return nil
	}
	return w.blockEntities
}

// ID returns stable world identifier.
func (w *World) ID() uint64 {
	if w == nil {
		return 0
	}
	return w.id
}

// Name returns world name.
func (w *World) Name() string {
	if w == nil {
		return ""
	}
	return w.name
}

// Dimension returns the namespaced dimension identifier.
func (w *World) Dimension() string {
	if w == nil {
		return ""
	}
	return w.dimension
}

// Tick advances world simulation by one tick.
func (w *World) Tick() {
	if w == nil {
		return
	}

	newTick := w.tick.Add(1)
	w.neighborWork = 0
	w.chunks.SetTick(newTick)

	w.worldAge.Add(1)
	if w.daylightCycle.Load() {
		w.timeOfDay.Add(1)
	}

	w.tickWeather()
	w.scheduler.Drain(w.schedulerDrainPerTick)
	w.updateActivity()
	w.tickChunkInhabitation()
	if w.playerTick != nil {
		w.playerTick(newTick)
	}
	w.processNeighborUpdates()
	w.processScheduledTicks()
	w.tickRandomBlocks()
	w.tickBlockEntities()
	w.tickFallingBlocks()
	w.tickItems()
	w.tickXPOrbs()
	w.tickTNT()
	w.tickProjectiles()
	w.tickMobs()
	w.processNeighborUpdates()
	players := w.tracker.Players()
	loadCenters := make([]chunk.LoadPriorityCenter, 0, len(players))
	for _, player := range players {
		if !player.Removed() {
			loadCenters = append(loadCenters, chunk.LoadPriorityCenter{X: player.ChunkX(), Z: player.ChunkZ()})
		}
	}
	w.chunks.SetLoadPriorityCenters(loadCenters)
	w.chunks.ApplyLoadCompletions(w.chunkCompletionsPerTick)
	w.processChunkNeighborChecks()
	w.processNeighborUpdates()
	w.pregen.tick()
	w.chunks.PumpLoadRequests()
	w.chunks.UpdateActiveChunks()
	w.tickStreaming()
	w.tickPlayers()
	w.entities.Tick(w.tick.Load())
	w.tracker.TickWithChunkFilter(w.entities, func(playerID, chunkX, chunkZ int32) bool {
		streamer, ok := w.streaming.GetStreamer(playerID)
		return ok && streamer.View().IsLoaded(streaming.ChunkPos{X: chunkX, Z: chunkZ})
	})
	w.tickLighting()
	w.logEntitySnapshotError(w.refreshEntitySnapshots())
	if w.autosave != nil {
		w.autosave.tick(w)
	}
	w.chunks.ProcessUnloads(w.chunkUnloadBatchPerTick)

	if w.tick.Load()%20 == 0 {
		w.broadcastTime()
	}
}

// CurrentTick returns currently completed world tick.
func (w *World) CurrentTick() uint64 {
	if w == nil {
		return 0
	}
	return w.tick.Load()
}

// GetChunk returns one loaded chunk when available.
func (w *World) GetChunk(x, z int32) (*chunk.Chunk, bool) {
	if w == nil {
		return nil, false
	}
	return w.chunks.GetChunk(x, z)
}

// LoadChunk queues one asynchronous chunk load operation.
func (w *World) LoadChunk(x, z int32) {
	if w == nil {
		return
	}
	w.scheduler.Schedule(func() {
		w.chunks.RequestLoad(x, z)
	})
}

// UnloadChunk marks one chunk for batched unload.
func (w *World) UnloadChunk(x, z int32) {
	if w == nil {
		return
	}
	w.scheduler.Schedule(func() {
		w.chunks.MarkForUnload(x, z)
	})
}

// ChunkManager exposes chunk subsystem for integration points.
func (w *World) ChunkManager() *chunk.Manager {
	if w == nil {
		return nil
	}
	return w.chunks
}

// Scheduler exposes world-local task scheduler.
func (w *World) Scheduler() *Scheduler {
	if w == nil {
		return nil
	}
	return w.scheduler
}

// Submit queues one mutation for execution by the world tick.
func (w *World) Submit(task func()) bool {
	if w == nil || w.scheduler == nil {
		return false
	}
	return w.scheduler.Schedule(task)
}

// EntityManager returns the world's entity manager.
func (w *World) EntityManager() *entity.Manager {
	if w == nil {
		return nil
	}
	return w.entities
}

// ItemEntities returns the world's item entity manager.
func (w *World) ItemEntities() *entity.ItemEntityManager {
	if w == nil {
		return nil
	}
	return w.items
}

// EntityTracker returns the world's entity tracker.
func (w *World) EntityTracker() *entity.Tracker {
	if w == nil {
		return nil
	}
	return w.tracker
}

// AddEntity adds an entity to the world's entity manager.
func (w *World) AddEntity(e *entity.Entity) bool {
	if w == nil || e == nil {
		return false
	}
	return w.entities.Add(e)
}

// RemoveEntity marks an entity for removal from the world.
func (w *World) RemoveEntity(id int32) {
	if w == nil {
		return
	}
	w.entities.Remove(id)
}

// GetEntity returns an entity by ID.
func (w *World) GetEntity(id int32) *entity.Entity {
	if w == nil {
		return nil
	}
	return w.entities.Get(id)
}

// AddPlayer adds a player entity to the world and registers it for tracking and chunk streaming.
func (w *World) AddPlayer(p *entity.Player) {
	if w == nil || p == nil {
		return
	}
	if !w.entities.Add(p.Entity) {
		return
	}
	w.bindPlayerArmor(p)
	w.tracker.AddPlayer(p)
	w.playerMemoryJoined()

	streamConfig := streaming.StreamerConfig{
		ChunksPerTick: w.chunkStreamChunksPerTick,
		ViewDistance:  p.ViewDistance(),
		Provider:      w.chunks,
		Sender:        p.Session(),
	}
	if w.chunkActivity != nil {
		streamConfig.OnChunkSent = w.chunkActivity.RecordSent
		streamConfig.OnChunkAcknowledged = w.chunkActivity.RecordFullPath
	}
	s := w.streaming.AddPlayer(p.ID(), streamConfig)
	s.InitialLoad(p.ChunkX(), p.ChunkZ())
}

// ResendPlayerChunks resets the streamer's client cache after respawn.
// The caller must run on the world tick.
func (w *World) ResendPlayerChunks(playerID int32, chunkX, chunkZ int32) {
	if w == nil || w.streaming == nil {
		return
	}
	if stream, ok := w.streaming.GetStreamer(playerID); ok {
		stream.ForceResend()
		stream.InitialLoad(chunkX, chunkZ)
	}
}

// RemovePlayer removes a player from the world and tracking.
func (w *World) RemovePlayer(p *entity.Player) {
	if w.DetachPlayer(p) {
		p.Remove()
	}
}

// DetachPlayer releases world membership without destroying a transferring player.
// The caller must run on the common world tick goroutine.
func (w *World) DetachPlayer(p *entity.Player) bool {
	if w == nil || p == nil {
		return false
	}
	if registered := w.entities.Get(p.ID()); registered != p.Entity {
		if registered != nil {
			return false
		}
		tracked := false
		for _, existing := range w.tracker.Players() {
			tracked = tracked || existing == p
		}
		if !tracked {
			return false
		}
	}
	p.Living().SetArmorDamageHandler(nil)
	w.streaming.RemovePlayer(p.ID(), p.Session())
	w.entities.Evict(p.ID())
	w.tracker.RemovePlayer(p.ID())
	for _, observer := range w.tracker.Players() {
		if _, tracked := observer.TrackedEntities()[p.ID()]; tracked {
			delete(observer.TrackedEntities(), p.ID())
			observer.SendDespawnEntities([]int32{p.ID()})
		}
	}
	p.ResetTracking()
	owner := uint64(uint32(p.ID()))
	w.RemoveChunkTicket(chunk.TicketPlayer, owner)
	w.RemoveChunkTicket(chunk.TicketView, owner)
	delete(w.playerTicketOwners, owner)
	w.playerMemoryLeft()
	return true
}

// Scheduler is a bounded world-thread task ingress queue.
type Scheduler struct {
	queue chan func()
}

// NewScheduler creates a world-local scheduler.
func NewScheduler(queueCapacity int) *Scheduler {
	if queueCapacity <= 0 {
		queueCapacity = defaultSchedulerQueueCapacity
	}
	return &Scheduler{queue: make(chan func(), queueCapacity)}
}

// Schedule enqueues one world-thread task without blocking the caller.
// It returns false when the bounded ingress queue cannot accept the task.
func (s *Scheduler) Schedule(task func()) bool {
	if s == nil || task == nil {
		return false
	}
	select {
	case s.queue <- task:
		return true
	default:
		return false
	}
}

// Drain executes up to max currently queued tasks.
func (s *Scheduler) Drain(max int) int {
	if s == nil || max <= 0 {
		return 0
	}
	executed := 0
	for i := 0; i < max; i++ {
		select {
		case task := <-s.queue:
			if task != nil {
				task()
			}
			executed++
		default:
			return executed
		}
	}
	return executed
}

// Pending returns current queued task count.
func (s *Scheduler) Pending() int {
	if s == nil {
		return 0
	}
	return len(s.queue)
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func saturatingIntToUint64(value int) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

// StreamingManager returns the world's chunk streaming manager.
func (w *World) StreamingManager() *streaming.Manager {
	if w == nil {
		return nil
	}
	return w.streaming
}

func (w *World) tickStreaming() {
	if w.streaming == nil {
		return
	}
	players := w.tracker.Players()
	w.streaming.Tick(func(playerID int32) (int32, int32, bool) {
		for _, p := range players {
			if p.ID() == playerID {
				viewDistance := w.clientViewDistanceFor(p)
				p.SetViewDistance(viewDistance)
				w.streaming.SetPlayerViewDistance(playerID, viewDistance)
				if streamer, ok := w.streaming.GetStreamer(playerID); ok {
					streamer.SetChunksPerTick(w.chunkStreamChunksPerTick)
					w.applyChunkBatchFeedback(streamer, p)
				}
				return p.ChunkX(), p.ChunkZ(), true
			}
		}
		return 0, 0, false
	})

	w.streaming.ForEachLoadedChunk(func(pos streaming.ChunkPos) {
		w.chunks.Touch(pos.X, pos.Z)
	})
}

// applyChunkBatchFeedback forwards validated client delivery feedback to a streamer.
func (w *World) applyChunkBatchFeedback(streamer *streaming.Streamer, player *entity.Player) {
	if streamer == nil || player == nil || player.Session() == nil {
		return
	}
	source, ok := player.Session().(chunkBatchRateSource)
	if !ok {
		return
	}
	rate, ok := source.ChunkBatchRate()
	if !ok || math.IsNaN(float64(rate)) || math.IsInf(float64(rate), 0) {
		return
	}
	streamer.UpdateClientFeedback(rate, source.ChunkBatchAcknowledgements())
}

// clientViewDistanceFor caps a client's render distance to the world configuration.
func (w *World) clientViewDistanceFor(player *entity.Player) int32 {
	distance := int32(w.chunkStreamViewDistance)
	if player == nil || player.Session() == nil {
		return distance
	}

	source, ok := player.Session().(clientViewDistanceSource)
	if !ok {
		return distance
	}
	requested, ok := source.ClientViewDistance()
	if !ok || requested < 2 {
		return distance
	}
	if requested < distance {
		return requested
	}
	return distance
}

// chunkStreamRateFor caps client feedback to the configured per-player send budget.
func (w *World) chunkStreamRateFor(player *entity.Player) int {
	rate := w.chunkStreamChunksPerTick
	if player == nil || player.Session() == nil {
		return rate
	}

	source, ok := player.Session().(chunkBatchRateSource)
	if !ok {
		return rate
	}
	requested, ok := source.ChunkBatchRate()
	if !ok || math.IsNaN(float64(requested)) || math.IsInf(float64(requested), 0) || requested < 0 {
		return rate
	}

	clientRate := int(math.Ceil(float64(requested)))
	if clientRate < rate {
		return clientRate
	}
	return rate
}

// WorldAge returns the total age of the world in ticks.
func (w *World) WorldAge() int64 {
	if w == nil {
		return 0
	}
	return w.worldAge.Load()
}

// TimeOfDay returns the full accumulated daytime.
func (w *World) TimeOfDay() int64 {
	if w == nil {
		return 0
	}
	return w.timeOfDay.Load()
}

// SetTimeOfDay sets the current time of day.
func (w *World) SetTimeOfDay(t int64) {
	if w == nil {
		return
	}
	w.timeOfDay.Store(t)
}

func (w *World) tickPlayers() {
	players := w.tracker.Players()
	for _, p := range players {
		if p.Removed() {
			continue
		}
		p.TickClientLoadTimeout()
		if !p.HasClientLoaded() {
			continue
		}
		living := p.Living()
		oldHealth, oldFood, wasDead := living.Health(), living.FoodLevel(), living.IsDead()
		living.TickLiving()
		living.TickHungerWithRules(w.Difficulty(), w.GameRule("naturalRegeneration") == "true")

		diff := living.TickEffects()
		if properties := living.Attributes().DirtyProperties(); len(properties) != 0 {
			w.broadcastTrackedPacket(p.ID(), &playpacket.EntityUpdateAttributes{EntityID: p.ID(), Properties: properties})
		}
		sess := p.Session()
		if sess != nil {
			for _, id := range diff.Removed {
				_ = sess.Send(&playpacket.RemoveEntityEffect{
					EntityID: p.ID(),
					EffectID: id,
				})
			}
		}

		envDamages := living.TickEnvironment(w.getBlockAt)
		for _, src := range envDamages {
			if living.IsDead() {
				break
			}
			actual := living.ApplyDamage(src, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
			if actual > 0 && sess != nil {
				_ = sess.Send(&playpacket.HurtAnimation{
					EntityID: p.ID(),
					Yaw:      0,
				})
			}
		}
		if sess != nil && (oldHealth != living.Health() || oldFood != living.FoodLevel() || w.tick.Load()%20 == 0) {
			_ = sess.Send(&playpacket.UpdateHealth{
				Health:         living.Health(),
				Food:           living.FoodLevel(),
				FoodSaturation: living.FoodSaturation(),
			})
		}
		if !wasDead && living.IsDead() && sess != nil {
			_ = sess.Send(&playpacket.DeathCombatEvent{PlayerID: p.ID(), Message: p.Username() + " died"})
		}
		if living.IsDead() && !living.DeathHandled() {
			living.MarkDeathHandled()
			if w.playerDeath != nil {
				w.playerDeath(p)
			}
		}
	}
}

func formatDeathMessage(dtype entity.DamageType, victimName, attackerName string) string {
	tmpl := dtype.DeathMessage()
	if attackerName != "" {
		return fmt.Sprintf(tmpl, victimName, attackerName)
	}
	return fmt.Sprintf(tmpl, victimName)
}

func (w *World) broadcastTime() {
	players := w.tracker.Players()
	if len(players) == 0 {
		return
	}
	for _, p := range players {
		if p.Removed() {
			continue
		}
		sess := p.Session()
		if sess == nil {
			continue
		}
		_ = sess.Send(&playpacket.UpdateTime{WorldAge: w.worldAge.Load(), TimeOfDay: w.timeOfDay.Load(), TickDayTime: w.daylightCycle.Load()})
	}
}

func (w *World) tickWeather() {
	if w.weather == nil {
		return
	}
	packets := w.weather.Tick()
	if len(packets) == 0 {
		return
	}
	players := w.tracker.Players()
	for _, pkt := range packets {
		ge := &playpacket.GameEvent{Event: pkt.Event, Value: pkt.Value}
		for _, p := range players {
			if p.Removed() {
				continue
			}
			sess := p.Session()
			if sess != nil {
				_ = sess.Send(ge)
			}
		}
	}
}

// Weather returns the world's weather system.
func (w *World) Weather() *weather.Weather {
	if w == nil {
		return nil
	}
	return w.weather
}

// SetWeather changes the world weather and broadcasts the change to all players.
func (w *World) SetWeather(state weather.State, duration int32) {
	if w == nil || w.weather == nil {
		return
	}
	packets := w.weather.SetWeather(state, duration)
	players := w.tracker.Players()
	for _, pkt := range packets {
		ge := &playpacket.GameEvent{Event: pkt.Event, Value: pkt.Value}
		for _, p := range players {
			if p.Removed() {
				continue
			}
			sess := p.Session()
			if sess != nil {
				_ = sess.Send(ge)
			}
		}
	}
}

// SaveAllDirty persists all in-memory dirty chunks to disk.
func (w *World) SaveAllDirty() (int, error) {
	if w == nil || w.chunks == nil {
		return 0, nil
	}
	w.processNeighborUpdates()
	w.markScheduledTickSnapshots()
	entityErr := w.refreshEntitySnapshots()
	saved, err := w.chunks.SaveAllDirty()
	return saved, errors.Join(entityErr, err)
}

// Close saves dirty chunks and releases world-owned async resources.
func (w *World) Close(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.processNeighborUpdates()
	var checkpointErr error
	if w.autosave != nil {
		w.autosave.close()
	}
	w.markScheduledTickSnapshots()
	if w.pregen != nil {
		checkpointErr = w.pregen.stopGenerationRelease(ctx)
		w.pregen.mu.Lock()
		if w.pregen.state == "running" {
			w.pregen.stopClockLocked()
			w.pregen.state = "paused"
		}
		if w.pregen.state == "paused" {
			checkpointErr = errors.Join(checkpointErr, w.pregen.saveLocked())
		}
		w.pregen.mu.Unlock()
	}
	_, saveErr := w.SaveAllDirty()
	closeErr := w.chunks.Close(ctx)
	if w.chunkStore != nil {
		closeErr = errors.Join(closeErr, w.chunkStore.Close())
	}
	var metadataErr error
	if w.worldDir != "" {
		metadata := w.MetadataSnapshot()
		var err error
		if metadata == nil {
			metadata, err = level.Open(w.worldDir)
		}
		if err != nil {
			metadataErr = err
		} else {
			metadata.Time = w.WorldAge()
			metadata.DayTime = w.TimeOfDay()
			metadata.Difficulty = int8(w.Difficulty())
			metadataErr = metadata.Save(w.worldDir)
		}
	}
	return errors.Join(checkpointErr, saveErr, closeErr, metadataErr)
}

func (w *World) processScheduledTicks() {
	ticks := w.tickScheduler.DrainEligible(w.tick.Load(), w.schedulerDrainPerTick, w.IsBlockTicking)
	for _, t := range ticks {
		switch t.Type {
		case tick.TickTypeBlock:
			w.processBlockTick(t)
		case tick.TickTypeFluid:
			w.processFluidTick(t)
		}
	}
}

func (w *World) processBlockTick(t *tick.ScheduledTick) {
	stateID := w.getBlockAt(t.X, t.Y, t.Z)
	if stateID <= 0 {
		return
	}
	if block.NameFromState(stateID) != block.NameFromState(t.SubID) {
		return
	}
	w.checkNeighborShape(neighborUpdate{x: t.X, y: t.Y, z: t.Z}, stateID, true)
	if w.getBlockAt(t.X, t.Y, t.Z) != stateID {
		return
	}
	behavior.GetByState(stateID).OnNeighborUpdate(&behavior.Context{X: t.X, Y: t.Y, Z: t.Z, StateID: stateID})
	if w.getBlockAt(t.X, t.Y, t.Z) != stateID {
		return
	}
	if name := block.NameFromState(stateID); name == "minecraft:fire" || name == "minecraft:soul_fire" {
		w.tickFire(t.X, t.Y, t.Z, stateID)
		return
	}

	b := behavior.GetByState(stateID)
	ctx := &behavior.TickContext{
		X: t.X, Y: t.Y, Z: t.Z,
		StateID:     stateID,
		CurrentTick: w.tick.Load(),
	}

	result := b.OnScheduledTick(ctx)
	if result == nil {
		return
	}

	if result.SpawnFallingBlock {
		belowState := w.getBlockAt(t.X, t.Y-1, t.Z)
		if !behavior.CanFallThrough(belowState) {
			return
		}
		w.spawnFallingBlock(t.X, t.Y, t.Z, result.BlockStateID)
	}

	if result.SetBlock && result.NewBlockState >= 0 {
		w.setBlockAt(t.X, t.Y, t.Z, result.NewBlockState)
		w.broadcastBlockUpdate(t.X, t.Y, t.Z, result.NewBlockState)
	}

	if result.Reschedule {
		delay := b.TickDelay()
		if delay > 0 {
			w.tickScheduler.Schedule(t.X, t.Y, t.Z, w.tick.Load(), delay, tick.PriorityNormal, tick.TickTypeBlock, stateID)
		}
	}
}

func (w *World) spawnFallingBlock(x, y, z int, stateID int32) {
	w.setBlockAt(x, y, z, 0)
	w.broadcastBlockUpdate(x, y, z, 0)

	uuid := protocol.GenerateUUID()
	id := w.entities.AllocateID()
	pos := entity.Vec3{X: float64(x) + 0.5, Y: float64(y), Z: float64(z) + 0.5}

	fb := entity.NewFallingBlock(id, uuid, pos, stateID)
	if !w.entities.Add(fb.Entity) {
		return
	}
	w.fallingBlocks.Add(fb)
}

func (w *World) tickFallingBlocks() {
	var toRemove []int32
	for id, fb := range w.fallingBlocks.All() {
		if !entity.IsTickingAt(w, fb.Position()) {
			continue
		}
		fb.Tick(func(x, y, z float64) bool {
			bx, by, bz := int(math.Floor(x)), int(math.Floor(y)), int(math.Floor(z))
			state := w.getBlockAt(bx, by, bz)
			return !behavior.CanFallThrough(state)
		})

		if fb.ShouldLand() {
			lx, ly, lz := fb.LandingPosition()
			targetState := w.getBlockAt(lx, ly, lz)
			if behavior.CanFallThrough(targetState) || block.IsAir(targetState) {
				w.setBlockAt(lx, ly, lz, fb.BlockStateID())
				w.broadcastBlockUpdate(lx, ly, lz, fb.BlockStateID())
			}
			fb.Entity.Remove()
			toRemove = append(toRemove, id)
		} else if fb.Removed() {
			toRemove = append(toRemove, id)
		}
	}
	for _, id := range toRemove {
		w.fallingBlocks.Remove(id)
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
}

// GetBlockStateAt returns the block state ID at absolute world coordinates.
// Implements physics.BlockAccess interface.
func (w *World) GetBlockStateAt(x, y, z int) int32 {
	return w.getBlockAt(x, y, z)
}

// getBlockAt preserves an unavailable sentinel for unloaded columns and air beyond build height.
func (w *World) getBlockAt(x, y, z int) int32 {
	c, ok := w.chunks.PeekLoaded(int32(x>>4), int32(z>>4))
	if !ok {
		return -1
	}
	return c.GetBlock(x, y, z)
}

func (w *World) setBlockAt(x, y, z int, stateID int32) {
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := w.chunks.GetChunk(cx, cz)
	if !ok || c == nil {
		return
	}
	if y < c.Data().MinY() || y > c.Data().MaxY() || c.GetBlock(x, y, z) == stateID {
		return
	}
	c.SetBlock(x, y, z, stateID)
	w.NotifyNeighbors(x, y, z)
}

func (w *World) broadcastBlockUpdate(x, y, z int, stateID int32) {
	pkt := &playpacket.BlockUpdate{
		Position: playpacket.BlockPos{X: int32(x), Y: int32(y), Z: int32(z)},
		BlockID:  stateID,
	}
	for _, p := range w.tracker.Players() {
		if p.Removed() {
			continue
		}
		sess := p.Session()
		if sess != nil {
			_ = sess.Send(pkt)
		}
	}
}

func (w *World) broadcastRemoveEntity(id int32) {
	pkt := &playpacket.RemoveEntities{EntityIDs: []int32{id}}
	for _, p := range w.tracker.Players() {
		if p.Removed() {
			continue
		}
		sess := p.Session()
		if sess != nil {
			_ = sess.Send(pkt)
		}
	}
}

func (w *World) ScheduleBlockTick(x, y, z int, delay int, priority tick.Priority) {
	if w == nil || w.tickScheduler == nil {
		return
	}
	stateID := w.getBlockAt(x, y, z)
	w.tickScheduler.Schedule(x, y, z, w.tick.Load(), delay, priority, tick.TickTypeBlock, stateID)
}

func (w *World) TickScheduler() *tick.Scheduler {
	if w == nil {
		return nil
	}
	return w.tickScheduler
}

func (w *World) processFluidTick(t *tick.ScheduledTick) {
	stateID := w.getBlockAt(t.X, t.Y, t.Z)
	if !fluid.IsFluid(stateID) {
		return
	}

	config := fluid.GetFluidConfig(stateID)
	if config == nil {
		return
	}

	resultState, hasInteraction := fluid.CheckLavaWaterInteraction(t.X, t.Y, t.Z, w.getBlockAt)
	if hasInteraction {
		w.setBlockAt(t.X, t.Y, t.Z, resultState)
		w.broadcastBlockUpdate(t.X, t.Y, t.Z, resultState)
		return
	}

	newState := fluid.ComputeNewFluidState(t.X, t.Y, t.Z, *config, w.getBlockAt)
	if newState != stateID {
		if newState == 0 {
			w.setBlockAt(t.X, t.Y, t.Z, 0)
			w.broadcastBlockUpdate(t.X, t.Y, t.Z, 0)
		} else {
			w.setBlockAt(t.X, t.Y, t.Z, newState)
			w.broadcastBlockUpdate(t.X, t.Y, t.Z, newState)
			w.scheduleFluidTick(t.X, t.Y, t.Z, config.FlowSpeed)
		}
	}

	flowResult := fluid.ComputeFlow(t.X, t.Y, t.Z, *config, w.getBlockAt)
	for pos, state := range flowResult.NewStates {
		existingState := w.getBlockAt(pos[0], pos[1], pos[2])
		if fluid.CanBeReplacedByFluid(existingState, config.FluidType) {
			w.setBlockAt(pos[0], pos[1], pos[2], state)
			w.broadcastBlockUpdate(pos[0], pos[1], pos[2], state)
			w.scheduleFluidTick(pos[0], pos[1], pos[2], config.FlowSpeed)
		}
	}
}

func (w *World) scheduleFluidTick(x, y, z int, delay int) {
	if w == nil || w.tickScheduler == nil {
		return
	}
	stateID := w.getBlockAt(x, y, z)
	fluidID := fluid.GetFluidID(fluid.GetFluidType(stateID))
	if !w.tickScheduler.IsScheduled(x, y, z, tick.TickTypeFluid, fluidID) {
		w.tickScheduler.Schedule(x, y, z, w.tick.Load(), delay, tick.PriorityNormal, tick.TickTypeFluid, fluidID)
	}
}

// ItemManager returns the world's item entity manager.
func (w *World) ItemManager() *entity.ItemEntityManager {
	if w == nil {
		return nil
	}
	return w.items
}

// SetItemPickupHandler installs the inventory transfer performed on the world tick.
func (w *World) SetItemPickupHandler(handler func(*entity.Player, inventory.Slot) int32) {
	w.itemPickup = handler
}

// SetPlayerDeathHandler installs one-time inventory and experience drops.
func (w *World) SetPlayerDeathHandler(handler func(*entity.Player)) {
	w.playerDeath = handler
}

// SetFurnaceUpdateHandler installs the client update callback on the world tick.
func (w *World) SetFurnaceUpdateHandler(handler func(x, y, z int32, slots []inventory.Slot, properties [4]int16)) {
	w.furnaceUpdate = handler
}

func (w *World) tickBlockEntities() {
	w.chunks.VisitLoaded(func(owner *chunk.Chunk) {
		if !w.IsBlockTicking(owner.X(), owner.Z()) {
			return
		}
		data := owner.Data()
		if data == nil {
			return
		}
		for _, blockEntity := range data.BlockEntities {
			if blockEntity == nil {
				continue
			}
			name := blockEntity.TypeName()
			if name != "minecraft:furnace" && name != "minecraft:blast_furnace" && name != "minecraft:smoker" {
				continue
			}
			x, y, z := blockEntity.Position()
			furnace, ok := blockEntity.(*blockentitystate.FurnaceLikeBlockEntity)
			if !ok {
				furnace = w.blockEntities.Furnace(int(x), int(y), int(z), name)
			}
			if furnace == nil || !furnace.Tick() {
				continue
			}
			state := w.getBlockAt(int(x), int(y), int(z))
			if block.NameFromState(state) == name {
				properties := block.PropertiesFromState(state)
				lit := "false"
				if furnace.IsBurning() {
					lit = "true"
				}
				if properties != nil && properties["lit"] != lit {
					properties["lit"] = lit
					if next := block.StateID(name, properties); next >= 0 {
						w.setBlockAt(int(x), int(y), int(z), next)
						w.broadcastBlockUpdate(int(x), int(y), int(z), next)
					}
				}
			}
			owner.MarkDirty()
			if w.furnaceUpdate != nil {
				properties := [4]int16{}
				for i := range properties {
					properties[i] = furnace.GetProperty(i)
				}
				w.furnaceUpdate(x, y, z, furnace.Container().Slots(), properties)
			}
		}
	})
}

// XPOrbManager returns the world's XP orb manager.
func (w *World) XPOrbManager() *entity.XPOrbManager {
	if w == nil {
		return nil
	}
	return w.xpOrbs
}

// TNTManager returns the world's TNT manager.
func (w *World) TNTManager() *entity.TNTManager {
	if w == nil {
		return nil
	}
	return w.tnts
}

// SpawnItemEntity creates and registers an item entity in the world.
func (w *World) SpawnItemEntity(item *entity.ItemEntity) {
	if w == nil || item == nil {
		return
	}
	if !w.entities.Add(item.Entity) {
		return
	}
	w.items.Add(item)
}

// SpawnXPOrb creates and registers an XP orb entity in the world.
func (w *World) SpawnXPOrb(orb *entity.XPOrb) {
	if w == nil || orb == nil {
		return
	}
	if !w.entities.Add(orb.Entity) {
		return
	}
	w.xpOrbs.Add(orb)
}

// SpawnTNT creates and registers a primed TNT entity in the world.
func (w *World) SpawnTNT(tnt *entity.TNTEntity) {
	if w == nil || tnt == nil {
		return
	}
	if !w.entities.Add(tnt.Entity) {
		return
	}
	w.tnts.Add(tnt)
}

// IgniteTNT replaces a loaded TNT block with a primed entity.
// The caller must run on the world tick.
func (w *World) IgniteTNT(x, y, z int, ownerID int32) bool {
	if w == nil || block.NameFromState(w.getBlockAt(x, y, z)) != "minecraft:tnt" {
		return false
	}
	w.setBlockAt(x, y, z, 0)
	w.broadcastBlockUpdate(x, y, z, 0)
	id := w.entities.AllocateID()
	tnt := entity.NewTNTEntity(id, protocol.GenerateUUID(), entity.Vec3{X: float64(x) + 0.5, Y: float64(y), Z: float64(z) + 0.5}, ownerID)
	w.SpawnTNT(tnt)
	return w.tnts.Get(id) != nil
}

func (w *World) tickItems() {
	removed := w.items.TickAll(w)
	for _, id := range removed {
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
	for _, dropped := range w.items.All() {
		if dropped.Dirty()&entity.DirtyItemStack != 0 && !dropped.Removed() && entity.IsTickingAt(w, dropped.Position()) {
			w.broadcastItemStack(dropped)
		}
	}
	if w.itemPickup == nil {
		return
	}
	for id, dropped := range w.items.All() {
		if !entity.IsTickingAt(w, dropped.Position()) {
			continue
		}
		if !dropped.CanPickup() {
			continue
		}
		for _, player := range w.tracker.Players() {
			if player.Removed() || player.IsDead() || !dropped.IsInRange(player.Position(), 1.5) {
				continue
			}
			stack := dropped.Stack()
			count := w.itemPickup(player, stack)
			if count <= 0 || count > stack.ItemCount {
				continue
			}
			for _, observer := range w.tracker.Players() {
				if !observer.Removed() && observer.Session() != nil {
					_ = observer.Session().Send(&playpacket.CollectItem{CollectedEntityID: id, CollectorEntityID: player.ID(), PickupItemCount: count})
				}
			}
			stack.ItemCount -= count
			if stack.Empty() {
				w.items.Remove(id)
				w.entities.Remove(id)
				w.broadcastRemoveEntity(id)
			} else {
				dropped.SetStack(stack)
				data := metadata.New()
				data.SetSlot(metadata.IndexItemEntityItem, stack)
				packet := &playpacket.SetEntityMetadata{EntityID: id, Metadata: data.Encode()}
				for _, observer := range w.tracker.Players() {
					if !observer.Removed() && observer.Session() != nil {
						_ = observer.Session().Send(packet)
					}
				}
			}
			break
		}
	}
}

func (w *World) tickXPOrbs() {
	findPlayer := func(pos entity.Vec3, radius float64) (int32, entity.Vec3, bool) {
		players := w.tracker.Players()
		bestDist := radius * radius
		var bestID int32 = -1
		var bestPos entity.Vec3
		for _, p := range players {
			if p.Removed() || p.IsDead() || p.Living().GameMode() == 3 || !p.HasClientLoaded() {
				continue
			}
			pp := p.Position()
			dx := pp.X - pos.X
			dy := pp.Y - pos.Y
			dz := pp.Z - pos.Z
			dist := dx*dx + dy*dy + dz*dz
			if dist < bestDist {
				bestDist = dist
				bestID = p.ID()
				bestPos = pp
			}
		}
		if bestID >= 0 {
			return bestID, bestPos, true
		}
		return -1, entity.Vec3{}, false
	}

	removed := w.xpOrbs.TickAll(w, findPlayer)
	for _, id := range removed {
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}

	w.pickupXPOrbs()
}

func (w *World) pickupXPOrbs() {
	players := w.tracker.Players()
	if len(players) == 0 {
		return
	}

	var pickedUp []int32
	for id, orb := range w.xpOrbs.All() {
		if orb.Removed() || !entity.IsTickingAt(w, orb.Position()) {
			continue
		}
		for _, p := range players {
			if p.Removed() || !p.CanPickupXP() || !p.HasClientLoaded() {
				continue
			}
			if !orb.IsInPickupRange(p.Position()) {
				continue
			}

			living := p.Living()
			xp := w.repairWithXP(p, orb.TakeOne())
			w.broadcastTrackedPacket(id, &playpacket.CollectItem{CollectedEntityID: id, CollectorEntityID: p.ID(), PickupItemCount: 1})
			p.DelayXPPickup()
			result := experience.AddXP(living.XPLevel(), living.XPTotal(), xp)
			living.SetXP(result.Level, result.Total, result.Bar)

			sess := p.Session()
			if sess != nil {
				_ = sess.Send(&playpacket.SetExperience{
					ExperienceBar:   result.Bar,
					ExperienceLevel: result.Level,
					TotalExperience: result.Total,
				})
			}

			if orb.Removed() {
				pickedUp = append(pickedUp, id)
			}
			break
		}
	}

	for _, id := range pickedUp {
		w.xpOrbs.Remove(id)
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
}

func (w *World) tickTNT() {
	exploded, removed := w.tnts.TickAll(w)

	for _, id := range removed {
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}

	for _, id := range exploded {
		tnt := w.entities.Get(id)
		if tnt == nil {
			continue
		}
		pos := tnt.Position()
		w.doExplosion(pos.X, pos.Y+0.5, pos.Z, 4.0)
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
}

// doExplosion performs a full explosion: destroys blocks, damages entities, broadcasts packet.
func (w *World) doExplosion(x, y, z, power float64) {
	w.doExplosionWithBlocks(x, y, z, power, true)
}

// doExplosionWithBlocks applies entity effects while optionally preserving blocks.
func (w *World) doExplosionWithBlocks(x, y, z, power float64, destroyBlocks bool) {
	w.doExplosionWithSource(x, y, z, power, destroyBlocks, nil)
}

// doExplosionWithSource preserves a structured cause across all entity effects.
func (w *World) doExplosionWithSource(x, y, z, power float64, destroyBlocks bool, cause *entity.DamageSource) {
	finder := func(box physics.AABB) []entity.ExplosionTarget {
		var targets []entity.ExplosionTarget
		for _, p := range w.tracker.Players() {
			if p.Removed() || p.IsDead() {
				continue
			}
			pos := p.Position()
			hw := playerWidth / 2
			pBox := physics.AABB{
				MinX: pos.X - hw, MinY: pos.Y, MinZ: pos.Z - hw,
				MaxX: pos.X + hw, MaxY: pos.Y + playerHeight, MaxZ: pos.Z + hw,
			}
			if box.Intersects(pBox) {
				targets = append(targets, entity.ExplosionTarget{
					EntityID: p.ID(), X: pos.X, Y: pos.Y, Z: pos.Z,
					Width: playerWidth, Height: playerHeight,
				})
			}
		}
		if w.mobs != nil {
			for _, mob := range w.mobs.Mobs() {
				if mob.Entity.Removed() || mob.IsDead() {
					continue
				}
				pos := mob.Position()
				def := mob.TypeDef()
				hw := def.Width / 2
				mBox := physics.AABB{
					MinX: pos.X - hw, MinY: pos.Y, MinZ: pos.Z - hw,
					MaxX: pos.X + hw, MaxY: pos.Y + def.Height, MaxZ: pos.Z + hw,
				}
				if box.Intersects(mBox) {
					targets = append(targets, entity.ExplosionTarget{
						EntityID: mob.ID(), X: pos.X, Y: pos.Y, Z: pos.Z,
						Width: def.Width, Height: def.Height,
					})
				}
			}
		}
		return targets
	}

	exp := entity.ComputeExplosion(w, x, y, z, power, finder)

	for _, bp := range exp.AffectedBlocks {
		if !destroyBlocks {
			continue
		}
		state := w.GetBlockStateAt(bp[0], bp[1], bp[2])
		w.setBlockAt(bp[0], bp[1], bp[2], 0)
		w.broadcastBlockUpdate(bp[0], bp[1], bp[2], 0)

		if block.NameFromState(state) == "minecraft:tnt" {
			eid := w.entities.AllocateID()
			uuid := protocol.GenerateUUID()
			fuse := int32(10 + rand.Intn(15))
			tnt := entity.NewTNTEntityWithFuse(eid, uuid,
				entity.Vec3{X: float64(bp[0]) + 0.5, Y: float64(bp[1]), Z: float64(bp[2]) + 0.5},
				0, fuse,
			)
			w.SpawnTNT(tnt)
		}
	}

	for _, eff := range exp.AffectedEntities {
		w.applyExplosionDamageSource(eff, cause)
	}

	w.broadcastExplosion(exp)
}

func (w *World) applyExplosionDamage(eff entity.ExplosionEntityEffect) {
	w.applyExplosionDamageSource(eff, nil)
}

// applyExplosionDamageSource applies armor, attribution and authoritative knockback.
func (w *World) applyExplosionDamageSource(eff entity.ExplosionEntityEffect, cause *entity.DamageSource) {
	dmg := float32(eff.Damage)
	if dmg <= 0 {
		return
	}
	src := entity.NewDamageSource(entity.DamageTypeExplosion, dmg)
	if cause != nil {
		copied := *cause
		copied.Amount = dmg
		src = &copied
	}

	for _, p := range w.tracker.Players() {
		if p.ID() != eff.EntityID || p.Removed() || p.IsDead() {
			continue
		}
		living := p.Living()
		actual := living.ApplyDamage(w.playerDifficultyDamage(src), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
		if actual > 0 {
			velocity := p.Velocity()
			velocity.X += eff.KnockX
			velocity.Y += eff.KnockY
			velocity.Z += eff.KnockZ
			p.SetVelocity(velocity)
		}
		sess := p.Session()
		if sess == nil {
			return
		}
		if actual > 0 {
			_ = sess.Send(&playpacket.UpdateHealth{
				Health:         living.Health(),
				Food:           living.FoodLevel(),
				FoodSaturation: living.FoodSaturation(),
			})
			hurtPkt := &playpacket.HurtAnimation{EntityID: p.ID(), Yaw: 0}
			for _, op := range w.tracker.Players() {
				if op.Removed() {
					continue
				}
				if s := op.Session(); s != nil {
					_ = s.Send(hurtPkt)
				}
			}
			if living.IsDead() {
				msg := formatDeathMessage(src.Type, p.Username(), "")
				_ = sess.Send(&playpacket.DeathCombatEvent{
					PlayerID: p.ID(),
					Message:  msg,
				})
			}
		}
		return
	}

	if w.mobs != nil {
		mob := w.mobs.Get(eff.EntityID)
		if mob != nil && !mob.IsDead() {
			if mob.ApplyDamage(src, mob.ArmorDefense(), mob.ArmorToughness(), mob.ArmorEnchProtection()) <= 0 {
				return
			}
			velocity := mob.Velocity()
			resistance := 1 - mob.KnockbackResistance()
			velocity.X += eff.KnockX * resistance
			velocity.Y += eff.KnockY * resistance
			velocity.Z += eff.KnockZ * resistance
			mob.SetVelocity(velocity)
			hurtPkt := &playpacket.HurtAnimation{EntityID: mob.ID(), Yaw: 0}
			for _, p := range w.tracker.Players() {
				if p.Removed() {
					continue
				}
				if s := p.Session(); s != nil {
					_ = s.Send(hurtPkt)
				}
			}
		}
	}
}

func (w *World) broadcastExplosion(exp *entity.Explosion) {
	for _, p := range w.tracker.Players() {
		if p.Removed() {
			continue
		}
		sess := p.Session()
		if sess == nil {
			continue
		}

		var motX, motY, motZ float64
		hasMotion := false
		for _, eff := range exp.AffectedEntities {
			if eff.EntityID == p.ID() {
				motX = eff.KnockX
				motY = eff.KnockY
				motZ = eff.KnockZ
				hasMotion = true
				break
			}
		}

		_ = sess.Send(&playpacket.Explosion{
			X:               exp.X,
			Y:               exp.Y,
			Z:               exp.Z,
			PlayerMotionX:   motX,
			PlayerMotionY:   motY,
			PlayerMotionZ:   motZ,
			HasPlayerMotion: hasMotion,
			Large:           exp.Power >= 2,
		})
	}
}

// ProjectileManager returns the world's projectile manager.
func (w *World) ProjectileManager() *projectile.Manager {
	if w == nil {
		return nil
	}
	return w.projectiles
}

// SpawnArrow creates and registers an arrow projectile in the world.
func (w *World) SpawnArrow(a *projectile.Arrow) {
	if w == nil || a == nil {
		return
	}
	if !w.entities.Add(a.Entity) {
		return
	}
	w.projectiles.AddArrow(a)
}

// SpawnThrown creates and registers a thrown projectile in the world.
func (w *World) SpawnThrown(t *projectile.ThrownEntity) {
	if w == nil || t == nil {
		return
	}
	if !w.entities.Add(t.Entity) {
		return
	}
	w.projectiles.AddThrown(t)
}

// SpawnTrident creates and registers a trident projectile in the world.
func (w *World) SpawnTrident(t *projectile.Trident) {
	if w == nil || t == nil {
		return
	}
	if !w.entities.Add(t.Entity) {
		return
	}
	w.projectiles.AddTrident(t)
}

func (w *World) tickProjectiles() {
	finder := &worldEntityFinder{tracker: w.tracker, mobs: w.mobs}
	result := w.projectiles.TickAll(w, finder)

	// Process arrow entity hits
	for _, hit := range result.ArrowHits {
		if !hit.HitResult.HitEntity {
			continue
		}
		arrow := w.projectiles.GetArrow(hit.ArrowID)
		if arrow == nil {
			continue
		}
		dmg := float32(arrow.ComputeHitDamage())
		if dmg < 0.5 {
			dmg = 0.5
		}
		src := entity.NewAttackDamageSource(entity.DamageTypeProjectile, dmg, arrow.OwnerID())
		src.DirectID, src.DirectType = arrow.ID(), "minecraft:arrow"
		w.applyProjectileDamage(hit.HitResult.EntityID, src, arrow.OwnerID())
		arrow.Entity.Remove()
	}

	// Process thrown entity hits
	for _, hit := range result.ThrownHits {
		switch hit.ThrownType {
		case projectile.ThrownSnowball, projectile.ThrownEgg:
			if hit.HitResult.HitEntity {
				src := entity.NewAttackDamageSource(entity.DamageTypeProjectile, 0, -1)
				w.applyProjectileDamage(hit.HitResult.EntityID, src, -1)
			}
		case projectile.ThrownEnderPearl:
			thrown := w.projectiles.GetThrown(hit.ThrownID)
			if thrown != nil {
				w.teleportOwnerToHit(thrown.OwnerID(), hit.HitResult)
			}
		case projectile.ThrownSplashPotion:
			thrown := w.projectiles.GetThrown(hit.ThrownID)
			if thrown != nil {
				w.applySplashPotionArea(hit.HitResult, thrown.ItemID())
			}
		}
	}

	// Process trident entity hits
	for _, hit := range result.TridentHits {
		if !hit.HitResult.HitEntity {
			continue
		}
		tr := w.projectiles.GetTrident(hit.TridentID)
		if tr == nil {
			continue
		}
		dmg := float32(tr.ComputeHitDamage())
		src := entity.NewAttackDamageSource(entity.DamageTypeProjectile, dmg, tr.OwnerID())
		src.DirectID, src.DirectType = tr.ID(), "minecraft:trident"
		w.applyProjectileDamage(hit.HitResult.EntityID, src, tr.OwnerID())
	}

	for _, id := range result.RemovedIDs {
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
	w.projectiles.FinalizeRemovals(result.RemovedIDs)
	w.syncArrowStates()
}

// applyProjectileDamage applies damage from a projectile to a target entity.
func (w *World) applyProjectileDamage(targetID int32, src *entity.DamageSource, ownerID int32) {
	if src == nil {
		return
	}
	for _, player := range w.tracker.Players() {
		if player.ID() == ownerID && !player.Removed() {
			src.PlayerCredit = true
			src.AttackerType = "minecraft:player"
			if w.playerInventory != nil {
				if windows := w.playerInventory(player); windows != nil {
					src.Weapon = windows.HeldItem().Clone()
				}
			}
			break
		}
	}
	if attacker := w.mobs.Get(ownerID); attacker != nil {
		src.AttackerType = "minecraft:" + attacker.TypeName()
	}
	// Check if target is a player
	for _, p := range w.tracker.Players() {
		if p.ID() != targetID || p.Removed() || p.IsDead() {
			continue
		}
		living := p.Living()
		if w.projectileBlocked(p, src) {
			return
		}
		actual := living.ApplyDamage(w.playerDifficultyDamage(src), living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
		if actual > 0 {
			w.applyArrowHitEffects(src, living)
		}
		sess := p.Session()
		if sess == nil {
			return
		}
		if actual > 0 || src.Amount == 0 {
			_ = sess.Send(&playpacket.UpdateHealth{
				Health:         living.Health(),
				Food:           living.FoodLevel(),
				FoodSaturation: living.FoodSaturation(),
			})
			// Broadcast hurt animation
			hurtPkt := &playpacket.HurtAnimation{EntityID: p.ID(), Yaw: 0}
			for _, op := range w.tracker.Players() {
				if op.Removed() {
					continue
				}
				if s := op.Session(); s != nil {
					_ = s.Send(hurtPkt)
				}
			}
			if living.IsDead() {
				attackerName := ""
				if ownerID > 0 {
					for _, ap := range w.tracker.Players() {
						if ap.ID() == ownerID {
							attackerName = ap.Username()
							break
						}
					}
				}
				msg := formatDeathMessage(src.Type, p.Username(), attackerName)
				_ = sess.Send(&playpacket.DeathCombatEvent{
					PlayerID: p.ID(),
					Message:  msg,
				})
			}
		}
		return
	}

	// Check if target is a mob
	if w.mobs != nil {
		mob := w.mobs.Get(targetID)
		if mob != nil && !mob.Removed() && !mob.IsDead() && entity.IsTickingAt(w, mob.Position()) {
			if mob.ApplyDamage(src, mob.ArmorDefense(), mob.ArmorToughness(), mob.ArmorEnchProtection()) <= 0 {
				return
			}
			w.applyArrowHitEffects(src, mob.LivingEntity)
			hurtPkt := &playpacket.HurtAnimation{EntityID: mob.ID(), Yaw: 0}
			w.broadcastTrackedPacket(mob.ID(), hurtPkt)
		}
	}
}

// teleportOwnerToHit teleports the projectile owner to the hit location (ender pearl).
func (w *World) teleportOwnerToHit(ownerID int32, hr *projectile.HitResult) {
	if hr == nil {
		return
	}
	for _, p := range w.tracker.Players() {
		if p.ID() != ownerID || p.Removed() {
			continue
		}
		newPos := entity.Vec3{X: hr.HitX, Y: hr.HitY, Z: hr.HitZ}
		p.SetPosition(newPos)
		sess := p.Session()
		if sess != nil {
			_ = sess.Send(&playpacket.SyncPlayerPosition{
				X: newPos.X, Y: newPos.Y, Z: newPos.Z,
				Yaw: float32(p.Rotation().X), Pitch: float32(p.Rotation().Y),
			})
			// Ender pearls deal 5 fall damage
			living := p.Living()
			src := entity.NewDamageSource(entity.DamageTypeFall, 5.0)
			actual := living.ApplyDamage(src, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
			if actual > 0 {
				_ = sess.Send(&playpacket.UpdateHealth{
					Health:         living.Health(),
					Food:           living.FoodLevel(),
					FoodSaturation: living.FoodSaturation(),
				})
			}
		}
		return
	}
}

// applySplashPotionArea applies splash potion effects to all entities within range.
func (w *World) applySplashPotionArea(hr *projectile.HitResult, potionID int32) {
	if hr == nil {
		return
	}
	pe := effect.PotionEffects(potionID)
	if pe == nil {
		return
	}
	splashEffects := effect.ScaleEffects(pe.Effects, effect.SplashDurationFactor())
	center := entity.Vec3{X: hr.HitX, Y: hr.HitY, Z: hr.HitZ}
	const splashRadius = 4.0
	const splashRadiusSq = splashRadius * splashRadius

	for _, p := range w.tracker.Players() {
		if p.Removed() || p.IsDead() {
			continue
		}
		pos := p.Position()
		dx := pos.X - center.X
		dy := pos.Y - center.Y
		dz := pos.Z - center.Z
		distSq := dx*dx + dy*dy + dz*dz
		if distSq > splashRadiusSq {
			continue
		}
		dist := math.Sqrt(distSq)
		proximity := 1.0 - dist/splashRadius
		if proximity < 0.25 {
			proximity = 0.25
		}
		living := p.Living()
		sess := p.Session()
		for _, e := range splashEffects {
			if effect.IsInstant(e.ID) {
				action := effect.ApplyInstant(e.ID, e.Amplifier)
				if action != nil {
					if action.Heal > 0 {
						action.Heal *= float32(proximity)
					}
					if action.Damage > 0 {
						action.Damage *= float32(proximity)
					}
					if action.Heal > 0 && !living.IsDead() {
						h := living.Health() + action.Heal
						if h > living.MaxHealth() {
							h = living.MaxHealth()
						}
						living.SetHealth(h)
					}
					if action.Damage > 0 {
						living.Damage(action.Damage, "magic")
					}
				}
			} else {
				scaled := e
				scaled.Duration = int32(float64(e.Duration) * proximity)
				if scaled.Duration < 1 {
					scaled.Duration = 1
				}
				living.Effects().Add(scaled)
				if sess != nil {
					_ = sess.Send(&playpacket.EntityEffect{
						EntityID:  p.ID(),
						EffectID:  scaled.ID,
						Amplifier: scaled.Amplifier,
						Duration:  scaled.Duration,
						Flags:     scaled.Flags,
					})
				}
			}
		}
		if sess != nil {
			_ = sess.Send(&playpacket.UpdateHealth{
				Health:         living.Health(),
				Food:           living.FoodLevel(),
				FoodSaturation: living.FoodSaturation(),
			})
		}
	}
}

func (w *World) tickMobs() {
	if w.mobs == nil {
		return
	}

	// Natural mob spawning
	if w.spawner != nil {
		playerPositions := w.buildSpawnPlayerList()
		requests := w.spawner.Tick(w.mobs, playerPositions, w)
		for _, req := range requests {
			mob := w.mobs.SpawnMob(req.TypeName, entity.Vec3{X: req.X, Y: req.Y, Z: req.Z})
			if mob != nil && req.SlimeSize > 0 {
				mob.SetSlimeSize(req.SlimeSize, true)
			}
			if mob != nil {
				w.initializeMobPickup(mob)
			}
		}
	}

	if w.mobBrains != nil {
		players := w.buildAIPlayerList()
		w.mobBrains.TickAll(w.mobs.Mobs(), players, w.tick.Load(), w)
	}

	w.pickupMobEquipment()
	drops := w.mobs.Tick(w)
	w.syncMobStates()
	for _, d := range drops {
		pos := entity.Vec3{X: d.X, Y: d.Y, Z: d.Z}
		if d.XP > 0 {
			w.AwardExperience(pos, d.XP)
		}
		stack := d.Stack
		if stack.Empty() && d.ItemID > 0 && d.Count > 0 {
			stack = inventory.NewSlot(d.ItemID, d.Count)
		}
		if !stack.Empty() && stack.ItemID > 0 {
			eid := w.entities.AllocateID()
			uuid := protocol.GenerateUUID()
			ie := entity.NewItemEntity(eid, uuid,
				pos,
				stack,
			)
			w.SpawnItemEntity(ie)
		}
	}
}

// GetBlock returns the block state at the given coordinates (implements spawning.BlockAccess).
func (w *World) GetBlock(x, y, z int) int32 {
	return w.getBlockAt(x, y, z)
}

func (w *World) buildSpawnPlayerList() []spawning.PlayerPos {
	trackedPlayers := w.tracker.Players()
	if len(trackedPlayers) == 0 {
		return nil
	}
	result := make([]spawning.PlayerPos, 0, len(trackedPlayers))
	for _, p := range trackedPlayers {
		if p.Removed() || p.IsDead() || !p.HasClientLoaded() || p.Living().GameMode() == 3 {
			continue
		}
		pos := p.Position()
		result = append(result, spawning.PlayerPos{X: pos.X, Y: pos.Y, Z: pos.Z})
	}
	return result
}

func (w *World) buildAIPlayerList() []ai.PlayerInfo {
	trackedPlayers := w.tracker.Players()
	if len(trackedPlayers) == 0 {
		return nil
	}
	result := make([]ai.PlayerInfo, 0, len(trackedPlayers))
	for _, p := range trackedPlayers {
		if p.Removed() || p.IsDead() || !p.HasClientLoaded() {
			continue
		}
		result = append(result, ai.PlayerInfo{
			EntityID: p.ID(),
			Pos:      p.Position(),
			GameMode: p.Living().GameMode(),
		})
		if w.playerInventory != nil {
			if windows := w.playerInventory(p); windows != nil {
				result[len(result)-1].HeldItem = item.NameByID(windows.HeldItem().ItemID)
				result[len(result)-1].OffhandItem = item.NameByID(windows.Inventory().Get(inventory.OffhandSlot).ItemID)
			}
		}
	}
	return result
}

// MobManager returns the world's mob manager.
func (w *World) MobManager() *entity.MobManager {
	if w == nil {
		return nil
	}
	return w.mobs
}

// SummonMob spawns a mob entity at the given float64 coordinates.
func (w *World) SummonMob(typeName string, x, y, z float64) {
	w.SpawnMob(typeName, entity.Vec3{X: x, Y: y, Z: z})
}

// SpawnMob creates and registers a mob entity in the world by type name.
func (w *World) SpawnMob(typeName string, pos entity.Vec3) *entity.MobEntity {
	if w == nil || w.mobs == nil {
		return nil
	}
	return w.mobs.SpawnMob(typeName, pos)
}

func makeChunkSaveFunc(store *persistence.ChunkStore) chunk.ChunkSaveFunc {
	if store == nil {
		return nil
	}
	return func(c *chunk.Chunk) error {
		if c == nil || c.Data() == nil {
			return nil
		}
		entities, err := c.EntityNBT()
		if err != nil {
			return fmt.Errorf("snapshot entities (%d,%d): %w", c.X(), c.Z(), err)
		}
		data, err := chunk.EncodeChunkNBT(c)
		if err != nil {
			return fmt.Errorf("encode chunk (%d,%d): %w", c.X(), c.Z(), err)
		}
		if err := store.WriteChunkNBT(int(c.X()), int(c.Z()), data); err != nil {
			return fmt.Errorf("write chunk (%d,%d): %w", c.X(), c.Z(), err)
		}
		if entities != nil {
			if err := store.WriteEntityNBT(int(c.X()), int(c.Z()), entities); err != nil {
				return fmt.Errorf("write entities (%d,%d): %w", c.X(), c.Z(), err)
			}
		}
		return nil
	}
}

func (w *World) notifyNeighbors(x, y, z int) {
	w.NotifyNeighbors(x, y, z)
}
