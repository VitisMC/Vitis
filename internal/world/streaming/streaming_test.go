package streaming

import (
	"errors"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
)

type mockProvider struct {
	chunks map[ChunkPos]*chunk.Chunk
}

func newMockProvider() *mockProvider {
	return &mockProvider{chunks: make(map[ChunkPos]*chunk.Chunk)}
}

func (m *mockProvider) GetChunk(x, z int32) (*chunk.Chunk, bool) {
	c, ok := m.chunks[ChunkPos{X: x, Z: z}]
	return c, ok
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
	for x := int32(-2); x <= 2; x++ {
		for z := int32(-2); z <= 2; z++ {
			loaded[ChunkPos{X: x, Z: z}] = struct{}{}
		}
	}

	diff := ComputeDiff(ChunkPos{X: 0, Z: 0}, ChunkPos{X: 1, Z: 0}, 2, loaded)

	if len(diff.ToUnload) == 0 {
		t.Fatal("expected chunks to unload")
	}
	for _, pos := range diff.ToUnload {
		if pos.X >= -1 {
			t.Fatalf("unloaded chunk %v should be out of new range", pos)
		}
	}

	if len(diff.ToLoad) == 0 {
		t.Fatal("expected chunks to load")
	}
	for _, pos := range diff.ToLoad {
		if pos.X != 3 {
			t.Fatalf("loaded chunk %v should be on the new edge", pos)
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

	totalChunks := (2*2 + 1) * (2*2 + 1)
	totalSent := 0
	for tick := 0; tick < 100; tick++ {
		sent, _ := s.Update(0, 0)
		totalSent += sent
	}

	if totalSent != totalChunks {
		t.Fatalf("expected exactly %d total chunks sent, got %d", totalChunks, totalSent)
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

	buf := protocol.WrapBuffer(sender.packets[0].data)
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
	}

	if totalSent != loadedBefore {
		t.Fatalf("expected %d chunks re-sent after force resend, got %d", loadedBefore, totalSent)
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
	if v.InRange(ChunkPos{X: 9, Z: 5}) {
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
