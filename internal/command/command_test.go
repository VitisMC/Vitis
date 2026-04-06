package command

import (
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

type mockSender struct {
	name     string
	messages []string
	level    int
}

func (m *mockSender) Name() string                          { return m.name }
func (m *mockSender) SendMessage(text string)               { m.messages = append(m.messages, text) }
func (m *mockSender) HasPermission(level int) bool          { return m.level >= level }
func (m *mockSender) IsPlayer() bool                        { return true }
func (m *mockSender) UUID() protocol.UUID                   { return protocol.UUID{} }
func (m *mockSender) EntityID() int32                       { return 1 }
func (m *mockSender) Position() (float64, float64, float64) { return 10.0, 65.0, 20.0 }
func (m *mockSender) GameMode() int32                       { return 1 }

type mockLookup struct {
	players map[string]*mockSender
}

func (m *mockLookup) FindPlayerByName(name string) PlayerSender {
	if p, ok := m.players[name]; ok {
		return p
	}
	return nil
}

func (m *mockLookup) OnlinePlayers() []string {
	names := make([]string, 0, len(m.players))
	for name := range m.players {
		names = append(names, name)
	}
	return names
}

type mockServer struct {
	stopped    bool
	seed       int64
	timeOfDay  int64
	weather    string
	difficulty int
	ops        map[string]int
	kicked     []string
	broadcasts []string
}

func newMockServer() *mockServer {
	return &mockServer{
		seed: 42,
		ops:  make(map[string]int),
	}
}

func (m *mockServer) Stop()                                         { m.stopped = true }
func (m *mockServer) Seed() int64                                   { return m.seed }
func (m *mockServer) SetTime(t int64)                               { m.timeOfDay = t }
func (m *mockServer) GetTime() int64                                { return m.timeOfDay }
func (m *mockServer) SetWeather(w string, _ int)                    { m.weather = w }
func (m *mockServer) SetGameMode(_ int32, _ int32) error            { return nil }
func (m *mockServer) TeleportPlayer(_ int32, _, _, _ float64) error { return nil }
func (m *mockServer) GiveItem(_ int32, _ string, _ int) error       { return nil }
func (m *mockServer) KillEntity(_ int32) error                      { return nil }
func (m *mockServer) SetDifficulty(d int) error                     { m.difficulty = d; return nil }
func (m *mockServer) SetOp(name string, level int) error            { m.ops[name] = level; return nil }
func (m *mockServer) RemoveOp(name string) error                    { delete(m.ops, name); return nil }
func (m *mockServer) KickPlayer(name string, _ string) error {
	m.kicked = append(m.kicked, name)
	return nil
}
func (m *mockServer) BroadcastMessage(msg string)                            { m.broadcasts = append(m.broadcasts, msg) }
func (m *mockServer) EnchantItem(_ int32, _ string, _ int) error             { return nil }
func (m *mockServer) SetBlockAt(_, _, _ int, _ string) (int32, error)        { return 0, nil }
func (m *mockServer) FillBlocks(_, _, _, _, _, _ int, _ string) (int, error) { return 0, nil }
func (m *mockServer) ClearInventory(_ int32, _ string, _ int) (int, error)   { return 0, nil }
func (m *mockServer) GetGameRule(_ string) (string, error)                   { return "true", nil }
func (m *mockServer) SetGameRule(_, _ string) error                          { return nil }
func (m *mockServer) SetDefaultGameMode(_ int32) error                       { return nil }
func (m *mockServer) SetWorldSpawn(_, _, _ int) error                        { return nil }
func (m *mockServer) SetSpawnPoint(_ int32, _, _, _ int) error               { return nil }
func (m *mockServer) SendTitle(_ int32, _, _ string, _, _, _ int) error      { return nil }
func (m *mockServer) SendActionBar(_ int32, _ string) error                  { return nil }
func (m *mockServer) AddXP(_ int32, _ int32) error                           { return nil }
func (m *mockServer) SetXPLevel(_ int32, _ int32) error                      { return nil }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	cmd := &Command{
		Name:    "test",
		Execute: func(ctx *Context) error { return nil },
	}

	if err := r.Register(cmd); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if r.Count() != 1 {
		t.Fatalf("expected 1 command, got %d", r.Count())
	}

	found, ok := r.Get("test")
	if !ok || found.Name != "test" {
		t.Fatalf("get failed")
	}

	if err := r.Register(cmd); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestRegistryAliases(t *testing.T) {
	r := NewRegistry()
	cmd := &Command{
		Name:    "teleport",
		Aliases: []string{"tp"},
		Execute: func(ctx *Context) error { return nil },
	}

	if err := r.Register(cmd); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	found, ok := r.Get("tp")
	if !ok || found.Name != "teleport" {
		t.Fatalf("alias lookup failed")
	}
}

func TestRegistryDispatch(t *testing.T) {
	r := NewRegistry()
	var executed bool
	cmd := &Command{
		Name: "ping",
		Execute: func(ctx *Context) error {
			executed = true
			ctx.Reply("pong")
			return nil
		},
	}
	r.Register(cmd)

	sender := &mockSender{name: "TestPlayer", level: 4}
	if err := r.Dispatch(sender, "ping"); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if !executed {
		t.Fatalf("command not executed")
	}
	if len(sender.messages) != 1 || sender.messages[0] != "pong" {
		t.Fatalf("unexpected messages: %v", sender.messages)
	}
}

func TestRegistryDispatchPermission(t *testing.T) {
	r := NewRegistry()
	cmd := &Command{
		Name:            "admin",
		PermissionLevel: 3,
		Execute:         func(ctx *Context) error { return nil },
	}
	r.Register(cmd)

	sender := &mockSender{name: "Player", level: 0}
	r.Dispatch(sender, "admin")

	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "permission") {
		t.Fatalf("expected permission denied message, got: %v", sender.messages)
	}
}

