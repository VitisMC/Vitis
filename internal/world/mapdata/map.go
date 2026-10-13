// Package mapdata implements persistent filled-map state and incremental updates.
package mapdata

import (
	"fmt"
	"sync"

	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

const Size = 128

// Map stores one filled map's metadata, colors, and dirty rectangle.
type Map struct {
	mu        sync.RWMutex
	id        int32
	dimension string
	centerX   int32
	centerZ   int32
	scale     byte
	tracking  bool
	unlimited bool
	locked    bool
	colors    [Size * Size]byte
	dirty     bool
	minX      int
	minY      int
	maxX      int
	maxY      int
}

// New creates a map centered on the scale-aligned grid.
func New(id int32, dimension string, worldX, worldZ int32, scale byte, tracking, unlimited bool) (*Map, error) {
	if id < 0 || dimension == "" || scale > 4 {
		return nil, fmt.Errorf("create map: invalid metadata")
	}
	span := int32(Size) * (int32(1) << scale)
	centerX := floorDiv(worldX+64, span)*span + span/2 - 64
	centerZ := floorDiv(worldZ+64, span)*span + span/2 - 64
	return &Map{id: id, dimension: dimension, centerX: centerX, centerZ: centerZ, scale: scale, tracking: tracking, unlimited: unlimited}, nil
}

// ID returns the map identifier.
func (m *Map) ID() int32 { return m.id }

// Center returns the world center coordinates.
func (m *Map) Center() (int32, int32) {
	if m == nil {
		return 0, 0
	}
	return m.centerX, m.centerZ
}

// Scale returns the map scale from zero through four.
func (m *Map) Scale() byte {
	if m == nil {
		return 0
	}
	return m.scale
}

// Locked reports whether terrain color updates are disabled.
func (m *Map) Locked() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	locked := m.locked
	m.mu.RUnlock()
	return locked
}

// Lock freezes future color changes.
func (m *Map) Lock() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.locked = true
	m.mu.Unlock()
}

// Color returns one pixel color.
func (m *Map) Color(x, y int) (byte, bool) {
	if m == nil || x < 0 || x >= Size || y < 0 || y >= Size {
		return 0, false
	}
	m.mu.RLock()
	color := m.colors[y*Size+x]
	m.mu.RUnlock()
	return color, true
}

// SetColor updates one pixel and expands the dirty rectangle.
func (m *Map) SetColor(x, y int, color byte) bool {
	if m == nil || x < 0 || x >= Size || y < 0 || y >= Size {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	index := y*Size + x
	if m.locked || m.colors[index] == color {
		return false
	}
	m.colors[index] = color
	if !m.dirty {
		m.minX, m.maxX, m.minY, m.maxY = x, x, y, y
		m.dirty = true
	} else {
		m.minX = min(m.minX, x)
		m.maxX = max(m.maxX, x)
		m.minY = min(m.minY, y)
		m.maxY = max(m.maxY, y)
	}
	return true
}

// TakePatch returns and clears the minimal dirty color rectangle.
func (m *Map) TakePatch() *playpacket.MapPatch {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirty {
		return nil
	}
	width, height := m.maxX-m.minX+1, m.maxY-m.minY+1
	colors := make([]byte, width*height)
	for y := 0; y < height; y++ {
		copy(colors[y*width:(y+1)*width], m.colors[(m.minY+y)*Size+m.minX:(m.minY+y)*Size+m.minX+width])
	}
	patch := &playpacket.MapPatch{StartX: byte(m.minX), StartY: byte(m.minY), Width: byte(width), Height: byte(height), Colors: colors}
	m.dirty = false
	return patch
}

// floorDiv divides with negative-infinity rounding.
func floorDiv(value, divisor int32) int32 {
	quotient := value / divisor
	if value%divisor != 0 && (value < 0) != (divisor < 0) {
		quotient--
	}
	return quotient
}
