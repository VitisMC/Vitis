package world

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/ai"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/experience"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/spawning"
	"github.com/vitismc/vitis/internal/world/streaming"
	"github.com/vitismc/vitis/internal/world/tick"
	"github.com/vitismc/vitis/internal/world/weather"
)

const (
	defaultSchedulerQueueCapacity   = 8192
	defaultSchedulerDrainPerTick    = 1024
	defaultChunkCompletionsPerTick  = 256
	defaultChunkUnloadBatchPerTick  = 64
	defaultWorldChunkStorageInitial = 2048
	defaultChunksPerTick            = 4
)

// Config defines world construction settings.
type Config struct {
	ID uint64

	Name string

	ChunkManager *chunk.Manager
	Generator    chunk.Generator
	ChunkStore   *persistence.ChunkStore

	WorldDir string
	Seed     int64
	BiomeID  int32

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
}

// World stores world-owned simulation state and chunk subsystem entry points.
type World struct {
	id   uint64
	name string

	chunks     *chunk.Manager
	chunkStore *persistence.ChunkStore
	streaming  *streaming.Manager

	scheduler     *Scheduler
	tickScheduler *tick.Scheduler
	entities      *entity.Manager
	tracker       *entity.Tracker
	fallingBlocks *entity.FallingBlockManager
	items         *entity.ItemEntityManager
	xpOrbs        *entity.XPOrbManager
	tnts          *entity.TNTManager
	projectiles   *projectile.Manager
	weather       *weather.Weather
	mobs          *entity.MobManager
	mobBrains     *ai.BrainRegistry
	spawner       *spawning.Spawner

	tick atomic.Uint64

	worldAge  int64
	timeOfDay int64

	schedulerDrainPerTick    int
	chunkCompletionsPerTick  int
	chunkUnloadBatchPerTick  int
	chunkStreamChunksPerTick int
}

// New creates a world with self-contained internal defaults.
func New(config Config) (*World, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("create world: empty world name")
	}

	chunkManager := config.ChunkManager
	if chunkManager == nil {
		generator := config.Generator
		if generator == nil {
			terrainGen := chunk.NewTerrainGenerator(config.Seed, config.BiomeID)
			if config.ChunkStore != nil {
				generator = chunk.NewPersistentGenerator(config.ChunkStore, terrainGen, config.BiomeID)
			} else {
				generator = terrainGen
			}
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

	entMgr := entity.NewManager()
	w := &World{
		id:                       config.ID,
		name:                     config.Name,
		chunks:                   chunkManager,
		chunkStore:               config.ChunkStore,
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
	}
	w.mobs = entity.NewMobManager(entMgr, entMgr.AllocateID)
	w.mobBrains = ai.NewBrainRegistry()
	w.spawner = spawning.NewSpawnerWithConfig(spawning.Config{
		Seed:       config.Seed,
		SpawnTable: spawning.DefaultOverworldSpawns,
	})
	return w, nil
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

// Tick advances world simulation by one tick.
func (w *World) Tick() {
	if w == nil {
		return
	}

	newTick := w.tick.Add(1)
	w.chunks.SetTick(newTick)

	w.worldAge++
	w.timeOfDay++
	if w.timeOfDay >= 24000 {
		w.timeOfDay = 0
	}

	w.tickWeather()
	w.scheduler.Drain(w.schedulerDrainPerTick)
	w.processScheduledTicks()
	w.tickFallingBlocks()
	w.tickItems()
	w.tickXPOrbs()
	w.tickTNT()
	w.tickProjectiles()
	w.tickMobs()
	w.chunks.PumpLoadRequests()
	w.chunks.ApplyLoadCompletions(w.chunkCompletionsPerTick)
	w.chunks.UpdateActiveChunks()
	w.tickStreaming()
	w.tickPlayers()
	w.entities.Tick(w.tick.Load())
	w.tracker.Tick(w.entities)
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
func (w *World) AddEntity(e *entity.Entity) {
	if w == nil || e == nil {
		return
	}
	w.entities.Add(e)
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
	w.entities.Add(p.Entity)
	w.tracker.AddPlayer(p)

	s := w.streaming.AddPlayer(p.ID(), streaming.StreamerConfig{
		ChunksPerTick: w.chunkStreamChunksPerTick,
		ViewDistance:  p.ViewDistance(),
		Provider:      w.chunks,
		Sender:        p.Session(),
	})
	s.InitialLoad(p.ChunkX(), p.ChunkZ())
}

// RemovePlayer removes a player from the world and tracking.
func (w *World) RemovePlayer(p *entity.Player) {
	if w == nil || p == nil {
		return
	}
	w.streaming.RemovePlayer(p.ID(), p.Session())
	w.entities.Remove(p.ID())
	w.tracker.RemovePlayer(p.ID())
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

// Schedule enqueues one world-thread task and blocks when ingress queue is full.
func (s *Scheduler) Schedule(task func()) {
	if s == nil || task == nil {
		return
	}
	s.queue <- task
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
				return p.ChunkX(), p.ChunkZ(), true
			}
		}
		return 0, 0, false
	})

	w.streaming.ForEachLoadedChunk(func(pos streaming.ChunkPos) {
		w.chunks.Touch(pos.X, pos.Z)
	})
}

