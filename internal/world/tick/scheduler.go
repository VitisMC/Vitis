package tick

import (
	"container/heap"
	"github.com/vitismc/vitis/internal/block"
	"math"
	"sort"
	"sync"
)

type Priority int8

const (
	PriorityExtremelyHigh Priority = -3
	PriorityVeryHigh      Priority = -2
	PriorityHigh          Priority = -1
	PriorityNormal        Priority = 0
	PriorityLow           Priority = 1
	PriorityVeryLow       Priority = 2
	PriorityExtremelyLow  Priority = 3
)

type TickType uint8

const (
	TickTypeBlock TickType = iota
	TickTypeFluid
)

type ScheduledTick struct {
	X, Y, Z    int
	TargetTick uint64
	Priority   Priority
	Type       TickType
	SubID      int32
	sequence   uint64
}

type tickHeap []*ScheduledTick

func (h tickHeap) Len() int { return len(h) }

func (h tickHeap) Less(i, j int) bool {
	if h[i].TargetTick != h[j].TargetTick {
		return h[i].TargetTick < h[j].TargetTick
	}
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].sequence < h[j].sequence
}

func (h tickHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *tickHeap) Push(x any) {
	item := x.(*ScheduledTick)
	*h = append(*h, item)
}

func (h *tickHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return item
}

type tickKey struct {
	x, y, z int32
	tt      TickType
	subID   int32
}

type Scheduler struct {
	mu        sync.Mutex
	queues    map[int64]*tickHeap
	scheduled map[tickKey]struct{}
	sequence  uint64
	onChange  func(int32, int32)
}

// SetChangeHandler observes scheduler mutations after unlocking; configure it before use.
func (s *Scheduler) SetChangeHandler(handler func(int32, int32)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = handler
}

// NewScheduler creates per-chunk queues sharing a stable execution sequence.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		queues:    make(map[int64]*tickHeap),
		scheduled: make(map[tickKey]struct{}, 256),
	}
	return s
}

// makeTickKey deduplicates a block's property variants by block identity.
func makeTickKey(x, y, z int, tickType TickType, subID int32) tickKey {
	if tickType == TickTypeBlock {
		if blockID := block.BlockIDFromState(subID); blockID >= 0 {
			subID = blockID
		}
	}
	return tickKey{x: int32(x), y: int32(y), z: int32(z), tt: tickType, subID: subID}
}

// Schedule inserts valid work once without replacing an existing tick.
func (s *Scheduler) Schedule(x, y, z int, currentTick uint64, delay int, priority Priority, tickType TickType, subID int32) {
	if s == nil || delay < 0 || uint64(delay) > math.MaxUint64-currentTick || priority < PriorityExtremelyHigh || priority > PriorityExtremelyLow || tickType > TickTypeFluid || x < math.MinInt32 || x > math.MaxInt32 || y < math.MinInt32 || y > math.MaxInt32 || z < math.MinInt32 || z > math.MaxInt32 {
		return
	}
	s.mu.Lock()
	changed := false
	defer func() {
		handler := s.onChange
		s.mu.Unlock()
		if changed && handler != nil {
			handler(int32(x)>>4, int32(z)>>4)
		}
	}()
	key := makeTickKey(x, y, z, tickType, subID)
	if _, exists := s.scheduled[key]; exists {
		return
	}
	s.scheduled[key] = struct{}{}
	tick := &ScheduledTick{
		X:          x,
		Y:          y,
		Z:          z,
		TargetTick: currentTick + uint64(delay),
		Priority:   priority,
		Type:       tickType,
		SubID:      subID,
		sequence:   s.sequence,
	}
	s.sequence++
	chunk := scheduledChunkKey(x, z)
	queue := s.queues[chunk]
	if queue == nil {
		queue = &tickHeap{}
		s.queues[chunk] = queue
	}
	heap.Push(queue, tick)
	changed = true
}