func TestRegistryDispatchUnknown(t *testing.T) {
	r := NewRegistry()
	sender := &mockSender{name: "Player", level: 4}
	r.Dispatch(sender, "nonexistent")

	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "Unknown command") {
		t.Fatalf("expected unknown command message, got: %v", sender.messages)
	}
}

func TestBuiltinHelp(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{}}
	RegisterBuiltinCommands(r, lookup, server)

	if r.Count() != 28 {
		t.Fatalf("expected 28 commands, got %d", r.Count())
	}

	sender := &mockSender{name: "Player", level: 4}
	r.Dispatch(sender, "help")

	if len(sender.messages) == 0 {
		t.Fatalf("help produced no output")
	}
	if !strings.Contains(sender.messages[0], "Available Commands") {
		t.Fatalf("unexpected help output: %s", sender.messages[0])
	}
}

func TestBuiltinSay(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Admin", level: 4}
	r.Dispatch(sender, "say Hello world")

	if len(server.broadcasts) != 1 || !strings.Contains(server.broadcasts[0], "Hello world") {
		t.Fatalf("expected broadcast, got: %v", server.broadcasts)
	}
}

func TestBuiltinStop(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Admin", level: 4}
	r.Dispatch(sender, "stop")

	if !server.stopped {
		t.Fatalf("server not stopped")
	}
}

func TestBuiltinSeed(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Player", level: 4}
	r.Dispatch(sender, "seed")

	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "42") {
		t.Fatalf("seed output incorrect: %v", sender.messages)
	}
}

func TestBuiltinTimeSet(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Player", level: 4}
	r.Dispatch(sender, "time set day")

	if server.timeOfDay != 1000 {
		t.Fatalf("expected time 1000, got %d", server.timeOfDay)
	}
}

func TestBuiltinDifficulty(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Player", level: 4}
	r.Dispatch(sender, "difficulty hard")

	if server.difficulty != 3 {
		t.Fatalf("expected difficulty 3, got %d", server.difficulty)
	}
}

func TestBuiltinList(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{
		"Alice": {name: "Alice", level: 0},
		"Bob":   {name: "Bob", level: 0},
	}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Player", level: 0}
	r.Dispatch(sender, "list")

	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0], "(2)") {
		t.Fatalf("list output incorrect: %v", sender.messages)
	}
}

func TestBuiltinOpDeop(t *testing.T) {
	r := NewRegistry()
	server := newMockServer()
	lookup := &mockLookup{players: map[string]*mockSender{
		"Bob": {name: "Bob", level: 0},
	}}
	RegisterBuiltinCommands(r, lookup, server)

	sender := &mockSender{name: "Admin", level: 4}
	r.Dispatch(sender, "op Bob")

	if level, ok := server.ops["Bob"]; !ok || level != 4 {
		t.Fatalf("expected Bob op level 4, got: %v", server.ops)
	}

	r.Dispatch(sender, "deop Bob")
	if _, ok := server.ops["Bob"]; ok {
		t.Fatalf("expected Bob to be deopped")
	}
}

func TestContextArgs(t *testing.T) {
	ctx := &Context{
		Args: []string{"hello", "world", "test"},
	}

	if ctx.ArgCount() != 3 {
		t.Fatalf("expected 3 args, got %d", ctx.ArgCount())
	}
	if ctx.Arg(0) != "hello" {
		t.Fatalf("expected hello, got %s", ctx.Arg(0))
	}
	if ctx.Arg(99) != "" {
		t.Fatalf("expected empty for out of bounds, got %s", ctx.Arg(99))
	}
	if ctx.JoinArgs(1) != "world test" {
		t.Fatalf("expected 'world test', got '%s'", ctx.JoinArgs(1))
	}
}

func TestTabSuggestions(t *testing.T) {
	r := NewRegistry()
	r.Register(&Command{
		Name:    "gamemode",
		Execute: func(ctx *Context) error { return nil },
	})
	r.Register(&Command{
		Name:    "give",
		Execute: func(ctx *Context) error { return nil },
	})
	r.Register(&Command{
		Name:    "help",
		Execute: func(ctx *Context) error { return nil },
	})

	suggestions := r.TabSuggestions(nil, "g")
	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions for 'g', got %d: %v", len(suggestions), suggestions)
	}
}

func TestEncodeCommandGraph(t *testing.T) {
	r := NewRegistry()
	r.Register(&Command{
		Name:    "test",
		Execute: func(ctx *Context) error { return nil },
	})

	data := EncodeCommandGraph(r, nil)
	if len(data) == 0 {
		t.Fatalf("expected non-empty graph data")
	}
	if len(data) < 5 {
		t.Fatalf("graph data too short: %d bytes", len(data))
	}
}

func TestParseCoordinates(t *testing.T) {
	sender := &mockSender{name: "Player", level: 0}

	x, y, z, err := parseCoordinates([]string{"100", "65", "200"}, sender)
	if err != nil {
		t.Fatalf("parse absolute coords: %v", err)
	}
	if x != 100 || y != 65 || z != 200 {
		t.Fatalf("expected 100,65,200, got %.0f,%.0f,%.0f", x, y, z)
	}

	x, y, z, err = parseCoordinates([]string{"~", "~10", "~-5"}, sender)
	if err != nil {
		t.Fatalf("parse relative coords: %v", err)
	}
	if x != 10 || y != 75 || z != 15 {
		t.Fatalf("expected 10,75,15, got %.0f,%.0f,%.0f", x, y, z)
	}
}
