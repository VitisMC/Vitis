package streaming

import (
	"errors"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
)

type mockProvider struct {
	chunks   map[ChunkPos]*chunk.Chunk
	requests []ChunkPos
}

func newMockProvider() *mockProvider {
	return &mockProvider{chunks: make(map[ChunkPos]*chunk.Chunk)}
}

func (m *mockProvider) GetChunk(x, z int32) (*chunk.Chunk, bool) {
	c, ok := m.chunks[ChunkPos{X: x, Z: z}]
	return c, ok
}

func (m *mockProvider) RequestLoad(x, z int32) bool {
	m.requests = append(m.requests, ChunkPos{X: x, Z: z})
	return true
}

func (m *mockProvider) addChunk(x, z int32) {
	m.chunks[ChunkPos{X: x, Z: z}] = chunk.New(x, z)
}

type sentPacket struct {
	id   int32
	data []byte
}

type mockSender struct {
	packets   []sentPacket
	failAfter int
	sendCount int
}

func newMockSender() *mockSender {
	return &mockSender{failAfter: -1}
}

func (m *mockSender) Send(pkt protocol.Packet) error {
	if m.failAfter >= 0 && m.sendCount >= m.failAfter {
		return errors.New("send queue full")
	}
	buf := protocol.NewBuffer(64)
	_ = pkt.Encode(buf)
	m.packets = append(m.packets, sentPacket{id: pkt.ID(), data: buf.Bytes()})
	m.sendCount++
	return nil
}

func (m *mockSender) chunkDataCount() int {
	chunkID := playpacket.NewChunkDataAndUpdateLight().ID()
	count := 0
	for _, p := range m.packets {
		if p.id == chunkID {
			count++
		}
	}
	return count
}

func (m *mockSender) unloadCount() int {
	unloadID := playpacket.NewUnloadChunk().ID()
	count := 0
	for _, p := range m.packets {
		if p.id == unloadID {
			count++
		}
	}
	return count
}

func (m *mockSender) reset() {
	m.packets = m.packets[:0]
	m.sendCount = 0
}

// TestStreamerCountsOnlySuccessfulChunkSends verifies the delivery metric callback.
func TestStreamerCountsOnlySuccessfulChunkSends(t *testing.T) {
	provider := newMockProvider()
	provider.addChunk(0, 0)
	sender := newMockSender()
	count := 0
	streamer := NewStreamer(StreamerConfig{
		ChunksPerTick: 1, ViewDistance: 1, Provider: provider, Sender: sender,
		OnChunkSent: func() { count++ },
	})
	streamer.InitialLoad(0, 0)
	streamer.Update(0, 0)
	if count != sender.chunkDataCount() || count != 1 {
		t.Fatalf("callback count=%d packet count=%d", count, sender.chunkDataCount())
	}
}

// TestStreamerAcceptsHighClientChunkRate keeps the negotiated rate above vanilla's cap.
func TestStreamerAcceptsHighClientChunkRate(t *testing.T) {
	streamer := NewStreamer(StreamerConfig{ChunksPerTick: 256})
	streamer.unacknowledgedBatches = 1
	streamer.UpdateClientFeedback(256, 1)
	if streamer.desiredChunksPerTick != 256 {
		t.Fatalf("client rate = %v, want 256", streamer.desiredChunksPerTick)
	}
	streamer.unacknowledgedBatches = 1
	streamer.UpdateClientFeedback(1000, 2)
	if streamer.desiredChunksPerTick != maxClientChunkRate {
		t.Fatalf("client rate = %v, want %v", streamer.desiredChunksPerTick, maxClientChunkRate)
	}
}

// TestStreamerRecordsFullPathOnClientAcknowledgement verifies the completion boundary.
func TestStreamerRecordsFullPathOnClientAcknowledgement(t *testing.T) {
	provider := newMockProvider()
	provider.addChunk(0, 0)
	provider.chunks[ChunkPos{}].MarkGenerationStart(time.Now().Add(-10 * time.Millisecond))
	sender := newMockSender()
	count := 0
	var elapsed time.Duration
	streamer := NewStreamer(StreamerConfig{
		ChunksPerTick: 1, ViewDistance: 1, Provider: provider, Sender: sender,
		OnChunkAcknowledged: func(duration time.Duration) { count++; elapsed = duration },
	})
	streamer.InitialLoad(0, 0)
	streamer.Update(0, 0)
	if count != 0 {
		t.Fatal("full path recorded before client acknowledgement")
	}
	streamer.UpdateClientFeedback(9, 1)
	if count != 1 || elapsed < 10*time.Millisecond {
		t.Fatalf("unexpected acknowledgement measurement: count=%d elapsed=%s", count, elapsed)
	}
	streamer.UpdateClientFeedback(9, 1)
	if count != 1 {
		t.Fatalf("duplicate acknowledgement counted: %d", count)
	}
}