// WorldAge returns the total age of the world in ticks.
func (w *World) WorldAge() int64 {
	if w == nil {
		return 0
	}
	return w.worldAge
}

// TimeOfDay returns the current time of day (0-23999).
func (w *World) TimeOfDay() int64 {
	if w == nil {
		return 0
	}
	return w.timeOfDay
}

// MinY returns the minimum build height for this world.
func (w *World) MinY() int {
	return section.OverworldMinY
}

// MaxY returns the maximum build height for this world.
func (w *World) MaxY() int {
	return section.OverworldMaxY
}

// SetTimeOfDay sets the current time of day.
func (w *World) SetTimeOfDay(t int64) {
	if w == nil {
		return
	}
	w.timeOfDay = t % 24000
	if w.timeOfDay < 0 {
		w.timeOfDay += 24000
	}
}

func (w *World) tickPlayers() {
	players := w.tracker.Players()
	for _, p := range players {
		if p.Removed() {
			continue
		}
		living := p.Living()
		living.TickLiving()
		living.TickHunger()

		diff := living.TickEffects()
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
				_ = sess.Send(&playpacket.UpdateHealth{
					Health:         living.Health(),
					Food:           living.FoodLevel(),
					FoodSaturation: living.FoodSaturation(),
				})
				_ = sess.Send(&playpacket.HurtAnimation{
					EntityID: p.ID(),
					Yaw:      0,
				})
				if living.IsDead() {
					name := p.Username()
					msg := formatDeathMessage(src.Type, name, "")
					_ = sess.Send(&playpacket.DeathCombatEvent{
						PlayerID: p.ID(),
						Message:  msg,
					})
				}
			}
		}

		if w.tick.Load()%20 == 0 && !living.IsDead() {
			if sess == nil {
				continue
			}
			_ = sess.Send(&playpacket.UpdateHealth{
				Health:         living.Health(),
				Food:           living.FoodLevel(),
				FoodSaturation: living.FoodSaturation(),
			})
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
		_ = sess.Send(&playpacket.UpdateTime{WorldAge: w.worldAge, TimeOfDay: w.timeOfDay, TickDayTime: true})
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
func (w *World) SaveAllDirty() int {
	if w == nil || w.chunks == nil {
		return 0
	}
	return w.chunks.SaveAllDirty()
}

// Close saves dirty chunks and releases world-owned async resources.
func (w *World) Close(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.SaveAllDirty()
	return w.chunks.Close(ctx)
}

func (w *World) processScheduledTicks() {
	ticks := w.tickScheduler.Drain(w.tick.Load(), w.schedulerDrainPerTick)
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

	if result.SetBlock && result.NewBlockState > 0 {
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
	w.entities.Add(fb.Entity)
	w.fallingBlocks.Add(fb)
}

func (w *World) tickFallingBlocks() {
	var toRemove []int32
	for id, fb := range w.fallingBlocks.All() {
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

func (w *World) getBlockAt(x, y, z int) int32 {
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := w.chunks.GetChunk(cx, cz)
	if !ok || c == nil {
		return 0
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
	c.SetBlock(x, y, z, stateID)
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
		w.notifyNeighbors(t.X, t.Y, t.Z)
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
		w.notifyNeighbors(t.X, t.Y, t.Z)
	}

	flowResult := fluid.ComputeFlow(t.X, t.Y, t.Z, *config, w.getBlockAt)
	for pos, state := range flowResult.NewStates {
		existingState := w.getBlockAt(pos[0], pos[1], pos[2])
		if fluid.CanBeReplacedByFluid(existingState, config.FluidType) {
			w.setBlockAt(pos[0], pos[1], pos[2], state)
			w.broadcastBlockUpdate(pos[0], pos[1], pos[2], state)
			w.scheduleFluidTick(pos[0], pos[1], pos[2], config.FlowSpeed)
			w.notifyNeighbors(pos[0], pos[1], pos[2])
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
	w.entities.Add(item.Entity)
	w.items.Add(item)
}

// SpawnXPOrb creates and registers an XP orb entity in the world.
func (w *World) SpawnXPOrb(orb *entity.XPOrb) {
	if w == nil || orb == nil {
		return
	}
	w.entities.Add(orb.Entity)
	w.xpOrbs.Add(orb)
}

// SpawnTNT creates and registers a primed TNT entity in the world.
func (w *World) SpawnTNT(tnt *entity.TNTEntity) {
	if w == nil || tnt == nil {
		return
	}
	w.entities.Add(tnt.Entity)
	w.tnts.Add(tnt)
}

func (w *World) tickItems() {
	removed := w.items.TickAll(w)
	for _, id := range removed {
		w.broadcastRemoveEntity(id)
	}
}

func (w *World) tickXPOrbs() {
	findPlayer := func(pos entity.Vec3, radius float64) (int32, entity.Vec3, bool) {
		players := w.tracker.Players()
		bestDist := radius * radius
		var bestID int32 = -1
		var bestPos entity.Vec3
		for _, p := range players {
			if p.Removed() {
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
		if orb.Removed() {
			continue
		}
		for _, p := range players {
			if p.Removed() {
				continue
			}
			if !orb.IsInPickupRange(p.Position()) {
				continue
			}

			living := p.Living()
			result := experience.AddXP(living.XPLevel(), living.XPTotal(), orb.XPValue())
			living.SetXP(result.Level, result.Total, result.Bar)

			sess := p.Session()
			if sess != nil {
				_ = sess.Send(&playpacket.SetExperience{
					ExperienceBar:   result.Bar,
					ExperienceLevel: result.Level,
					TotalExperience: result.Total,
				})
			}

			orb.Remove()
			pickedUp = append(pickedUp, id)
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

	const tntMinState int32 = 2137
	const tntMaxState int32 = 2138

	for _, bp := range exp.AffectedBlocks {
		state := w.GetBlockStateAt(bp[0], bp[1], bp[2])
		w.setBlockAt(bp[0], bp[1], bp[2], 0)
		w.broadcastBlockUpdate(bp[0], bp[1], bp[2], 0)

		if state >= tntMinState && state <= tntMaxState {
			eid := w.entities.AllocateID()
			uuid := protocol.UUID{uint64(eid), uint64(eid) ^ 0xBEEF}
			fuse := int32(10 + rand.Intn(15))
			tnt := entity.NewTNTEntityWithFuse(eid, uuid,
				entity.Vec3{X: float64(bp[0]) + 0.5, Y: float64(bp[1]), Z: float64(bp[2]) + 0.5},
				0, fuse,
			)
			w.SpawnTNT(tnt)
		}
	}

	for _, eff := range exp.AffectedEntities {
		w.applyExplosionDamage(eff)
	}

	w.broadcastExplosion(exp)
}

func (w *World) applyExplosionDamage(eff entity.ExplosionEntityEffect) {
	dmg := float32(eff.Damage)
	if dmg <= 0 {
		return
	}
	src := entity.NewDamageSource(entity.DamageTypeExplosion, dmg)

	for _, p := range w.tracker.Players() {
		if p.ID() != eff.EntityID || p.Removed() || p.IsDead() {
			continue
		}
		living := p.Living()
		actual := living.ApplyDamage(src, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
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
			mob.Damage(dmg, "explosion")
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
	records := make([]byte, 0, len(exp.AffectedBlocks)*3)
	cx, cy, cz := int(math.Floor(exp.X)), int(math.Floor(exp.Y)), int(math.Floor(exp.Z))
	for _, bp := range exp.AffectedBlocks {
		records = append(records, byte(bp[0]-cx), byte(bp[1]-cy), byte(bp[2]-cz))
	}

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
			X:                exp.X,
			Y:                exp.Y,
			Z:                exp.Z,
			Strength:         float32(exp.Power),
			RecordCount:      int32(len(exp.AffectedBlocks)),
			Records:          records,
			PlayerMotionX:    motX,
			PlayerMotionY:    motY,
			PlayerMotionZ:    motZ,
			HasPlayerMotion:  hasMotion,
			BlockInteraction: 1,
			SoundName:        "minecraft:entity.generic.explode",
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
	w.entities.Add(a.Entity)
	w.projectiles.AddArrow(a)
}

// SpawnThrown creates and registers a thrown projectile in the world.
func (w *World) SpawnThrown(t *projectile.ThrownEntity) {
	if w == nil || t == nil {
		return
	}
	w.entities.Add(t.Entity)
	w.projectiles.AddThrown(t)
}

// SpawnTrident creates and registers a trident projectile in the world.
func (w *World) SpawnTrident(t *projectile.Trident) {
	if w == nil || t == nil {
		return
	}
	w.entities.Add(t.Entity)
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
		w.applyProjectileDamage(hit.HitResult.EntityID, src, tr.OwnerID())
	}

	for _, id := range result.RemovedIDs {
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
}

// applyProjectileDamage applies damage from a projectile to a target entity.
func (w *World) applyProjectileDamage(targetID int32, src *entity.DamageSource, ownerID int32) {
	// Check if target is a player
	for _, p := range w.tracker.Players() {
		if p.ID() != targetID || p.Removed() || p.IsDead() {
			continue
		}
		living := p.Living()
		actual := living.ApplyDamage(src, living.ArmorDefense(), living.ArmorToughness(), living.ArmorEnchProtection())
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
		if mob != nil && !mob.IsDead() {
			mob.Damage(src.Amount, src.Type.SourceName())
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
		spawnResult := w.spawner.Tick(w.mobs, playerPositions, w)
		for _, id := range spawnResult.DespawnIDs {
			w.mobs.Remove(id)
			if w.mobBrains != nil {
				w.mobBrains.Remove(id)
			}
			w.broadcastRemoveEntity(id)
		}
		for _, req := range spawnResult.Requests {
			mob := w.mobs.SpawnMob(req.TypeName, entity.Vec3{X: req.X, Y: req.Y, Z: req.Z})
			if mob != nil && w.mobBrains != nil {
				w.mobBrains.RegisterMob(mob)
			}
		}
	}

	if w.mobBrains != nil {
		players := w.buildAIPlayerList()
		w.mobBrains.TickAll(w.mobs.Mobs(), players, w.tick.Load(), w)
	}

	drops := w.mobs.Tick(w)
	for _, d := range drops {
		if d.ItemID > 0 && d.Count > 0 {
			eid := w.entities.AllocateID()
			uuid := protocol.UUID{uint64(eid), uint64(eid) ^ 0xDEAD}
			stack := inventory.NewSlot(d.ItemID, d.Count)
			ie := entity.NewItemEntity(eid, uuid,
				entity.Vec3{X: d.X + 0.5, Y: d.Y + 0.5, Z: d.Z + 0.5},
				stack,
			)
			w.SpawnItemEntity(ie)
		}
	}
}

// GetBlock returns the block state at the given coordinates.
func (w *World) GetBlock(x, y, z int) int32 {
	return w.getBlockAt(x, y, z)
}

// GetBiome returns the biome ID at the given world coordinates.
func (w *World) GetBiome(x, y, z int) int32 {
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := w.chunks.GetChunk(cx, cz)
	if !ok || c == nil || c.GameData() == nil {
		return 0
	}
	return c.GameData().GetBiome(x, y, z)
}

// GetBlockLight returns the block light level at the given world coordinates.
func (w *World) GetBlockLight(x, y, z int) uint8 {
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := w.chunks.GetChunk(cx, cz)
	if !ok || c == nil || c.GameData() == nil || c.GameData().LightEngine == nil {
		return 0
	}
	return c.GameData().LightEngine.BlockLightAt(x&0xF, y, z&0xF)
}

// GetSkyLight returns the sky light level at the given world coordinates.
func (w *World) GetSkyLight(x, y, z int) uint8 {
	cx := int32(math.Floor(float64(x) / 16))
	cz := int32(math.Floor(float64(z) / 16))
	c, ok := w.chunks.GetChunk(cx, cz)
	if !ok || c == nil || c.GameData() == nil || c.GameData().LightEngine == nil {
		return 0
	}
	return c.GameData().LightEngine.SkyLightAt(x&0xF, y, z&0xF)
}

// IsChunkLoaded reports whether a chunk is loaded and available for simulation.
func (w *World) IsChunkLoaded(x, z int32) bool {
	_, ok := w.chunks.GetChunk(x, z)
	return ok
}

func (w *World) buildSpawnPlayerList() []spawning.PlayerInfo {
	trackedPlayers := w.tracker.Players()
	if len(trackedPlayers) == 0 {
		return nil
	}
	result := make([]spawning.PlayerInfo, 0, len(trackedPlayers))
	for _, p := range trackedPlayers {
		if p.Removed() || p.Living().IsDead() {
			continue
		}
		pos := p.Position()
		result = append(result, spawning.PlayerInfo{
			X:        pos.X,
			Y:        pos.Y,
			Z:        pos.Z,
			GameMode: p.Living().GameMode(),
			Dead:     p.Living().IsDead(),
		})
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
		if p.Removed() {
			continue
		}
		result = append(result, ai.PlayerInfo{
			EntityID: p.ID(),
			Pos:      p.Position(),
			GameMode: p.Living().GameMode(),
		})
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
	mob := w.mobs.SpawnMob(typeName, pos)
	if mob != nil && w.mobBrains != nil {
		w.mobBrains.RegisterMob(mob)
	}
	return mob
}

func makeChunkSaveFunc(store *persistence.ChunkStore) chunk.ChunkSaveFunc {
	if store == nil {
		return nil
	}
	return func(c *chunk.Chunk) {
		if c == nil || c.GameData() == nil {
			return
		}
		data, err := chunk.EncodeChunkNBT(c.GameData())
		if err != nil {
			return
		}
		_ = store.WriteChunkNBT(int(c.X()), int(c.Z()), data)
	}
}

func (w *World) notifyNeighbors(x, y, z int) {
	offsets := [][3]int{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}}
	for _, off := range offsets {
		nx, ny, nz := x+off[0], y+off[1], z+off[2]
		state := w.getBlockAt(nx, ny, nz)
		if state <= 0 {
			continue
		}

		b := behavior.GetByState(state)
		if b.ScheduleOnNeighborUpdate() {
			delay := b.TickDelay()
			if delay > 0 {
				w.ScheduleBlockTick(nx, ny, nz, delay, tick.PriorityNormal)
			}
		}

		if fluid.IsFluid(state) {
			config := fluid.GetFluidConfig(state)
			if config != nil {
				w.scheduleFluidTick(nx, ny, nz, config.FlowSpeed)
			}
		}
	}
}
