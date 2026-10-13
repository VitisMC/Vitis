package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vitismc/vitis/internal/logger"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/storage"
	"github.com/vitismc/vitis/internal/world/chunk"
)

const (
	pregenerationInFlight  = 128
	pregenerationScanBatch = 512
)

// PregenerationStatus describes one world's persisted pregeneration job.
type PregenerationStatus struct {
	World     string
	State     string
	Shape     string
	CenterX   int
	CenterZ   int
	Radius    int
	Total     int64
	Completed int64
	Generated int64
	Elapsed   time.Duration
	Average   float64
	Maximum   float64
	Error     string
}

type pregenerationPosition struct {
	X         int32 `json:"x"`
	Z         int32 `json:"z"`
	New       bool  `json:"new"`
	Saving    bool  `json:"-"`
	Requested bool  `json:"-"`
}

type pregenerationCheckpoint struct {
	State        string                  `json:"state"`
	Shape        string                  `json:"shape"`
	Order        string                  `json:"order,omitempty"`
	CenterX      int                     `json:"center_x"`
	CenterZ      int                     `json:"center_z"`
	Radius       int                     `json:"radius"`
	MinX         int32                   `json:"min_x"`
	MinZ         int32                   `json:"min_z"`
	Width        int64                   `json:"width"`
	Total        int64                   `json:"total"`
	Cursor       int64                   `json:"cursor"`
	Completed    int64                   `json:"completed"`
	Generated    int64                   `json:"generated"`
	ElapsedNanos int64                   `json:"elapsed_nanos"`
	Maximum      float64                 `json:"maximum"`
	Pending      []pregenerationPosition `json:"pending"`
}

type pregeneration struct {
	mu                sync.Mutex
	w                 *World
	path              string
	task              pregenerationCheckpoint
	pending           map[int64]*pregenerationPosition
	state             string
	err               string
	activeSince       time.Time
	windowSince       time.Time
	windowCount       int64
	lastCheckpoint    time.Time
	lastReport        time.Time
	reportCompleted   int64
	releaseCancel     context.CancelFunc
	releaseDone       chan struct{}
	releaseForIdle    bool
	idleChunksPending bool
}

func newPregeneration(w *World, path string) *pregeneration {
	p := &pregeneration{w: w, path: path, pending: make(map[int64]*pregenerationPosition)}
	if path == "" {
		return p
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p
	}
	if err != nil || json.Unmarshal(data, &p.task) != nil || (p.task.Order != "column16" && p.task.Order != "column32") || p.task.Width <= 0 || p.task.Width > 4000 || p.task.Total <= 0 || p.task.Total > p.task.Width*p.task.Width || p.task.Cursor < 0 || p.task.Cursor > p.task.Width*p.task.Width || p.task.Completed < 0 || p.task.Completed > p.task.Total || (p.task.Shape != "square" && p.task.Shape != "circle") || len(p.task.Pending) > pregenerationInFlight {
		logger.Warn("pregeneration checkpoint could not be loaded", "world", w.name, "error", err)
		p.task = pregenerationCheckpoint{}
		return p
	}
	for i := range p.task.Pending {
		pos := p.task.Pending[i]
		pos.Requested = false
		pos.Saving = false
		p.pending[chunk.ChunkKey(pos.X, pos.Z)] = &pos
	}
	p.task.Pending = nil
	p.state = p.task.State
	if p.state != "completed" {
		p.state = "paused"
	}
	logger.Info("pregeneration checkpoint loaded", "world", w.name, "completed", p.task.Completed, "total", p.task.Total)
	return p
}