func populateProvider(provider *mockProvider, cx, cz, radius int32) {
	for x := cx - radius; x <= cx+radius; x++ {
		for z := cz - radius; z <= cz+radius; z++ {
			provider.addChunk(x, z)
		}
	}
}

func TestPriorityQueueOrdering(t *testing.T) {
	pq := NewPriorityQueue(16)
	center := ChunkPos{X: 0, Z: 0}

	pq.Enqueue(ChunkPos{X: 3, Z: 0}, ManhattanDistance(ChunkPos{X: 3, Z: 0}, center))
	pq.Enqueue(ChunkPos{X: 0, Z: 0}, ManhattanDistance(ChunkPos{X: 0, Z: 0}, center))
	pq.Enqueue(ChunkPos{X: 1, Z: 1}, ManhattanDistance(ChunkPos{X: 1, Z: 1}, center))
	pq.Enqueue(ChunkPos{X: 0, Z: 1}, ManhattanDistance(ChunkPos{X: 0, Z: 1}, center))

	entry, ok := pq.Dequeue()
	if !ok {
		t.Fatal("expected dequeue to succeed")
	}
	if entry.Pos != (ChunkPos{X: 0, Z: 0}) {
		t.Fatalf("expected center chunk first, got %v", entry.Pos)
	}

	entry, ok = pq.Dequeue()
	if !ok {
		t.Fatal("expected dequeue to succeed")
	}
	if entry.Priority != 1 {
		t.Fatalf("expected priority 1, got %d", entry.Priority)
	}

	entry, _ = pq.Dequeue()
	if entry.Priority != 2 {
		t.Fatalf("expected priority 2, got %d", entry.Priority)
	}

	entry, _ = pq.Dequeue()
	if entry.Priority != 3 {
		t.Fatalf("expected priority 3, got %d", entry.Priority)
	}

	_, ok = pq.Dequeue()
	if ok {
		t.Fatal("expected empty queue")
	}
}

func TestPriorityQueueClear(t *testing.T) {
	pq := NewPriorityQueue(16)
	pq.Enqueue(ChunkPos{X: 1, Z: 2}, 5)
	pq.Enqueue(ChunkPos{X: 3, Z: 4}, 3)
	pq.Clear()

	if pq.Len() != 0 {
		t.Fatalf("expected empty queue after clear, got %d", pq.Len())
	}
}

func TestDiffCompute(t *testing.T) {
	loaded := make(map[ChunkPos]struct{})
	for _, pos := range ComputeFullLoad(ChunkPos{}, 2, nil) {
		loaded[pos] = struct{}{}
	}

	diff := ComputeDiff(ChunkPos{X: 0, Z: 0}, ChunkPos{X: 1, Z: 0}, 2, loaded)

	if len(diff.ToUnload) == 0 {
		t.Fatal("expected chunks to unload")
	}
	for _, pos := range diff.ToUnload {
		if inRange(pos, ChunkPos{X: 1}, 2) {
			t.Fatalf("unloaded chunk %v should be out of new range", pos)
		}
	}

	if len(diff.ToLoad) == 0 {
		t.Fatal("expected chunks to load")
	}
	for _, pos := range diff.ToLoad {
		if !inRange(pos, ChunkPos{X: 1}, 2) || inRange(pos, ChunkPos{}, 2) {
			t.Fatalf("loaded chunk %v is not exclusive to the new view", pos)
		}
	}
}

func TestDiffSamePosition(t *testing.T) {
	loaded := make(map[ChunkPos]struct{})
	loaded[ChunkPos{X: 0, Z: 0}] = struct{}{}

	diff := ComputeDiff(ChunkPos{X: 5, Z: 5}, ChunkPos{X: 5, Z: 5}, 2, loaded)
	if len(diff.ToLoad) != 0 || len(diff.ToUnload) != 0 {
		t.Fatal("expected empty diff for same position")
	}
}

