package streaming

import (
	"errors"
	"time"

	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
)

var (
	ErrChunkNotReady = errors.New("chunk not ready")
)

const (
	defaultChunksPerTick                    = 32
	defaultViewDistance                     = 10
	defaultClientChunkRate          float32 = 9
	maxClientChunkRate              float32 = 500
	initialMaxUnacknowledgedBatches         = 1
	maxUnacknowledgedBatches                = 10
	minimumPendingScanBudget                = 256
	minimumLoadRequestWindow                = 48
	loadRequestRetryTicks                   = 200
	prefetchTicks                           = 20
)

// StreamerConfig defines per-player chunk streamer settings.
type StreamerConfig struct {
	ChunksPerTick       int
	ViewDistance        int32
	Provider            ChunkProvider
	Sender              ChunkSender
	OnChunkSent         func()
	OnChunkAcknowledged func(time.Duration)
}

// Streamer drives incremental, prioritized, rate-limited chunk delivery for one player.
type Streamer struct {
	view     *View
	pipeline *SendPipeline

	provider            ChunkProvider
	sender              ChunkSender
	onChunkSent         func()
	onChunkAcknowledged func(time.Duration)
	pendingAckChunks    [][]*chunk.Chunk

	chunksPerTick            int
	desiredChunksPerTick     float32
	batchQuota               float32
	unacknowledgedBatches    int
	maxUnacknowledgedBatches int
	lastAcknowledgement      uint64

	sendBuf        []ChunkPos
	pendingUnloads map[ChunkPos]struct{}
	requestedLoads map[ChunkPos]uint64
	updateTick     uint64
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
		view:                     NewView(ChunkPos{}, viewDistance),
		pipeline:                 NewSendPipeline(),
		provider:                 cfg.Provider,
		sender:                   cfg.Sender,
		onChunkSent:              cfg.OnChunkSent,
		onChunkAcknowledged:      cfg.OnChunkAcknowledged,
		chunksPerTick:            chunksPerTick,
		desiredChunksPerTick:     defaultClientChunkRate,
		maxUnacknowledgedBatches: initialMaxUnacknowledgedBatches,
		sendBuf:                  make([]ChunkPos, 0, chunksPerTick),
		pendingUnloads:           make(map[ChunkPos]struct{}),
		requestedLoads:           make(map[ChunkPos]uint64),
	}
}

