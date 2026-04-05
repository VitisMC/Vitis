package tick

import (
	"container/heap"
	"sync"
)

type Priority int8

const (
	PriorityExtremelyHigh Priority = -3
	PriorityVeryHigh      Priority = -2
	PriorityHigh          Priority = -1
	PriorityNormal        Priority = 0
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
	index      int
}

type tickHeap []*ScheduledTick

func (h tickHeap) Len() int { return len(h) }

func (h tickHeap) Less(i, j int) bool {
	if h[i].TargetTick != h[j].TargetTick {
		return h[i].TargetTick < h[j].TargetTick
	}
	return h[i].Priority < h[j].Priority
}

func (h tickHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *tickHeap) Push(x any) {
	n := len(*h)
	item := x.(*ScheduledTick)
	item.index = n
	*h = append(*h, item)
}

func (h *tickHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
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
	heap      tickHeap
	scheduled map[tickKey]struct{}
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		heap:      make(tickHeap, 0, 256),
		scheduled: make(map[tickKey]struct{}, 256),
	}
	heap.Init(&s.heap)
	return s
}

func makeTickKey(x, y, z int, tickType TickType, subID int32) tickKey {
	return tickKey{x: int32(x), y: int32(y), z: int32(z), tt: tickType, subID: subID}
}

func (s *Scheduler) Schedule(x, y, z int, currentTick uint64, delay int, priority Priority, tickType TickType, subID int32) {
	if s == nil || delay < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	}
	heap.Push(&s.heap, tick)
}

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

func (s *Scheduler) Drain(currentTick uint64, maxCount int) []*ScheduledTick {
	if s == nil || maxCount <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*ScheduledTick, 0, min(maxCount, s.heap.Len()))
	for len(result) < maxCount && s.heap.Len() > 0 {
		if s.heap[0].TargetTick > currentTick {
			break
		}
		tick := heap.Pop(&s.heap).(*ScheduledTick)
		key := makeTickKey(tick.X, tick.Y, tick.Z, tick.Type, tick.SubID)
		delete(s.scheduled, key)
		result = append(result, tick)
	}
	return result
}

func (s *Scheduler) Pending() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heap.Len()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
