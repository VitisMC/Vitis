package bossbar

import (
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

const (
	ColorPink   int32 = 0
	ColorBlue   int32 = 1
	ColorRed    int32 = 2
	ColorGreen  int32 = 3
	ColorYellow int32 = 4
	ColorPurple int32 = 5
	ColorWhite  int32 = 6
)

const (
	DivisionNone int32 = 0
	Division6    int32 = 1
	Division10   int32 = 2
	Division12   int32 = 3
	Division20   int32 = 4
)

const (
	FlagDarkenSky  byte = 0x01
	FlagDragonBar  byte = 0x02
	FlagCreateFog  byte = 0x04
)

type PacketSender interface {
	Send(pkt protocol.Packet) error
}

type Bar struct {
	UUID     protocol.UUID
	Title    string
	Health   float32
	Color    int32
	Division int32
	Flags    byte
	viewers  map[int64]PacketSender
}

type Manager struct {
	mu   sync.RWMutex
	bars map[protocol.UUID]*Bar
}

func NewManager() *Manager {
	return &Manager{
		bars: make(map[protocol.UUID]*Bar),
	}
}

func (m *Manager) Create(uuid protocol.UUID, title string, color, division int32) *Bar {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.bars[uuid]; exists {
		return m.bars[uuid]
	}
	b := &Bar{
		UUID:     uuid,
		Title:    title,
		Health:   1.0,
		Color:    color,
		Division: division,
		viewers:  make(map[int64]PacketSender),
	}
	m.bars[uuid] = b
	return b
}

func (m *Manager) Remove(uuid protocol.UUID) {
	m.mu.Lock()
	b, ok := m.bars[uuid]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.bars, uuid)
	m.mu.Unlock()

	pkt := &playpacket.BossBar{UUID: uuid, Action: playpacket.BossBarActionRemove}
	for _, s := range b.viewers {
		_ = s.Send(pkt)
	}
}

func (m *Manager) Get(uuid protocol.UUID) *Bar {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bars[uuid]
}

func (m *Manager) List() []protocol.UUID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]protocol.UUID, 0, len(m.bars))
	for id := range m.bars {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) AddViewer(barUUID protocol.UUID, viewerID int64, sender PacketSender) {
	m.mu.RLock()
	b, ok := m.bars[barUUID]
	m.mu.RUnlock()
	if !ok || sender == nil {
		return
	}
	b.viewers[viewerID] = sender
	_ = sender.Send(&playpacket.BossBar{
		UUID:     b.UUID,
		Action:   playpacket.BossBarActionAdd,
		Title:    b.Title,
		Health:   b.Health,
		Color:    b.Color,
		Division: b.Division,
		Flags:    b.Flags,
	})
}

func (m *Manager) RemoveViewer(barUUID protocol.UUID, viewerID int64) {
	m.mu.RLock()
	b, ok := m.bars[barUUID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	sender, exists := b.viewers[viewerID]
	if !exists {
		return
	}
	delete(b.viewers, viewerID)
	_ = sender.Send(&playpacket.BossBar{UUID: barUUID, Action: playpacket.BossBarActionRemove})
}

func (m *Manager) RemoveAllViewers(viewerID int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, b := range m.bars {
		if sender, ok := b.viewers[viewerID]; ok {
			_ = sender.Send(&playpacket.BossBar{UUID: b.UUID, Action: playpacket.BossBarActionRemove})
			delete(b.viewers, viewerID)
		}
	}
}

func (m *Manager) SetTitle(uuid protocol.UUID, title string) {
	m.mu.RLock()
	b, ok := m.bars[uuid]
	m.mu.RUnlock()
	if !ok {
		return
	}
	b.Title = title
	pkt := &playpacket.BossBar{UUID: uuid, Action: playpacket.BossBarActionTitle, Title: title}
	for _, s := range b.viewers {
		_ = s.Send(pkt)
	}
}

func (m *Manager) SetHealth(uuid protocol.UUID, health float32) {
	m.mu.RLock()
	b, ok := m.bars[uuid]
	m.mu.RUnlock()
	if !ok {
		return
	}
	if health < 0 {
		health = 0
	}
	if health > 1 {
		health = 1
	}
	b.Health = health
	pkt := &playpacket.BossBar{UUID: uuid, Action: playpacket.BossBarActionHealth, Health: health}
	for _, s := range b.viewers {
		_ = s.Send(pkt)
	}
}

func (m *Manager) SetStyle(uuid protocol.UUID, color, division int32) {
	m.mu.RLock()
	b, ok := m.bars[uuid]
	m.mu.RUnlock()
	if !ok {
		return
	}
	b.Color = color
	b.Division = division
	pkt := &playpacket.BossBar{UUID: uuid, Action: playpacket.BossBarActionStyle, Color: color, Division: division}
	for _, s := range b.viewers {
		_ = s.Send(pkt)
	}
}

func (m *Manager) SetFlags(uuid protocol.UUID, flags byte) {
	m.mu.RLock()
	b, ok := m.bars[uuid]
	m.mu.RUnlock()
	if !ok {
		return
	}
	b.Flags = flags
	pkt := &playpacket.BossBar{UUID: uuid, Action: playpacket.BossBarActionFlags, Flags: flags}
	for _, s := range b.viewers {
		_ = s.Send(pkt)
	}
}

func (m *Manager) SendAllTo(viewerID int64, sender PacketSender) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, b := range m.bars {
		b.viewers[viewerID] = sender
		_ = sender.Send(&playpacket.BossBar{
			UUID:     b.UUID,
			Action:   playpacket.BossBarActionAdd,
			Title:    b.Title,
			Health:   b.Health,
			Color:    b.Color,
			Division: b.Division,
			Flags:    b.Flags,
		})
	}
}
