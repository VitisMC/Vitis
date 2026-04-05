package streaming

import "container/heap"

const defaultPriorityQueueCapacity = 512

// PriorityEntry pairs a chunk position with its computed priority.
type PriorityEntry struct {
	Pos      ChunkPos
	Priority int32
}

// PriorityQueue is a min-heap of chunk positions ordered by distance from center.
type PriorityQueue struct {
	entries []PriorityEntry
}

// NewPriorityQueue creates a priority queue with pre-allocated backing storage.
func NewPriorityQueue(capacity int) *PriorityQueue {
	if capacity <= 0 {
		capacity = defaultPriorityQueueCapacity
	}
	return &PriorityQueue{entries: make([]PriorityEntry, 0, capacity)}
}

// Len returns the number of entries in the queue.
func (pq *PriorityQueue) Len() int {
	return len(pq.entries)
}

// Less reports whether entry i should be dequeued before entry j.
func (pq *PriorityQueue) Less(i, j int) bool {
	if pq.entries[i].Priority != pq.entries[j].Priority {
		return pq.entries[i].Priority < pq.entries[j].Priority
	}
	if pq.entries[i].Pos.X != pq.entries[j].Pos.X {
		return pq.entries[i].Pos.X < pq.entries[j].Pos.X
	}
	return pq.entries[i].Pos.Z < pq.entries[j].Pos.Z
}

// Swap exchanges two entries.
func (pq *PriorityQueue) Swap(i, j int) {
	pq.entries[i], pq.entries[j] = pq.entries[j], pq.entries[i]
}

// Push adds an entry to the heap. Use heap.Push instead of calling directly.
func (pq *PriorityQueue) Push(x any) {
	pq.entries = append(pq.entries, x.(PriorityEntry))
}

// Pop removes and returns the minimum entry. Use heap.Pop instead of calling directly.
func (pq *PriorityQueue) Pop() any {
	old := pq.entries
	n := len(old)
	entry := old[n-1]
	pq.entries = old[:n-1]
	return entry
}

// Enqueue pushes a chunk position with the given priority.
func (pq *PriorityQueue) Enqueue(pos ChunkPos, priority int32) {
	heap.Push(pq, PriorityEntry{Pos: pos, Priority: priority})
}

// Dequeue removes and returns the highest-priority (lowest distance) entry.
func (pq *PriorityQueue) Dequeue() (PriorityEntry, bool) {
	if len(pq.entries) == 0 {
		return PriorityEntry{}, false
	}
	entry := heap.Pop(pq).(PriorityEntry)
	return entry, true
}

// Clear removes all entries while retaining backing storage.
func (pq *PriorityQueue) Clear() {
	pq.entries = pq.entries[:0]
}

// ManhattanDistance computes the Manhattan distance between two chunk positions.
func ManhattanDistance(a, b ChunkPos) int32 {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dz := a.Z - b.Z
	if dz < 0 {
		dz = -dz
	}
	return dx + dz
}
