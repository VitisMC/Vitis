package streaming

import "errors"

var (
	ErrChunkNotReady = errors.New("chunk not ready")
)

const (
	defaultChunksPerTick = 4
	defaultViewDistance   = 10
)

// StreamerConfig defines per-player chunk streamer settings.
type StreamerConfig struct {
	ChunksPerTick int
	ViewDistance   int32
	Provider      ChunkProvider
	Sender        ChunkSender
}

// Streamer drives incremental, prioritized, rate-limited chunk delivery for one player.
type Streamer struct {
	view     *View
	pipeline *SendPipeline

	provider ChunkProvider
	sender   ChunkSender

	chunksPerTick int

	sendBuf []ChunkPos
}

// NewStreamer creates a per-player chunk streamer.
func NewStreamer(cfg StreamerConfig) *Streamer {
	chunksPerTick := cfg.ChunksPerTick
	if chunksPerTick <= 0 {
		chunksPerTick = defaultChunksPerTick
	}

	viewDistance := cfg.ViewDistance
	if viewDistance <= 0 {
		viewDistance = defaultViewDistance
	}

	return &Streamer{
		view:          NewView(ChunkPos{}, viewDistance),
		pipeline:      NewSendPipeline(),
		provider:      cfg.Provider,
		sender:        cfg.Sender,
		chunksPerTick: chunksPerTick,
		sendBuf:       make([]ChunkPos, 0, chunksPerTick),
	}
}

// Update processes one tick of chunk streaming for the player at the given chunk coordinates.
// Returns the number of chunks sent and unloaded this tick.
func (s *Streamer) Update(chunkX, chunkZ int32) (sent int, unloaded int) {
	newCenter := ChunkPos{X: chunkX, Z: chunkZ}
	oldCenter := s.view.Center()

	if oldCenter != newCenter {
		diff := ComputeDiff(oldCenter, newCenter, s.view.ViewDistance(), s.view.Loaded())
		s.view.SetCenter(newCenter)

		for _, pos := range diff.ToUnload {
			_ = s.pipeline.SendUnloadChunk(pos, s.sender)
			s.view.MarkUnloaded(pos)
			unloaded++
		}

		for _, pos := range diff.ToLoad {
			s.view.AddPending(pos)
		}
	}

	s.sendBuf = s.sendBuf[:0]
	skipped := make([]PriorityEntry, 0, 4)

	for len(s.sendBuf) < s.chunksPerTick {
		pos, ok := s.view.PopPending()
		if !ok {
			break
		}

		if !s.view.InRange(pos) {
			continue
		}

		if _, chunkReady := s.provider.GetChunk(pos.X, pos.Z); !chunkReady {
			skipped = append(skipped, PriorityEntry{
				Pos:      pos,
				Priority: ManhattanDistance(pos, s.view.Center()),
			})
			continue
		}

		s.sendBuf = append(s.sendBuf, pos)
	}

	for _, entry := range skipped {
		s.view.pending.Enqueue(entry.Pos, entry.Priority)
		s.view.pendingSet[entry.Pos] = struct{}{}
	}

	for _, pos := range s.sendBuf {
		if err := s.pipeline.SendChunk(pos, s.provider, s.sender); err != nil {
			break
		}
		s.view.MarkLoaded(pos)
		sent++
	}

	return sent, unloaded
}

// InitialLoad enqueues all chunks within view distance for the first load.
func (s *Streamer) InitialLoad(chunkX, chunkZ int32) {
	center := ChunkPos{X: chunkX, Z: chunkZ}
	s.view.SetCenter(center)
	s.view.ClearPending()

	chunks := ComputeFullLoad(center, s.view.ViewDistance(), s.view.Loaded())
	for _, pos := range chunks {
		s.view.AddPending(pos)
	}
}

// ForceResend clears the loaded set and re-enqueues everything for re-send.
func (s *Streamer) ForceResend() {
	loaded := s.view.Loaded()
	for pos := range loaded {
		delete(loaded, pos)
	}
	s.view.ClearPending()

	chunks := ComputeFullLoad(s.view.Center(), s.view.ViewDistance(), loaded)
	for _, pos := range chunks {
		s.view.AddPending(pos)
	}
}

// SetChunksPerTick updates the rate limit.
func (s *Streamer) SetChunksPerTick(n int) {
	if n <= 0 {
		n = defaultChunksPerTick
	}
	s.chunksPerTick = n
}

// ChunksPerTick returns the current rate limit.
func (s *Streamer) ChunksPerTick() int {
	return s.chunksPerTick
}

// View returns the player's chunk view state.
func (s *Streamer) View() *View {
	return s.view
}

// Close releases streamer resources.
func (s *Streamer) Close() {
	if s.view != nil {
		s.view.ClearPending()
	}
}

// Manager coordinates chunk streaming for all players in a world.
type Manager struct {
	streamers map[int32]*Streamer
	pipeline  *SendPipeline
}

// NewManager creates a streaming manager.
func NewManager() *Manager {
	return &Manager{
		streamers: make(map[int32]*Streamer, 64),
		pipeline:  NewSendPipeline(),
	}
}

// AddPlayer creates and registers a streamer for the given player ID.
func (m *Manager) AddPlayer(playerID int32, cfg StreamerConfig) *Streamer {
	s := NewStreamer(cfg)
	m.streamers[playerID] = s
	return s
}

// RemovePlayer removes and closes the streamer for the given player ID.
func (m *Manager) RemovePlayer(playerID int32, sender ChunkSender) {
	s, ok := m.streamers[playerID]
	if !ok {
		return
	}

	if sender != nil {
		for pos := range s.view.Loaded() {
			_ = s.pipeline.SendUnloadChunk(pos, sender)
		}
	}

	s.Close()
	delete(m.streamers, playerID)
}

// GetStreamer returns the streamer for the given player ID.
func (m *Manager) GetStreamer(playerID int32) (*Streamer, bool) {
	s, ok := m.streamers[playerID]
	return s, ok
}

// Tick runs one streaming cycle for all registered players.
// The caller provides a function to resolve each player's current chunk coordinates.
func (m *Manager) Tick(positionFn func(playerID int32) (chunkX, chunkZ int32, ok bool)) (totalSent, totalUnloaded int) {
	for playerID, s := range m.streamers {
		chunkX, chunkZ, ok := positionFn(playerID)
		if !ok {
			continue
		}
		sent, unloaded := s.Update(chunkX, chunkZ)
		totalSent += sent
		totalUnloaded += unloaded
	}
	return totalSent, totalUnloaded
}

// PlayerCount returns the number of registered streamers.
func (m *Manager) PlayerCount() int {
	return len(m.streamers)
}