func TestComputeFullLoad(t *testing.T) {
	loaded := make(map[ChunkPos]struct{})
	loaded[ChunkPos{X: 0, Z: 0}] = struct{}{}

	result := ComputeFullLoad(ChunkPos{X: 0, Z: 0}, 1, loaded)

	expected := (2*1+1)*(2*1+1) - 1
	if len(result) != expected {
		t.Fatalf("expected %d chunks, got %d", expected, len(result))
	}

	for _, pos := range result {
		if pos == (ChunkPos{X: 0, Z: 0}) {
			t.Fatal("already loaded chunk should not appear in full load result")
		}
	}
}

func TestRateLimiting(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	populateProvider(provider, 0, 0, 10)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 3,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(0, 0)
	sent, _ := s.Update(0, 0)

	if sent > 3 {
		t.Fatalf("expected at most 3 chunks sent, got %d", sent)
	}
	if sender.chunkDataCount() > 3 {
		t.Fatalf("expected at most 3 chunk data packets, got %d", sender.chunkDataCount())
	}
}

func TestZeroChunkRatePausesNewChunkDelivery(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 1)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 2,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)
	s.SetChunksPerTick(0)

	sent, _ := s.Update(0, 0)
	if sent != 0 || sender.chunkDataCount() != 0 {
		t.Fatalf("zero rate sent %d chunks", sent)
	}
	if s.ChunksPerTick() != 0 {
		t.Fatalf("rate = %d, want 0", s.ChunksPerTick())
	}
}

func TestViewDistanceChangeReconcilesStreamingState(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 2)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 100,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)
	s.Update(0, 0)
	if s.View().LoadedCount() != 9 {
		t.Fatalf("loaded chunks = %d, want 9", s.View().LoadedCount())
	}

	s.SetViewDistance(0)
	_, unloaded := s.Update(0, 0)
	if unloaded != 8 {
		t.Fatalf("unloaded chunks = %d, want 8", unloaded)
	}
	if s.View().LoadedCount() != 1 {
		t.Fatalf("loaded chunks = %d, want 1", s.View().LoadedCount())
	}
}

func TestNoDuplicateSends(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	populateProvider(provider, 0, 0, 5)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 100,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(0, 0)

	totalChunks := len(ComputeFullLoad(ChunkPos{}, 2, nil))
	totalSent := 0
	for tick := 0; tick < 100; tick++ {
		sent, _ := s.Update(0, 0)
		totalSent += sent
		s.UpdateClientFeedback(64, uint64(tick+1))
	}

	if totalSent != totalChunks {
		t.Fatalf("expected exactly %d total chunks sent, got %d", totalChunks, totalSent)
	}
}

func TestReadyChunksSustainConfiguredDeliveryRate(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 20)
	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 32,
		ViewDistance:  20,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)

	total := 0
	for tick := 0; tick < 20; tick++ {
		sent, _ := s.Update(0, 0)
		if tick == 0 && sent != 9 {
			t.Fatalf("initial batch sent %d chunks, want 9", sent)
		}
		if tick > 0 && sent != 32 {
			t.Fatalf("tick %d sent %d chunks, want 32", tick, sent)
		}
		total += sent
		s.UpdateClientFeedback(32, uint64(tick+1))
	}
	if want := 9 + 19*32; total != want {
		t.Fatalf("sent %d chunks in 20 ticks, want %d", total, want)
	}
}

func TestUnloadOnMove(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	populateProvider(provider, 0, 0, 15)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 200,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(0, 0)
	for tick := 0; tick < 10; tick++ {
		s.Update(0, 0)
	}

	sender.reset()

	_, unloaded := s.Update(5, 0)
	if unloaded == 0 {
		t.Fatal("expected chunks to be unloaded on move")
	}
	if sender.unloadCount() == 0 {
		t.Fatal("expected unload packets sent")
	}
}

func TestMoveSendsCenterBeforeChunkStateChanges(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 8)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  0,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)
	s.View().MarkLoaded(ChunkPos{})

	s.Update(2, -3)
	if len(sender.packets) == 0 {
		t.Fatal("expected center packet on chunk boundary crossing")
	}
	if sender.packets[0].id != playpacket.NewSetCenterChunk().ID() {
		t.Fatalf("first packet id = %d, want set center chunk", sender.packets[0].id)
	}

	buf := protocol.WrapBuffer(sender.packets[0].data)
	x, err := buf.ReadVarInt()
	if err != nil {
		t.Fatalf("read center x: %v", err)
	}
	z, err := buf.ReadVarInt()
	if err != nil {
		t.Fatalf("read center z: %v", err)
	}
	if x != 2 || z != -3 {
		t.Fatalf("center = (%d,%d), want (2,-3)", x, z)
	}
}