// Update processes one tick of chunk streaming for the player at the given chunk coordinates.
// Returns the number of chunks sent and unloaded this tick.
func (s *Streamer) Update(chunkX, chunkZ int32) (sent int, unloaded int) {
	if s.provider == nil || s.sender == nil {
		return 0, 0
	}
	s.updateTick++

	newCenter := ChunkPos{X: chunkX, Z: chunkZ}
	oldCenter := s.view.Center()

	if oldCenter != newCenter {
		if err := s.sender.Send(&play.SetCenterChunk{ChunkX: chunkX, ChunkZ: chunkZ}); err != nil {
			return 0, 0
		}

		diff := ComputeDiff(oldCenter, newCenter, s.view.ViewDistance(), s.view.Loaded())
		s.view.SetCenter(newCenter)
		s.view.SetDirection(newCenter.X-oldCenter.X, newCenter.Z-oldCenter.Z)

		for _, pos := range diff.ToUnload {
			s.pendingUnloads[pos] = struct{}{}
		}

		// Rebuild the queue so all unsent chunks in the overlapping part of the
		// view remain present and priorities follow the new player position.
		s.view.ClearPending()
		for pos := range s.requestedLoads {
			if !s.view.InRange(pos) {
				delete(s.requestedLoads, pos)
			}
		}
		for _, pos := range ComputeFullLoad(newCenter, s.view.ViewDistance(), s.view.Loaded()) {
			delete(s.pendingUnloads, pos)
			s.view.AddPending(pos)
		}
	}

	for pos := range s.pendingUnloads {
		if s.view.InRange(pos) {
			delete(s.pendingUnloads, pos)
			continue
		}
		if err := s.pipeline.SendUnloadChunk(pos, s.sender); err != nil {
			continue
		}
		delete(s.pendingUnloads, pos)
		s.view.MarkUnloaded(pos)
		unloaded++
	}

	s.prefetchLoads()

	if s.chunksPerTick == 0 || s.unacknowledgedBatches >= s.maxUnacknowledgedBatches {
		return 0, unloaded
	}

	desired := s.desiredChunksPerTick
	if desired > float32(s.chunksPerTick) {
		desired = float32(s.chunksPerTick)
	}
	if desired < 0.01 {
		desired = 0.01
	}
	maxQuota := desired
	if maxQuota < 1 {
		maxQuota = 1
	}
	s.batchQuota += desired
	if s.batchQuota > maxQuota {
		s.batchQuota = maxQuota
	}
	if s.batchQuota < 1 {
		return 0, unloaded
	}

	budget := int(s.batchQuota)
	if budget > s.chunksPerTick {
		budget = s.chunksPerTick
	}
	s.sendBuf = s.sendBuf[:0]
	skipped := make([]ChunkPos, 0, budget)
	scanBudget := s.chunksPerTick * 16
	if scanBudget < minimumPendingScanBudget {
		scanBudget = minimumPendingScanBudget
	}
	if pending := s.view.PendingCount(); scanBudget > pending {
		scanBudget = pending
	}
	requestWindow := s.loadRequestWindow()

	for scanned := 0; len(s.sendBuf) < budget && scanned < scanBudget; scanned++ {
		pos, ok := s.view.PopPending()
		if !ok {
			break
		}

		if !s.view.InRange(pos) {
			continue
		}

		if _, chunkReady := s.provider.GetChunk(pos.X, pos.Z); !chunkReady {
			requestedAt, requested := s.requestedLoads[pos]
			retry := requested && s.updateTick-requestedAt >= loadRequestRetryTicks
			if (!requested && len(s.requestedLoads) < requestWindow) || retry {
				if requester, ok := s.provider.(ChunkRequester); ok && requester.RequestLoad(pos.X, pos.Z) {
					s.requestedLoads[pos] = s.updateTick
				}
			}
			skipped = append(skipped, pos)
			continue
		}

		delete(s.requestedLoads, pos)
		s.sendBuf = append(s.sendBuf, pos)
	}

	for _, pos := range skipped {
		s.view.AddPending(pos)
	}

	if len(s.sendBuf) == 0 {
		return 0, unloaded
	}
	if err := s.sender.Send(&play.ChunkBatchStart{}); err != nil {
		for _, pos := range s.sendBuf {
			s.view.AddPending(pos)
		}
		return 0, unloaded
	}

	var sentChunks []*chunk.Chunk
	for i, pos := range s.sendBuf {
		if err := s.pipeline.SendChunk(pos, s.provider, s.sender); err != nil {
			for _, retry := range s.sendBuf[i:] {
				s.view.AddPending(retry)
			}
			break
		}
		s.view.MarkLoaded(pos)
		sent++
		if s.onChunkAcknowledged != nil {
			if generated, ok := s.provider.GetChunk(pos.X, pos.Z); ok {
				sentChunks = append(sentChunks, generated)
			}
		}
		if s.onChunkSent != nil {
			s.onChunkSent()
		}
	}
	if err := s.sender.Send(&play.ChunkBatchFinished{BatchSize: int32(sent)}); err != nil {
		return sent, unloaded
	}
	s.pendingAckChunks = append(s.pendingAckChunks, sentChunks)
	s.unacknowledgedBatches++
	s.batchQuota -= float32(sent)

	return sent, unloaded
}

func (s *Streamer) prefetchLoads() {
	requester, ok := s.provider.(ChunkRequester)
	if !ok {
		return
	}
	window := s.loadRequestWindow()
	if len(s.requestedLoads) >= window {
		return
	}
	scanBudget := window * 4
	if pending := s.view.PendingCount(); scanBudget > pending {
		scanBudget = pending
	}
	deferred := make([]ChunkPos, 0, scanBudget)
	for scanned := 0; scanned < scanBudget && len(s.requestedLoads) < window; scanned++ {
		pos, present := s.view.PopPending()
		if !present {
			break
		}
		if !s.view.InRange(pos) {
			continue
		}
		deferred = append(deferred, pos)
		if _, ready := s.provider.GetChunk(pos.X, pos.Z); ready {
			continue
		}
		requestedAt, requested := s.requestedLoads[pos]
		if requested && s.updateTick-requestedAt < loadRequestRetryTicks {
			continue
		}
		if requester.RequestLoad(pos.X, pos.Z) {
			s.requestedLoads[pos] = s.updateTick
		}
	}
	for _, pos := range deferred {
		s.view.AddPending(pos)
	}
}

// loadRequestWindow keeps enough work queued for one second of client delivery.
func (s *Streamer) loadRequestWindow() int {
	rate := s.desiredChunksPerTick
	if rate > float32(s.chunksPerTick) {
		rate = float32(s.chunksPerTick)
	}
	window := int(rate*prefetchTicks + 0.999)
	if window < minimumLoadRequestWindow {
		window = minimumLoadRequestWindow
	}
	return window
}

// InitialLoad enqueues all chunks within view distance for the first load.
func (s *Streamer) InitialLoad(chunkX, chunkZ int32) {
	center := ChunkPos{X: chunkX, Z: chunkZ}
	s.view.SetCenter(center)
	s.view.ClearPending()
	clear(s.requestedLoads)
	for pos := range s.pendingUnloads {
		delete(s.pendingUnloads, pos)
	}

	chunks := ComputeFullLoad(center, s.view.ViewDistance(), s.view.Loaded())
	for _, pos := range chunks {
		s.view.AddPending(pos)
	}
}

