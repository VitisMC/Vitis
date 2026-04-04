package scoreboard

import (
	"sync"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

type capturedPacket struct {
	mu   sync.Mutex
	pkts []protocol.Packet
}

func (c *capturedPacket) capture() func(protocol.Packet) {
	return func(pkt protocol.Packet) {
		c.mu.Lock()
		c.pkts = append(c.pkts, pkt)
		c.mu.Unlock()
	}
}

func (c *capturedPacket) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pkts)
}

func TestAddRemoveObjective(t *testing.T) {
	cap := &capturedPacket{}
	sb := New(cap.capture())

	if !sb.AddObjective("kills", "Kills", RenderTypeInteger) {
		t.Fatal("expected AddObjective to succeed")
	}
	if sb.AddObjective("kills", "Kills", RenderTypeInteger) {
		t.Fatal("expected duplicate AddObjective to fail")
	}

	objs := sb.ListObjectives()
	if len(objs) != 1 || objs[0] != "kills" {
		t.Fatalf("expected [kills], got %v", objs)
	}

	if !sb.RemoveObjective("kills") {
		t.Fatal("expected RemoveObjective to succeed")
	}
	if sb.RemoveObjective("kills") {
		t.Fatal("expected duplicate RemoveObjective to fail")
	}

	if cap.count() != 2 {
		t.Fatalf("expected 2 broadcast packets, got %d", cap.count())
	}
}

func TestSetAndGetScore(t *testing.T) {
	sb := New(nil)
	sb.AddObjective("deaths", "Deaths", RenderTypeInteger)

	sb.SetScore("deaths", "player1", 5)
	v, ok := sb.GetScore("deaths", "player1")
	if !ok || v != 5 {
		t.Fatalf("expected score 5, got %d (ok=%v)", v, ok)
	}

	sb.SetScore("deaths", "player1", 10)
	v, _ = sb.GetScore("deaths", "player1")
	if v != 10 {
		t.Fatalf("expected updated score 10, got %d", v)
	}

	sb.ResetScore("deaths", "player1")
	_, ok = sb.GetScore("deaths", "player1")
	if ok {
		t.Fatal("expected score to be reset")
	}
}

func TestResetAllScores(t *testing.T) {
	sb := New(nil)
	sb.AddObjective("a", "A", RenderTypeInteger)
	sb.AddObjective("b", "B", RenderTypeInteger)
	sb.SetScore("a", "p1", 1)
	sb.SetScore("b", "p1", 2)

	sb.ResetAllScores("p1")

	if _, ok := sb.GetScore("a", "p1"); ok {
		t.Fatal("expected score in 'a' to be reset")
	}
	if _, ok := sb.GetScore("b", "p1"); ok {
		t.Fatal("expected score in 'b' to be reset")
	}
}

func TestDisplaySlot(t *testing.T) {
	cap := &capturedPacket{}
	sb := New(cap.capture())
	sb.AddObjective("kills", "Kills", RenderTypeInteger)
	sb.SetDisplaySlot(DisplaySlotSidebar, "kills")

	if cap.count() != 2 {
		t.Fatalf("expected 2 packets (add + display), got %d", cap.count())
	}
}

func TestCreateRemoveTeam(t *testing.T) {
	cap := &capturedPacket{}
	sb := New(cap.capture())

	if !sb.CreateTeam("red", "Red Team") {
		t.Fatal("expected CreateTeam to succeed")
	}
	if sb.CreateTeam("red", "Red Team") {
		t.Fatal("expected duplicate CreateTeam to fail")
	}

	teams := sb.ListTeams()
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}

	if !sb.RemoveTeam("red") {
		t.Fatal("expected RemoveTeam to succeed")
	}

	if cap.count() != 2 {
		t.Fatalf("expected 2 packets, got %d", cap.count())
	}
}

func TestTeamMembers(t *testing.T) {
	sb := New(nil)
	sb.CreateTeam("blue", "Blue Team")

	if !sb.TeamAddMembers("blue", []string{"alice", "bob"}) {
		t.Fatal("expected TeamAddMembers to succeed")
	}

	team := sb.GetTeam("blue")
	if team == nil {
		t.Fatal("expected team to exist")
	}
	if len(team.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(team.Members))
	}

	sb.TeamRemoveMembers("blue", []string{"alice"})
	if len(team.Members) != 1 {
		t.Fatalf("expected 1 member after remove, got %d", len(team.Members))
	}
}

func TestUpdateTeam(t *testing.T) {
	cap := &capturedPacket{}
	sb := New(cap.capture())
	sb.CreateTeam("green", "Green")

	sb.UpdateTeam("green", func(t *Team) {
		t.Prefix = "[G] "
		t.Color = 10
	})

	team := sb.GetTeam("green")
	if team.Prefix != "[G] " || team.Color != 10 {
		t.Fatalf("expected updated team, got prefix=%q color=%d", team.Prefix, team.Color)
	}

	if cap.count() != 2 {
		t.Fatalf("expected 2 packets (create + update), got %d", cap.count())
	}
}

func TestListScores(t *testing.T) {
	sb := New(nil)
	sb.AddObjective("pts", "Points", RenderTypeInteger)
	sb.SetScore("pts", "a", 10)
	sb.SetScore("pts", "b", 20)

	scores := sb.ListScores("pts")
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
	if scores["a"] != 10 || scores["b"] != 20 {
		t.Fatalf("unexpected scores: %v", scores)
	}
}

type mockSender struct {
	mu   sync.Mutex
	pkts []protocol.Packet
}

func (m *mockSender) Send(pkt protocol.Packet) error {
	m.mu.Lock()
	m.pkts = append(m.pkts, pkt)
	m.mu.Unlock()
	return nil
}

func TestSendInitTo(t *testing.T) {
	sb := New(nil)
	sb.AddObjective("kills", "Kills", RenderTypeInteger)
	sb.SetDisplaySlot(DisplaySlotSidebar, "kills")
	sb.SetScore("kills", "player1", 5)
	sb.CreateTeam("red", "Red")
	sb.TeamAddMembers("red", []string{"player1"})

	sender := &mockSender{}
	sb.SendInitTo(sender)

	if len(sender.pkts) < 4 {
		t.Fatalf("expected at least 4 init packets, got %d", len(sender.pkts))
	}
}
