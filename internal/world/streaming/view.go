package streaming

// ChunkPos identifies a chunk by its X and Z coordinates.
type ChunkPos struct {
	X int32
	Z int32
}

// Key packs chunk coordinates into a single int64 for map usage.
func (p ChunkPos) Key() int64 {
	return int64(uint64(uint32(p.X))<<32 | uint64(uint32(p.Z)))
}

const defaultLoadedCapacity = 256

// View tracks per-player chunk streaming state.
type View struct {
	center       ChunkPos
	direction    ChunkPos
	viewDistance int32

	loaded     map[ChunkPos]struct{}
	pending    *PriorityQueue
	pendingSet map[ChunkPos]struct{}
}

// NewView creates a player chunk view with the given initial center and view distance.
func NewView(center ChunkPos, viewDistance int32) *View {
	if viewDistance < 0 {
		viewDistance = 0
	}
	expected := int((2*viewDistance + 1) * (2*viewDistance + 1))
	if expected < defaultLoadedCapacity {
		expected = defaultLoadedCapacity
	}
	return &View{
		center:       center,
		viewDistance: viewDistance,
		loaded:       make(map[ChunkPos]struct{}, expected),
		pending:      NewPriorityQueue(expected),
		pendingSet:   make(map[ChunkPos]struct{}, expected),
	}
}

// Center returns the current chunk center position.
func (v *View) Center() ChunkPos {
	return v.center
}

// SetCenter updates the chunk center position.
func (v *View) SetCenter(center ChunkPos) {
	v.center = center
}

// SetDirection prioritizes chunks ahead of the player's last chunk movement.
func (v *View) SetDirection(dx, dz int32) {
	v.direction = ChunkPos{X: sign32(dx), Z: sign32(dz)}
}

func sign32(value int32) int32 {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

// ViewDistance returns the current view distance.
func (v *View) ViewDistance() int32 {
	return v.viewDistance
}

// SetViewDistance updates the view distance.
func (v *View) SetViewDistance(d int32) {
	if d < 0 {
		d = 0
	}
	v.viewDistance = d
}

// IsLoaded reports whether the given chunk has been sent to the client.
func (v *View) IsLoaded(pos ChunkPos) bool {
	_, ok := v.loaded[pos]
	return ok
}

// LoadedCount returns the number of chunks currently loaded on the client.
func (v *View) LoadedCount() int {
	return len(v.loaded)
}

// MarkLoaded records that a chunk has been sent to the client.
func (v *View) MarkLoaded(pos ChunkPos) {
	v.loaded[pos] = struct{}{}
	delete(v.pendingSet, pos)
}

// MarkUnloaded removes a chunk from the loaded set.
func (v *View) MarkUnloaded(pos ChunkPos) {
	delete(v.loaded, pos)
	delete(v.pendingSet, pos)
}

// IsPending reports whether the given chunk is already in the pending queue.
func (v *View) IsPending(pos ChunkPos) bool {
	_, ok := v.pendingSet[pos]
	return ok
}

// AddPending enqueues a chunk for sending with distance-based priority.
func (v *View) AddPending(pos ChunkPos) {
	if v.IsLoaded(pos) || v.IsPending(pos) {
		return
	}
	priority := DistanceSquared(pos, v.center)
	secondary := balancedRadialOrder(pos, v.center)
	if v.direction != (ChunkPos{}) {
		dx, dz := pos.X-v.center.X, pos.Z-v.center.Z
		secondary -= (dx*v.direction.X + dz*v.direction.Z) * 16
	}
	v.pending.EnqueueBalanced(pos, priority, secondary)
	v.pendingSet[pos] = struct{}{}
}

func balancedRadialOrder(pos, center ChunkPos) int32 {
	dx, dz := pos.X-center.X, pos.Z-center.Z
	radius := dx
	if radius < 0 {
		radius = -radius
	}
	if value := dz; value < 0 {
		if -value > radius {
			radius = -value
		}
	} else if value > radius {
		radius = value
	}
	if radius == 0 {
		return 0
	}
	var perimeter int32
	switch {
	case dz == -radius:
		perimeter = dx + radius
	case dx == radius:
		perimeter = 2*radius + dz + radius
	case dz == radius:
		perimeter = 4*radius + radius - dx
	default:
		perimeter = 6*radius + radius - dz
	}
	half := 4 * radius
	return (perimeter%half)*2 + perimeter/half
}

// PopPending dequeues the highest-priority pending chunk.
func (v *View) PopPending() (ChunkPos, bool) {
	for v.pending.Len() > 0 {
		entry, ok := v.pending.Dequeue()
		if !ok {
			return ChunkPos{}, false
		}
		if _, stillPending := v.pendingSet[entry.Pos]; !stillPending {
			continue
		}
		delete(v.pendingSet, entry.Pos)
		return entry.Pos, true
	}
	return ChunkPos{}, false
}

// PendingCount returns the number of chunks in the pending queue.
func (v *View) PendingCount() int {
	return len(v.pendingSet)
}

// InRange reports whether a chunk position is within the current view distance.
func (v *View) InRange(pos ChunkPos) bool {
	return inRange(pos, v.center, v.viewDistance)
}

// ClearPending removes all entries from the pending queue.
func (v *View) ClearPending() {
	v.pending.Clear()
	for k := range v.pendingSet {
		delete(v.pendingSet, k)
	}
}

// Loaded returns a reference to the loaded chunk set for iteration.
func (v *View) Loaded() map[ChunkPos]struct{} {
	return v.loaded
}