// StartPregeneration starts a bounded, durable pregeneration job.
func (w *World) StartPregeneration(shape string, centerX, centerZ, radius int) error {
	if w == nil || w.pregen == nil || w.chunkStore == nil || w.autosave == nil {
		return fmt.Errorf("pregeneration is unavailable for this world")
	}
	if shape != "square" && shape != "circle" {
		return fmt.Errorf("shape must be square or circle")
	}
	if radius <= 0 || radius > 30000 || centerX < -29999984+radius || centerX > 29999984-radius || centerZ < -29999984+radius || centerZ > 29999984-radius {
		return fmt.Errorf("selection exceeds allowed world coordinates")
	}
	minX, maxX := int32(floorDiv16(centerX-radius)), int32(floorDiv16(centerX+radius))
	minZ, maxZ := int32(floorDiv16(centerZ-radius)), int32(floorDiv16(centerZ+radius))
	width := int64(maxX - minX + 1)
	rows := int64(maxZ - minZ + 1)
	total := int64(0)
	for index := int64(0); index < width*rows; index++ {
		x, z := candidate(minX, minZ, width, index)
		if insideSelection(shape, centerX, centerZ, radius, x, z) {
			total++
		}
	}
	if total == 0 {
		return fmt.Errorf("selection contains no chunk centers")
	}
	p := w.pregen
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == "running" || p.state == "paused" {
		return fmt.Errorf("a pregeneration job already exists; cancel it first")
	}
	p.cancelGenerationReleaseLocked()
	p.idleChunksPending = false
	p.task = pregenerationCheckpoint{Shape: shape, Order: "column32", CenterX: centerX, CenterZ: centerZ, Radius: radius, MinX: minX, MinZ: minZ, Width: width, Total: total}
	p.pending = make(map[int64]*pregenerationPosition)
	p.state = "running"
	p.err = ""
	p.activeSince = time.Now()
	p.windowSince = p.activeSince
	p.lastReport = p.activeSince
	p.reportCompleted = 0
	p.windowCount = 0
	if err := p.saveLocked(); err != nil {
		p.state = ""
		return err
	}
	logger.Info("pregeneration started", "world", w.name, "shape", shape, "chunks", total)
	return nil
}

// PausePregeneration stops issuing new chunk requests and checkpoints progress.
func (w *World) PausePregeneration() error {
	return w.pregen.changeState("running", "paused")
}

// ContinuePregeneration resumes a paused job.
func (w *World) ContinuePregeneration() error {
	return w.pregen.changeState("paused", "running")
}