func TestCenterBackpressureDoesNotAdvanceStreamingView(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	sender.failAfter = 0

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  0,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)
	s.View().MarkLoaded(ChunkPos{})

	sent, unloaded := s.Update(1, 0)
	if sent != 0 || unloaded != 0 {
		t.Fatalf("failed center update changed chunks: sent=%d unloaded=%d", sent, unloaded)
	}
	if center := s.View().Center(); center != (ChunkPos{}) {
		t.Fatalf("view advanced after failed center packet: %v", center)
	}
	if !s.View().IsLoaded(ChunkPos{}) {
		t.Fatal("loaded state changed after failed center packet")
	}

	sender.failAfter = -1
	s.Update(1, 0)
	if center := s.View().Center(); center != (ChunkPos{X: 1}) {
		t.Fatalf("view did not advance after center retry: %v", center)
	}
}

func TestBackpressure(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	sender.failAfter = 2

	populateProvider(provider, 0, 0, 10)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 10,
		ViewDistance:  3,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(0, 0)
	sent, _ := s.Update(0, 0)

	if sent > 2 {
		t.Fatalf("expected at most 2 chunks sent before backpressure, got %d", sent)
	}

	sender.failAfter = -1
	sender.reset()

	sent2, _ := s.Update(0, 0)
	if sent2 == 0 {
		t.Fatal("expected streaming to resume after backpressure clears")
	}
}

func TestBackpressureRetriesEveryUnsentChunk(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	sender.failAfter = 0
	provider.addChunk(0, 0)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})
	s.View().SetViewDistance(0)
	s.InitialLoad(0, 0)

	if sent, _ := s.Update(0, 0); sent != 0 {
		t.Fatalf("expected no sends while queue is full, got %d", sent)
	}
	if !s.View().IsPending(ChunkPos{}) {
		t.Fatal("failed chunk must remain pending for retry")
	}

	sender.failAfter = -1
	if sent, _ := s.Update(0, 0); sent != 1 {
		t.Fatalf("expected the failed chunk to be retried, got %d sends", sent)
	}
	if !s.View().IsLoaded(ChunkPos{}) {
		t.Fatal("retried chunk was not marked loaded")
	}
}

func TestMissingChunkRequestsLoad(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  0,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)

	s.Update(0, 0)
	if len(provider.requests) == 0 || provider.requests[0] != (ChunkPos{}) {
		t.Fatalf("expected a load request for the missing center chunk, got %v", provider.requests)
	}
	if !s.View().IsPending(ChunkPos{}) {
		t.Fatal("missing chunk must remain pending")
	}
}

func TestMissingChunkRequestsAreBoundedAndDeduplicated(t *testing.T) {
	provider := newMockProvider()
	streamer := NewStreamer(StreamerConfig{
		ChunksPerTick: 16,
		ViewDistance:  32,
		Provider:      provider,
		Sender:        newMockSender(),
	})
	streamer.InitialLoad(0, 0)
	want := streamer.loadRequestWindow()

	streamer.Update(0, 0)
	if got := len(provider.requests); got != want {
		t.Fatalf("load requests = %d, want %d", got, want)
	}
	streamer.Update(0, 0)
	if got := len(provider.requests); got != want {
		t.Fatalf("duplicate load requests increased count to %d", got)
	}
}

func TestPrefetchWindowTracksClientRate(t *testing.T) {
	streamer := NewStreamer(StreamerConfig{ChunksPerTick: 32})
	if got := streamer.loadRequestWindow(); got != 180 {
		t.Fatalf("initial request window = %d, want 180", got)
	}
	streamer.desiredChunksPerTick = 4
	if got := streamer.loadRequestWindow(); got != 80 {
		t.Fatalf("request window at four chunks per tick = %d, want 80", got)
	}
	streamer.desiredChunksPerTick = 0.01
	if got := streamer.loadRequestWindow(); got != minimumLoadRequestWindow {
		t.Fatalf("minimum request window = %d, want %d", got, minimumLoadRequestWindow)
	}
}

