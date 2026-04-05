package inventory

import (
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
)

// Slot represents an item stack in an inventory slot.
type Slot struct {
	ItemCount     int32
	ItemID        int32
	ComponentsAdd int32
	ComponentsRem int32
	RawComponents []byte
}

// Empty returns true if this slot has no item.
func (s Slot) Empty() bool {
	return s.ItemCount <= 0
}

// EmptySlot returns a slot with no item.
func EmptySlot() Slot {
	return Slot{}
}

// NewSlot creates a simple item slot with an item ID and count.
func NewSlot(itemID, count int32) Slot {
	return Slot{ItemCount: count, ItemID: itemID}
}

// EncodeSlot writes a slot to a protocol buffer (1.21.4 structured format).
func EncodeSlot(buf *protocol.Buffer, s Slot) {
	buf.WriteVarInt(s.ItemCount)
	if s.ItemCount <= 0 {
		return
	}
	buf.WriteVarInt(s.ItemID)
	buf.WriteVarInt(s.ComponentsAdd)
	buf.WriteVarInt(s.ComponentsRem)
	if len(s.RawComponents) > 0 {
		buf.WriteBytes(s.RawComponents)
	}
}

// DecodeSlot reads a slot from a protocol buffer (1.21.4 structured format).
// When ComponentsAdd and ComponentsRem are both zero, no additional bytes are read.
// Otherwise, removal component type IDs are skipped and any remaining addition
// component data is captured as RawComponents. Note: this only works reliably
// when the slot is the LAST element in the buffer. For multi-slot sequences,
// use DecodeSlotSkip instead.
func DecodeSlot(buf *protocol.Buffer) (Slot, error) {
	var s Slot
	var err error
	if s.ItemCount, err = buf.ReadVarInt(); err != nil {
		return s, err
	}
	if s.ItemCount <= 0 {
		return EmptySlot(), nil
	}
	if s.ItemID, err = buf.ReadVarInt(); err != nil {
		return s, err
	}
	if s.ComponentsAdd, err = buf.ReadVarInt(); err != nil {
		return s, err
	}
	if s.ComponentsRem, err = buf.ReadVarInt(); err != nil {
		return s, err
	}
	if s.ComponentsAdd == 0 && s.ComponentsRem == 0 {
		return s, nil
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		s.RawComponents, err = buf.ReadBytes(remaining)
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

// DecodeSlotSkip reads a slot header (count, id, components add/rem) and
// discards any component data. Returns only ItemCount and ItemID.
// Safe for use in multi-slot sequences where component boundaries are unknown.
func DecodeSlotSkip(buf *protocol.Buffer) (Slot, error) {
	var s Slot
	var err error
	if s.ItemCount, err = buf.ReadVarInt(); err != nil {
		return s, err
	}
	if s.ItemCount <= 0 {
		return EmptySlot(), nil
	}
	if s.ItemID, err = buf.ReadVarInt(); err != nil {
		return s, err
	}
	addCount, err := buf.ReadVarInt()
	if err != nil {
		return s, err
	}
	remCount, err := buf.ReadVarInt()
	if err != nil {
		return s, err
	}
	if addCount == 0 && remCount == 0 {
		return s, nil
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		_, err = buf.ReadBytes(remaining)
	}
	return s, err
}

const (
	PlayerInventorySize = 46
	HotbarStart         = 36
	HotbarEnd           = 44
	ArmorStart          = 5
	ArmorEnd            = 8
	OffhandSlot         = 45
	CraftOutputSlot     = 0
	CraftInputStart     = 1
	CraftInputEnd       = 4
	MainInventoryStart  = 9
	MainInventoryEnd    = 35
)

// Container holds a fixed number of slots with thread-safe access.
type Container struct {
	mu    sync.RWMutex
	slots []Slot
}

// NewContainer creates a container with the given number of slots.
func NewContainer(size int) *Container {
	return &Container{slots: make([]Slot, size)}
}

// NewPlayerInventory creates a 46-slot player inventory.
func NewPlayerInventory() *Container {
	return NewContainer(PlayerInventorySize)
}

// Size returns the number of slots.
func (c *Container) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.slots)
}

// Get returns the slot at the given index.
func (c *Container) Get(index int) Slot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if index < 0 || index >= len(c.slots) {
		return EmptySlot()
	}
	return c.slots[index]
}

// Set stores a slot at the given index.
func (c *Container) Set(index int, slot Slot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.slots) {
		return
	}
	c.slots[index] = slot
}

// Clear empties a slot.
func (c *Container) Clear(index int) {
	c.Set(index, EmptySlot())
}

// ClearAll empties all slots.
func (c *Container) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.slots {
		c.slots[i] = EmptySlot()
	}
}

// Slots returns a snapshot copy of all slots.
func (c *Container) Slots() []Slot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Slot, len(c.slots))
	copy(out, c.slots)
	return out
}

// HotbarSlot returns the slot in the hotbar at the given offset (0-8).
func (c *Container) HotbarSlot(offset int) Slot {
	return c.Get(HotbarStart + offset)
}

// SetHotbarSlot sets a hotbar slot at the given offset (0-8).
func (c *Container) SetHotbarSlot(offset int, slot Slot) {
	c.Set(HotbarStart+offset, slot)
}

// SwapSlots swaps the contents of two slots atomically.
func (c *Container) SwapSlots(a, b int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a < 0 || a >= len(c.slots) || b < 0 || b >= len(c.slots) {
		return
	}
	c.slots[a], c.slots[b] = c.slots[b], c.slots[a]
}

// FindFirst returns the index of the first slot matching the predicate, or -1.
func (c *Container) FindFirst(pred func(Slot) bool) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i, s := range c.slots {
		if pred(s) {
			return i
		}
	}
	return -1
}