// ForceResend clears the loaded set and re-enqueues everything for re-send.
func (s *Streamer) ForceResend() {
	s.unacknowledgedBatches = 0
	s.batchQuota = 0
	clear(s.pendingAckChunks)
	s.pendingAckChunks = s.pendingAckChunks[:0]
	loaded := s.view.Loaded()
	for pos := range loaded {
		delete(loaded, pos)
	}
	s.view.ClearPending()
	clear(s.requestedLoads)

	chunks := ComputeFullLoad(s.view.Center(), s.view.ViewDistance(), loaded)
	for _, pos := range chunks {
		s.view.AddPending(pos)
	}
}

// SetChunksPerTick updates the rate limit.
func (s *Streamer) SetChunksPerTick(n int) {
	if n < 0 {
		n = 0
	}
	s.chunksPerTick = n
}

// UpdateClientFeedback applies the latest client chunk rate and batch acknowledgement.
func (s *Streamer) UpdateClientFeedback(rate float32, acknowledgement uint64) {
	if s == nil || acknowledgement <= s.lastAcknowledgement {
		return
	}

	acknowledged := acknowledgement - s.lastAcknowledgement
	s.lastAcknowledgement = acknowledgement
	consume := len(s.pendingAckChunks)
	if acknowledged < uint64(consume) {
		consume = int(acknowledged)
	}
	for _, batch := range s.pendingAckChunks[:consume] {
		for _, generated := range batch {
			if elapsed, first := generated.FirstAcknowledgedDuration(); first && s.onChunkAcknowledged != nil {
				s.onChunkAcknowledged(elapsed)
			}
		}
	}
	copy(s.pendingAckChunks, s.pendingAckChunks[consume:])
	for i := len(s.pendingAckChunks) - consume; i < len(s.pendingAckChunks); i++ {
		s.pendingAckChunks[i] = nil
	}
	s.pendingAckChunks = s.pendingAckChunks[:len(s.pendingAckChunks)-consume]
	if s.unacknowledgedBatches == 0 {
		return
	}
	if acknowledged >= uint64(s.unacknowledgedBatches) {
		s.unacknowledgedBatches = 0
	} else {
		s.unacknowledgedBatches -= int(acknowledged)
	}

	if rate < 0.01 {
		rate = 0.01
	}
	if rate > maxClientChunkRate {
		rate = maxClientChunkRate
	}
	s.desiredChunksPerTick = rate
	if s.unacknowledgedBatches == 0 {
		s.batchQuota = 1
	}
	s.maxUnacknowledgedBatches = maxUnacknowledgedBatches
}

// SetViewDistance updates the streamer's view and reconciles queued loads and unloads.
func (s *Streamer) SetViewDistance(distance int32) {
	if s == nil || s.view == nil {
		return
	}
	if distance < 0 {
		distance = 0
	}
	if s.view.ViewDistance() == distance {
		return
	}

	s.view.SetViewDistance(distance)
	s.view.ClearPending()
	for pos := range s.requestedLoads {
		if !s.view.InRange(pos) {
			delete(s.requestedLoads, pos)
		}
	}
	for pos := range s.view.Loaded() {
		if !s.view.InRange(pos) {
			s.pendingUnloads[pos] = struct{}{}
		}
	}
	for _, pos := range ComputeFullLoad(s.view.Center(), distance, s.view.Loaded()) {
		s.view.AddPending(pos)
	}
}

// ChunksPerTick returns the current rate limit.
func (s *Streamer) ChunksPerTick() int {
	return s.chunksPerTick
}

// View returns the player's chunk view state.
func (s *Streamer) View() *View {
	return s.view
}

// ForEachLoaded calls fn for every chunk currently loaded on the client.
func (s *Streamer) ForEachLoaded(fn func(pos ChunkPos)) {
	for pos := range s.view.Loaded() {
		fn(pos)
	}
}

// Close releases streamer resources.
func (s *Streamer) Close() {
	clear(s.pendingAckChunks)
	s.pendingAckChunks = nil
	if s.view != nil {
		s.view.ClearPending()
	}
	clear(s.requestedLoads)
	for pos := range s.pendingUnloads {
		delete(s.pendingUnloads, pos)
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

// SetPlayerViewDistance updates the streaming radius for one registered player.
func (m *Manager) SetPlayerViewDistance(playerID int32, distance int32) {
	if s, ok := m.GetStreamer(playerID); ok {
		s.SetViewDistance(distance)
	}
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

// ForEachLoadedChunk calls fn once for every chunk loaded on any player's client.
// A chunk may be visited multiple times if watched by multiple players.
func (m *Manager) ForEachLoadedChunk(fn func(pos ChunkPos)) {
	for _, s := range m.streamers {
		s.ForEachLoaded(fn)
	}
}

// PlayerCount returns the number of registered streamers.
func (m *Manager) PlayerCount() int {
	return len(m.streamers)
}