func TestViewPrioritizesMovementDirectionWithinDistance(t *testing.T) {
	view := NewView(ChunkPos{}, 2)
	view.SetDirection(1, 0)
	for _, pos := range []ChunkPos{{X: -1}, {X: 1}, {Z: -1}, {Z: 1}, {}} {
		view.AddPending(pos)
	}
	if pos, ok := view.PopPending(); !ok || pos != (ChunkPos{}) {
		t.Fatalf("first chunk = %v, want center", pos)
	}
	if pos, ok := view.PopPending(); !ok || pos != (ChunkPos{X: 1}) {
		t.Fatalf("second chunk = %v, want forward chunk", pos)
	}
}

func TestOverlappingMoveKeepsLoadReservations(t *testing.T) {
	provider := newMockProvider()
	streamer := NewStreamer(StreamerConfig{ChunksPerTick: 16, ViewDistance: 32, Provider: provider, Sender: newMockSender()})
	streamer.InitialLoad(0, 0)
	streamer.Update(0, 0)
	firstRequests := len(provider.requests)
	streamer.Update(1, 0)
	if got := len(provider.requests); got != firstRequests {
		t.Fatalf("overlapping move repeated %d requests", got-firstRequests)
	}
}

func TestCenterMoveKeepsUnsentOverlapQueued(t *testing.T) {
	provider := newMockProvider()
	streamer := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        newMockSender(),
	})
	streamer.InitialLoad(0, 0)
	streamer.Update(0, 0)
	streamer.Update(1, 0)

	if got, want := streamer.View().PendingCount(), len(ComputeFullLoad(ChunkPos{X: 1}, 2, nil)); got != want {
		t.Fatalf("pending chunks after center move = %d, want %d", got, want)
	}
	if !streamer.View().IsPending(ChunkPos{X: 0, Z: 0}) {
		t.Fatal("unsent chunk in overlapping view was lost")
	}
}

func TestCenterMoveRefreshesLoadReservationWindow(t *testing.T) {
	provider := newMockProvider()
	streamer := NewStreamer(StreamerConfig{
		ChunksPerTick: 16,
		ViewDistance:  32,
		Provider:      provider,
		Sender:        newMockSender(),
	})
	streamer.InitialLoad(0, 0)
	want := streamer.loadRequestWindow()
	streamer.Update(0, 0)
	if got := len(provider.requests); got != want {
		t.Fatalf("initial load requests = %d, want %d", got, want)
	}

	streamer.Update(100, 0)
	if got := len(provider.requests); got != 2*want {
		t.Fatalf("load requests after center move = %d, want %d", got, 2*want)
	}
	requestedNearNewCenter := false
	for _, pos := range provider.requests[want:] {
		if DistanceSquared(pos, ChunkPos{X: 100, Z: 0}) <= 4 {
			requestedNearNewCenter = true
			break
		}
	}
	if !requestedNearNewCenter {
		t.Fatal("center move did not reserve generation near the new center")
	}
}

func TestChunkPrefetchContinuesWhileDeliveryIsBackpressured(t *testing.T) {
	provider := newMockProvider()
	populateProvider(provider, 0, 0, 2)
	streamer := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        newMockSender(),
	})
	streamer.InitialLoad(0, 0)
	streamer.Update(0, 0)
	if streamer.unacknowledgedBatches == 0 {
		t.Fatal("expected an unacknowledged delivery batch")
	}

	delete(provider.chunks, ChunkPos{X: 0, Z: -1})
	streamer.Update(0, 0)
	if len(provider.requests) == 0 {
		t.Fatal("chunk generation stopped while delivery was backpressured")
	}
}

func TestViewBalancesEqualDistanceDirections(t *testing.T) {
	view := NewView(ChunkPos{}, 2)
	for _, pos := range ComputeFullLoad(ChunkPos{}, 2, nil) {
		view.AddPending(pos)
	}
	center, ok := view.PopPending()
	if !ok || center != (ChunkPos{}) {
		t.Fatalf("first chunk = %v, %t", center, ok)
	}
	first, ok := view.PopPending()
	if !ok {
		t.Fatal("missing first radial chunk")
	}
	second, ok := view.PopPending()
	if !ok {
		t.Fatal("missing opposite radial chunk")
	}
	if first.X+second.X != 0 || first.Z+second.Z != 0 {
		t.Fatalf("equal-distance chunks are not balanced: %v and %v", first, second)
	}
}