// IsScheduled reports whether a position and type already have pending work.
func (s *Scheduler) IsScheduled(x, y, z int, tickType TickType, subID int32) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := makeTickKey(x, y, z, tickType, subID)
	_, exists := s.scheduled[key]
	return exists
}

// Drain selects due ticks across all chunks up to the requested limit.
func (s *Scheduler) Drain(currentTick uint64, maxCount int) []*ScheduledTick {
	return s.DrainEligible(currentTick, maxCount, nil)
}

// DrainEligible drains due ticks only from active chunks, without blocking on inactive heads.
// The predicate runs while the scheduler is locked and must not call scheduler methods.
func (s *Scheduler) DrainEligible(currentTick uint64, maxCount int, eligible func(int32, int32) bool) []*ScheduledTick {
	if s == nil || maxCount <= 0 {
		return nil
	}
	s.mu.Lock()
	changed := make(map[int64]struct{})
	defer func() {
		handler := s.onChange
		s.mu.Unlock()
		if handler != nil {
			for key := range changed {
				handler(int32(uint64(key)>>32), int32(key))
			}
		}
	}()
	heads := make(tickHeap, 0, len(s.queues))
	for _, queue := range s.queues {
		if len(*queue) == 0 {
			continue
		}
		t := (*queue)[0]
		if t.TargetTick <= currentTick && (eligible == nil || eligible(int32(t.X)>>4, int32(t.Z)>>4)) {
			heads = append(heads, t)
		}
	}
	heap.Init(&heads)
	result := make([]*ScheduledTick, 0, min(maxCount, len(s.scheduled)))
	for len(result) < maxCount && len(heads) != 0 {
		tick := heap.Pop(&heads).(*ScheduledTick)
		chunk := scheduledChunkKey(tick.X, tick.Z)
		changed[chunk] = struct{}{}
		queue := s.queues[chunk]
		heap.Pop(queue)
		if len(*queue) == 0 {
			delete(s.queues, chunk)
		} else if (*queue)[0].TargetTick <= currentTick {
			heap.Push(&heads, (*queue)[0])
		}
		key := makeTickKey(tick.X, tick.Y, tick.Z, tick.Type, tick.SubID)
		delete(s.scheduled, key)
		result = append(result, tick)
	}
	return result
}

// Pending returns the total number of retained ticks, including inactive chunks.
func (s *Scheduler) Pending() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.scheduled)
}

// SnapshotChunk copies a chunk's pending ticks in execution order without removing them.
func (s *Scheduler) SnapshotChunk(x, z int32) []ScheduledTick {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.queues[scheduledChunkKey(int(x)*16, int(z)*16)]
	if queue == nil {
		return nil
	}
	result := make([]ScheduledTick, 0, len(*queue))
	for _, value := range *queue {
		result = append(result, *value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TargetTick != result[j].TargetTick {
			return result[i].TargetTick < result[j].TargetTick
		}
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].sequence < result[j].sequence
	})
	return result
}

// PendingInChunk reads queue size without allocating a tick snapshot.
func (s *Scheduler) PendingInChunk(x, z int32) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if queue := s.queues[scheduledChunkKey(int(x)*16, int(z)*16)]; queue != nil {
		return len(*queue)
	}
	return 0
}

// RemoveChunk forgets ticks only after the caller has durably saved their snapshot.
func (s *Scheduler) RemoveChunk(x, z int32) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := scheduledChunkKey(int(x)*16, int(z)*16)
	if queue := s.queues[key]; queue != nil {
		for _, t := range *queue {
			delete(s.scheduled, makeTickKey(t.X, t.Y, t.Z, t.Type, t.SubID))
		}
		delete(s.queues, key)
	}
}

// scheduledChunkKey packs signed block coordinates into their owning chunk key.
func scheduledChunkKey(x, z int) int64 {
	return int64(uint64(uint32(int32(x)>>4))<<32 | uint64(uint32(int32(z)>>4)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