// CancelPregeneration discards a job while preserving generated world data.
func (w *World) CancelPregeneration() error {
	if w == nil || w.pregen == nil {
		return fmt.Errorf("pregeneration unavailable")
	}
	p := w.pregen
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != "running" && p.state != "paused" {
		return fmt.Errorf("no active pregeneration job")
	}
	p.stopClockLocked()
	p.state = "cancelled"
	p.pending = make(map[int64]*pregenerationPosition)
	p.releaseGenerationMemoryLocked()
	if p.path == "" {
		return nil
	}
	if err := os.Remove(p.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// PregenerationStatus returns a snapshot safe for command goroutines.
func (w *World) PregenerationStatus() PregenerationStatus {
	if w == nil || w.pregen == nil {
		return PregenerationStatus{State: "unavailable"}
	}
	p := w.pregen
	p.mu.Lock()
	defer p.mu.Unlock()
	elapsed := time.Duration(p.task.ElapsedNanos)
	if p.state == "running" {
		elapsed += time.Since(p.activeSince)
	}
	avg := 0.0
	if elapsed > 0 {
		avg = float64(p.task.Completed) / elapsed.Seconds()
	}
	return PregenerationStatus{World: w.name, State: p.state, Shape: p.task.Shape, CenterX: p.task.CenterX, CenterZ: p.task.CenterZ, Radius: p.task.Radius, Total: p.task.Total, Completed: p.task.Completed, Generated: p.task.Generated, Elapsed: elapsed, Average: avg, Maximum: p.task.Maximum, Error: p.err}
}

func (p *pregeneration) changeState(from, to string) error {
	if p == nil {
		return fmt.Errorf("pregeneration unavailable")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != from {
		return fmt.Errorf("pregeneration is %s", p.state)
	}
	if to == "paused" {
		p.stopClockLocked()
	} else {
		p.activeSince = time.Now()
		p.windowSince = p.activeSince
		p.lastReport = p.activeSince
		p.reportCompleted = p.task.Completed
		p.windowCount = 0
	}
	p.state = to
	if to == "paused" {
		p.releaseGenerationMemoryLocked()
	} else {
		p.cancelGenerationReleaseLocked()
	}
	return p.saveLocked()
}

func (p *pregeneration) stopClockLocked() {
	if p.state == "running" {
		p.task.ElapsedNanos += time.Since(p.activeSince).Nanoseconds()
	}
}

func (p *pregeneration) tick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != "running" {
		p.releaseIdleChunksMemoryLocked()
		return
	}
	now := time.Now()
	if elapsed := now.Sub(p.windowSince); elapsed >= time.Second {
		p.task.Maximum = math.Max(p.task.Maximum, float64(p.windowCount)/elapsed.Seconds())
		p.windowSince, p.windowCount = now, 0
	}
	snapshots := 0
	snapshotBudget := cap(p.w.autosave.queue) - len(p.w.autosave.queue)
	for key, pos := range p.pending {
		if pos.Saving {
			continue
		}
		if p.w.chunkStore.HasChunk(int(pos.X), int(pos.Z)) {
			p.completeLocked(key, pos.New)
			continue
		}
		if !pos.Requested {
			pos.Requested = p.w.chunks.RequestPregeneration(pos.X, pos.Z)
			if !pos.Requested {
				continue
			}
		}
		if snapshots >= snapshotBudget {
			continue
		}
		c, ok := p.w.chunks.GetPregenerationChunk(pos.X, pos.Z)
		if !ok {
			continue
		}
		queued, err := p.w.autosave.enqueue(c)
		if err != nil {
			p.failLocked(err)
			return
		}
		pos.Saving = queued
		if queued {
			snapshots++
		}
	}
	limit := int64(len(p.pending))
	for scanned := 0; scanned < pregenerationScanBatch && limit < pregenerationInFlight && p.task.Cursor < p.task.Width*p.task.Width; scanned++ {
		index := p.task.Cursor
		p.task.Cursor++
		x, z := candidateForTask(&p.task, index)
		if !insideSelection(p.task.Shape, p.task.CenterX, p.task.CenterZ, p.task.Radius, x, z) {
			continue
		}
		key := chunk.ChunkKey(x, z)
		if p.w.chunkStore.HasChunk(int(x), int(z)) {
			p.task.Completed++
			p.windowCount++
			continue
		}
		if !p.w.chunks.RequestPregeneration(x, z) {
			p.task.Cursor--
			break
		}
		p.pending[key] = &pregenerationPosition{X: x, Z: z, New: true, Requested: true}
		limit++
	}
	if p.task.Completed >= p.task.Total && len(p.pending) == 0 {
		p.finishLocked()
		return
	}
	if now.Sub(p.lastCheckpoint) >= 5*time.Second {
		if err := p.saveLocked(); err != nil {
			p.failLocked(err)
		}
	}
	if now.Sub(p.lastReport) >= 10*time.Second {
		elapsed := time.Duration(p.task.ElapsedNanos) + now.Sub(p.activeSince)
		rate := float64(p.task.Completed) / elapsed.Seconds()
		eta := time.Duration(0)
		if rate > 0 {
			eta = time.Duration(float64(p.task.Total-p.task.Completed) / rate * float64(time.Second))
		}
		currentRate := float64(p.task.Completed-p.reportCompleted) / now.Sub(p.lastReport).Seconds()
		logger.Info("pregeneration progress", "world", p.w.name, "completed", p.task.Completed, "total", p.task.Total, "percent", 100*float64(p.task.Completed)/float64(p.task.Total), "current_chunks_per_second", currentRate, "average_chunks_per_second", rate, "maximum_chunks_per_second", p.task.Maximum, "eta", eta)
		p.lastReport = now
		p.reportCompleted = p.task.Completed
	}
}

func (p *pregeneration) onLoad(x, z int32, err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pending[chunk.ChunkKey(x, z)]; ok {
		p.failLocked(fmt.Errorf("chunk (%d,%d): %w", x, z, err))
	}
}

func (p *pregeneration) onSaved(result chunkSaveResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := chunk.ChunkKey(result.snapshot.chunk.X(), result.snapshot.chunk.Z())
	pos, ok := p.pending[key]
	if !ok {
		return
	}
	if result.err != nil {
		pos.Saving = false
		p.failLocked(result.err)
		return
	}
	// Keep the regular runtime lifetime whenever players or entities need simulation.
	if pos.New && p.canReleaseSavedChunks() {
		p.w.chunks.ReleaseSavedChunk(result.snapshot.chunk, result.snapshot.revision, result.snapshot.accessTick)
	}
	p.completeLocked(key, true)
}

// canReleaseSavedChunks excludes runtime simulation from pregeneration-only eviction.
func (p *pregeneration) canReleaseSavedChunks() bool {
	return (p.w.entities == nil || p.w.entities.Count() == 0) &&
		(p.w.mobs == nil || p.w.mobs.Count() == 0) &&
		(p.w.tracker == nil || p.w.tracker.Count() == 0) &&
		(p.w.tickScheduler == nil || p.w.tickScheduler.Pending() == 0)
}

func (p *pregeneration) completeLocked(key int64, generated bool) {
	delete(p.pending, key)
	p.task.Completed++
	if generated {
		p.task.Generated++
	}
	p.windowCount++
}

func (p *pregeneration) finishLocked() {
	p.stopClockLocked()
	p.state = "completed"
	p.releaseGenerationMemoryLocked()
	seconds := time.Duration(p.task.ElapsedNanos).Seconds()
	if seconds <= 0 {
		seconds = 1e-9
	}
	p.task.Maximum = math.Max(p.task.Maximum, math.Max(float64(p.windowCount), float64(p.task.Completed)/seconds))
	if err := p.saveLocked(); err != nil {
		logger.Error("pregeneration final checkpoint failed", "world", p.w.name, "error", err)
	}
	logger.Info("pregeneration completed", "world", p.w.name, "chunks", p.task.Completed, "new_chunks", p.task.Generated, "seconds", seconds, "average_chunks_per_second", float64(p.task.Completed)/seconds, "maximum_chunks_per_second", p.task.Maximum)
	message := fmt.Sprintf("§aPregeneration complete in %s: %d chunks (%d new), average %.1f chunks/s, maximum %.1f chunks/s", p.w.name, p.task.Completed, p.task.Generated, float64(p.task.Completed)/seconds, p.task.Maximum)
	for _, player := range p.w.tracker.Players() {
		if !player.Removed() && player.Session() != nil {
			_ = player.Session().Send(playpacket.NewSystemChatText(message))
		}
	}
}

func (p *pregeneration) failLocked(err error) {
	p.err = err.Error()
	p.stopClockLocked()
	p.state = "paused"
	p.releaseGenerationMemoryLocked()
	logger.Error("pregeneration paused after error", "world", p.w.name, "error", err)
	if saveErr := p.saveLocked(); saveErr != nil {
		logger.Error("pregeneration checkpoint failed", "world", p.w.name, "error", saveErr)
	}
}

func (p *pregeneration) saveLocked() error {
	if p.path == "" {
		return nil
	}
	p.task.State = p.state
	p.task.Pending = p.task.Pending[:0]
	for _, pos := range p.pending {
		p.task.Pending = append(p.task.Pending, *pos)
	}
	data, err := json.Marshal(p.task)
	p.task.Pending = nil
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0755); err != nil {
		return err
	}
	if err := storage.WriteFileAtomic(p.path, data, 0644); err != nil {
		return err
	}
	p.lastCheckpoint = time.Now()
	return nil
}

func candidate(minX, minZ int32, width, index int64) (int32, int32) {
	return minX + int32(index%width), minZ + int32(index/width)
}

// candidateForTask groups requests into narrow bands for cache locality.
func candidateForTask(task *pregenerationCheckpoint, index int64) (int32, int32) {
	stripe := int64(16)
	if task.Order == "column32" {
		stripe = 32
	}
	band := index / (task.Width * stripe)
	rows := task.Width - band*stripe
	if rows > stripe {
		rows = stripe
	}
	within := index - band*task.Width*stripe
	return task.MinX + int32(within/rows), task.MinZ + int32(band*stripe+within%rows)
}

func insideSelection(shape string, centerX, centerZ, radius int, x, z int32) bool {
	dx := distanceToChunk(int64(centerX), int64(x)*16)
	dz := distanceToChunk(int64(centerZ), int64(z)*16)
	if shape == "circle" {
		return dx*dx+dz*dz <= int64(radius)*int64(radius)
	}
	return absInt64(dx) <= int64(radius) && absInt64(dz) <= int64(radius)
}

func distanceToChunk(point, chunkMin int64) int64 {
	if point < chunkMin {
		return chunkMin - point
	}
	if point > chunkMin+15 {
		return point - chunkMin - 15
	}
	return 0
}

func floorDiv16(n int) int { return n >> 4 }
func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