func TestPoppedOutOfRangeChunkCanBeQueuedAgain(t *testing.T) {
	v := NewView(ChunkPos{}, 0)
	pos := ChunkPos{X: 1}
	v.AddPending(pos)
	v.SetCenter(ChunkPos{X: 10})

	popped, ok := v.PopPending()
	if !ok || popped != pos {
		t.Fatalf("expected queued chunk, got %v, %t", popped, ok)
	}
	if v.IsPending(pos) {
		t.Fatal("popped entry must not retain its pending marker")
	}
	v.SetCenter(ChunkPos{})
	v.AddPending(pos)
	if !v.IsPending(pos) {
		t.Fatal("returning to range must allow the chunk to be queued again")
	}
}

func TestUnloadBackpressureRetriesWithoutForgettingClientState(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 8)
	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 100,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})
	s.View().SetViewDistance(0)
	s.InitialLoad(0, 0)
	s.Update(0, 0)
	if !s.View().IsLoaded(ChunkPos{}) {
		t.Fatal("expected initial chunk to be loaded")
	}

	sender.reset()
	sender.failAfter = 0
	if _, unloaded := s.Update(1, 0); unloaded != 0 {
		t.Fatalf("expected unload to wait for accepted packet, got %d", unloaded)
	}
	if !s.View().IsLoaded(ChunkPos{}) {
		t.Fatal("client state was dropped despite failed unload packet")
	}

	sender.failAfter = -1
	if _, unloaded := s.Update(1, 0); unloaded != 1 {
		t.Fatalf("expected one retried unload, got %d", unloaded)
	}
	if s.View().IsLoaded(ChunkPos{}) {
		t.Fatal("client state was not removed after successful unload")
	}
}

func TestChunkNotReadySkipped(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	provider.addChunk(0, 0)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 100,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(0, 0)
	sent, _ := s.Update(0, 0)

	if sent != 1 {
		t.Fatalf("expected 1 chunk sent (only one available), got %d", sent)
	}

	provider.addChunk(1, 0)
	provider.addChunk(-1, 0)
	provider.addChunk(0, 1)
	provider.addChunk(0, -1)

	sender.reset()
	s.UpdateClientFeedback(9, 1)
	sent2, _ := s.Update(0, 0)
	if sent2 < 1 {
		t.Fatal("expected previously skipped chunks to be sent when available")
	}
}

func TestCenterChunkSentFirst(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	populateProvider(provider, 5, 5, 10)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 1,
		ViewDistance:  3,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(5, 5)
	s.Update(5, 5)

	if len(sender.packets) == 0 {
		t.Fatal("expected at least one packet sent")
	}

	chunkID := playpacket.NewChunkDataAndUpdateLight().ID()
	var chunkPacket sentPacket
	for _, packet := range sender.packets {
		if packet.id == chunkID {
			chunkPacket = packet
			break
		}
	}
	if chunkPacket.data == nil {
		t.Fatal("expected a chunk data packet")
	}
	buf := protocol.WrapBuffer(chunkPacket.data)
	cx, _ := buf.ReadInt32()
	cz, _ := buf.ReadInt32()

	if cx != 5 || cz != 5 {
		t.Fatalf("expected center chunk (5,5) sent first, got (%d,%d)", cx, cz)
	}
}

func TestManagerTickAllPlayers(t *testing.T) {
	provider := newMockProvider()
	sender1 := newMockSender()
	sender2 := newMockSender()

	populateProvider(provider, 0, 0, 10)
	populateProvider(provider, 10, 10, 10)

	mgr := NewManager()

	s1 := mgr.AddPlayer(1, StreamerConfig{
		ChunksPerTick: 5,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        sender1,
	})
	s1.InitialLoad(0, 0)

	s2 := mgr.AddPlayer(2, StreamerConfig{
		ChunksPerTick: 5,
		ViewDistance:  2,
		Provider:      provider,
		Sender:        sender2,
	})
	s2.InitialLoad(10, 10)

	totalSent, _ := mgr.Tick(func(playerID int32) (int32, int32, bool) {
		switch playerID {
		case 1:
			return 0, 0, true
		case 2:
			return 10, 10, true
		}
		return 0, 0, false
	})

	if totalSent == 0 {
		t.Fatal("expected chunks sent for at least one player")
	}
	if sender1.chunkDataCount() == 0 {
		t.Fatal("expected chunks sent for player 1")
	}
	if sender2.chunkDataCount() == 0 {
		t.Fatal("expected chunks sent for player 2")
	}
}

