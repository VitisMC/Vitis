package world

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/block/fluid"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/streaming"
	"github.com/vitismc/vitis/internal/world/tick"
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

	return &World{
		id:                       config.ID,
		name:                     config.Name,
		chunks:                   chunkManager,
		chunkStore:               config.ChunkStore,
		streaming:                streaming.NewManager(),
		scheduler:                NewScheduler(schedulerCapacity),
		tickScheduler:            tick.NewScheduler(),
		entities:                 entity.NewManager(),
		tracker:                  entity.NewTracker(),
		fallingBlocks:            entity.NewFallingBlockManager(),
		items:                    entity.NewItemEntityManager(),
		xpOrbs:                   entity.NewXPOrbManager(),
		tnts:                     entity.NewTNTManager(),
		projectiles:              projectile.NewManager(),
		schedulerDrainPerTick:    schedulerDrain,
		chunkCompletionsPerTick:  chunkCompletions,
		chunkUnloadBatchPerTick:  chunkUnloadBatch,
		chunkStreamChunksPerTick: chunkStreamPerTick,
	}, nil
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

	w.scheduler.Drain(w.schedulerDrainPerTick)
	w.processScheduledTicks()
	w.tickFallingBlocks()
	w.tickItems()
	w.tickXPOrbs()
	w.tickTNT()
	w.tickProjectiles()
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

		exp := entity.ComputeExplosion(w, pos.X, pos.Y+0.5, pos.Z, 4.0, nil)

		for _, bp := range exp.AffectedBlocks {
			w.setBlockAt(bp[0], bp[1], bp[2], 0)
			w.broadcastBlockUpdate(bp[0], bp[1], bp[2], 0)
		}

		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
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

func (w *World) tickProjectiles() {
	result := w.projectiles.TickAll(w, nil)
	for _, id := range result.RemovedIDs {
		w.broadcastRemoveEntity(id)
		w.entities.Remove(id)
	}
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