func TestManagerRemovePlayerSendsUnloads(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	populateProvider(provider, 0, 0, 5)

	mgr := NewManager()
	s := mgr.AddPlayer(1, StreamerConfig{
		ChunksPerTick: 100,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)

	for i := 0; i < 5; i++ {
		mgr.Tick(func(playerID int32) (int32, int32, bool) {
			return 0, 0, true
		})
	}

	sender.reset()
	mgr.RemovePlayer(1, sender)

	if sender.unloadCount() == 0 {
		t.Fatal("expected unload packets on player removal")
	}
	if mgr.PlayerCount() != 0 {
		t.Fatal("expected zero players after removal")
	}
}

func TestForceResend(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()

	populateProvider(provider, 0, 0, 5)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 100,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})

	s.InitialLoad(0, 0)
	for i := 0; i < 5; i++ {
		s.Update(0, 0)
		s.UpdateClientFeedback(64, uint64(i+1))
	}

	loadedBefore := s.View().LoadedCount()
	if loadedBefore == 0 {
		t.Fatal("expected loaded chunks before force resend")
	}

	sender.reset()
	s.ForceResend()

	totalSent := 0
	for i := 0; i < 5; i++ {
		sent, _ := s.Update(0, 0)
		totalSent += sent
		s.UpdateClientFeedback(64, uint64(i+6))
	}

	if totalSent != loadedBefore {
		t.Fatalf("expected %d chunks re-sent after force resend, got %d", loadedBefore, totalSent)
	}
}

func TestForceResendClearsOldBatchBackpressure(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 5)
	streamer := NewStreamer(StreamerConfig{ChunksPerTick: 4, ViewDistance: 1, Provider: provider, Sender: sender})
	streamer.InitialLoad(0, 0)
	if sent, _ := streamer.Update(0, 0); sent == 0 {
		t.Fatal("initial chunk batch was not sent")
	}
	streamer.ForceResend()
	if sent, _ := streamer.Update(0, 0); sent == 0 {
		t.Fatal("respawn chunk batch remained blocked by old acknowledgement")
	}
}

func TestChunkDeliveryWaitsForBatchAcknowledgement(t *testing.T) {
	provider := newMockProvider()
	sender := newMockSender()
	populateProvider(provider, 0, 0, 2)

	s := NewStreamer(StreamerConfig{
		ChunksPerTick: 2,
		ViewDistance:  1,
		Provider:      provider,
		Sender:        sender,
	})
	s.InitialLoad(0, 0)

	if sent, _ := s.Update(0, 0); sent != 2 {
		t.Fatalf("first batch sent %d chunks, want 2", sent)
	}
	if sent, _ := s.Update(0, 0); sent != 0 {
		t.Fatalf("sent %d chunks before acknowledgement", sent)
	}
	s.UpdateClientFeedback(2, 1)
	if sent, _ := s.Update(0, 0); sent != 2 {
		t.Fatalf("second batch sent %d chunks after acknowledgement, want 2", sent)
	}

	startID := playpacket.NewChunkBatchStart().ID()
	finishID := playpacket.NewChunkBatchFinished().ID()
	if sender.packets[0].id != startID || sender.packets[3].id != finishID {
		t.Fatalf("unexpected first batch packet sequence: %v", sender.packets)
	}
}

func TestViewInRange(t *testing.T) {
	v := NewView(ChunkPos{X: 5, Z: 5}, 3)

	if !v.InRange(ChunkPos{X: 5, Z: 5}) {
		t.Fatal("center should be in range")
	}
	if !v.InRange(ChunkPos{X: 8, Z: 5}) {
		t.Fatal("edge chunk should be in range")
	}
	if v.InRange(ChunkPos{X: 10, Z: 5}) {
		t.Fatal("chunk beyond view distance should be out of range")
	}
}

func TestManhattanDistance(t *testing.T) {
	d := ManhattanDistance(ChunkPos{X: 0, Z: 0}, ChunkPos{X: 3, Z: 4})
	if d != 7 {
		t.Fatalf("expected manhattan distance 7, got %d", d)
	}

	d = ManhattanDistance(ChunkPos{X: -2, Z: 3}, ChunkPos{X: 1, Z: -1})
	if d != 7 {
		t.Fatalf("expected manhattan distance 7, got %d", d)
	}
}
