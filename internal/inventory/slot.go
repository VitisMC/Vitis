package inventory

import (
	"bytes"
	"fmt"
	"maps"
	"sync"

	datacomponent "github.com/vitismc/vitis/internal/data/generated/data_component"
	itemdata "github.com/vitismc/vitis/internal/data/generated/item"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// ItemStack represents an item stack and its 1.21.4 data-component patch.
type ItemStack struct {
	ItemCount      int32
	ItemID         int32
	Damage         int32
	ComponentsAdd  int32
	ComponentsRem  int32
	RawComponents  []byte
	ComponentPatch ComponentPatch
	Enchantments   map[int32]int32
	PersistentData *nbt.Compound
}

// Slot is a compatibility alias for ItemStack.
// Deprecated: use ItemStack in new APIs.
type Slot = ItemStack

// Clone returns an independent stack value suitable for crossing an inventory
// synchronization boundary.
func (s ItemStack) Clone() ItemStack {
	s.RawComponents = bytes.Clone(s.RawComponents)
	s.ComponentPatch = s.ComponentPatch.Clone()
	s.Enchantments = maps.Clone(s.Enchantments)
	s.PersistentData = s.PersistentData.Clone()
	return s
}

// Empty returns true if this slot has no item.
func (s ItemStack) Empty() bool {
	return s.ItemCount <= 0
}

// EnchantLevel returns the level of an enchantment on this item, or 0.
func (s ItemStack) EnchantLevel(enchID int32) int32 {
	if s.Enchantments == nil {
		return 0
	}
	return s.Enchantments[enchID]
}

// SetEnchant sets an enchantment on this slot. Level <= 0 removes it.
func (s *ItemStack) SetEnchant(enchID, level int32) {
	if level <= 0 {
		if s.Enchantments != nil {
			delete(s.Enchantments, enchID)
			s.syncEnchantmentsComponent()
		}
		return
	}
	if s.Enchantments == nil {
		s.Enchantments = make(map[int32]int32)
	}
	s.Enchantments[enchID] = level
	s.syncEnchantmentsComponent()
}

// SetDamage updates both the gameplay damage field and its wire component.
func (s *ItemStack) SetDamage(damage int32) {
	if damage < 0 {
		damage = 0
	}
	s.Damage = damage
	s.clearRemoved(datacomponent.ComponentDamage)
	for i := range s.ComponentPatch.Added {
		if s.ComponentPatch.Added[i].TypeID == datacomponent.ComponentDamage {
			s.ComponentPatch.Added[i].Value = VarIntComponent(damage)
			s.ComponentPatch.Added[i].Raw = nil
			return
		}
	}
	if damage != 0 {
		s.ComponentPatch.Added = append(s.ComponentPatch.Added, DataComponent{
			TypeID: datacomponent.ComponentDamage,
			Value:  VarIntComponent(damage),
		})
	}
}

func (s *ItemStack) syncEnchantmentsComponent() {
	s.clearRemoved(datacomponent.ComponentEnchantments)
	for i := range s.ComponentPatch.Added {
		if s.ComponentPatch.Added[i].TypeID != datacomponent.ComponentEnchantments {
			continue
		}
		show := true
		if current, ok := s.ComponentPatch.Added[i].Value.(EnchantmentsComponent); ok {
			show = current.ShowInTooltip
		}
		s.ComponentPatch.Added[i].Value = EnchantmentsComponent{Levels: maps.Clone(s.Enchantments), ShowInTooltip: show}
		s.ComponentPatch.Added[i].Raw = nil
		return
	}
	s.ComponentPatch.Added = append(s.ComponentPatch.Added, DataComponent{
		TypeID: datacomponent.ComponentEnchantments,
		Value:  EnchantmentsComponent{Levels: maps.Clone(s.Enchantments), ShowInTooltip: true},
	})
}

// EmptySlot returns a slot with no item.
func EmptySlot() ItemStack {
	return ItemStack{}
}

// NewSlot creates a simple item slot with an item ID and count.
func NewSlot(itemID, count int32) ItemStack {
	return ItemStack{ItemCount: count, ItemID: itemID}
}

// StackableWith reports whether two stacks have identical item data.
func (s ItemStack) StackableWith(other ItemStack) bool {
	if s.ItemID != other.ItemID {
		return false
	}
	ids := map[int32]struct{}{}
	for _, patch := range []ComponentPatch{s.componentPatchForEncoding(), other.componentPatchForEncoding()} {
		for _, entry := range patch.Added {
			ids[entry.TypeID] = struct{}{}
		}
		for _, id := range patch.Removed {
			ids[id] = struct{}{}
		}
	}
	for id := range ids {
		a, ok := s.Component(id)
		b, otherOK := other.Component(id)
		if ok != otherOK || ok && !componentValuesEqual(a, b) {
			return false
		}
	}
	return persistentExtrasEqual(s.PersistentData, other.PersistentData)
}

// MaxStackSize returns the effective maximum count for this stack.
func (s ItemStack) MaxStackSize() int32 {
	for _, id := range s.ComponentPatch.Removed {
		if id == datacomponent.ComponentMaxStackSize {
			return 1
		}
	}
	if value, ok := s.Component(datacomponent.ComponentMaxStackSize); ok {
		if n, valid := value.(VarIntComponent); valid && n > 0 && n <= 99 {
			return int32(n)
		}
	}
	return item.StackSize(s.ItemID)
}

// Component returns an explicitly added component, accounting for removals.
func (s ItemStack) Component(typeID int32) (ComponentValue, bool) {
	for _, removed := range s.ComponentPatch.Removed {
		if removed == typeID {
			return nil, false
		}
	}
	for _, entry := range s.componentPatchForEncoding().Added {
		if entry.TypeID == typeID {
			return cloneComponentValue(entry.Value), true
		}
	}
	return defaultItemComponent(s.ItemID, typeID)
}

// SetComponent replaces a patch value and clears stale wire data and removals.
func (s *ItemStack) SetComponent(typeID int32, value ComponentValue) {
	if typeID == datacomponent.ComponentDamage {
		if damage, ok := value.(VarIntComponent); ok {
			s.Damage = int32(damage)
		}
	}
	if typeID == datacomponent.ComponentEnchantments {
		if enchantments, ok := value.(EnchantmentsComponent); ok {
			s.Enchantments = maps.Clone(enchantments.Levels)
		}
	}
	for i := range s.ComponentPatch.Added {
		if s.ComponentPatch.Added[i].TypeID == typeID {
			s.ComponentPatch.Added[i] = DataComponent{TypeID: typeID, Value: cloneComponentValue(value)}
			s.clearRemoved(typeID)
			return
		}
	}
	s.ComponentPatch.Added = append(s.ComponentPatch.Added, DataComponent{TypeID: typeID, Value: cloneComponentValue(value)})
	s.clearRemoved(typeID)
}

// clearRemoved removes a component from the explicit deletion set.
func (s *ItemStack) clearRemoved(typeID int32) {
	result := s.ComponentPatch.Removed[:0]
	for _, id := range s.ComponentPatch.Removed {
		if id != typeID {
			result = append(result, id)
		}
	}
	s.ComponentPatch.Removed = result
}

// EncodeItemStack writes a validated item stack and its bounded component patch.
func EncodeItemStack(buf *protocol.Buffer, s ItemStack) error {
	return encodeItemStack(buf, s, 0)
}

// encodeItemStack rejects recursive component graphs before encoding them.
func encodeItemStack(buf *protocol.Buffer, s ItemStack, depth int) error {
	if depth > maxNestedItemStackDepth {
		return fmt.Errorf("nested item stack depth exceeds %d", maxNestedItemStackDepth)
	}
	if s.ItemCount < 0 {
		return fmt.Errorf("negative item count: %d", s.ItemCount)
	}
	tmp := protocol.NewBuffer(128)
	tmp.WriteVarInt(s.ItemCount)
	if s.ItemCount == 0 {
		buf.WriteBytes(tmp.Bytes())
		return nil
	}
	if s.ItemCount > 99 {
		return fmt.Errorf("item count out of range: %d", s.ItemCount)
	}
	if s.ItemID <= 0 || s.ItemID >= itemdata.ItemCount {
		return fmt.Errorf("item ID out of range: %d", s.ItemID)
	}
	tmp.WriteVarInt(s.ItemID)
	patch := protocol.NewBuffer(128)
	if err := encodeComponentPatch(patch, s.componentPatchForEncoding(), depth); err != nil {
		return err
	}
	tmp.WriteBytes(patch.Bytes())
	buf.WriteBytes(tmp.Bytes())
	return nil
}

// EncodeSlot writes an item stack. Deprecated packet implementations should
// migrate to EncodeItemStack so validation errors can be returned.
func EncodeSlot(buf *protocol.Buffer, s ItemStack) {
	_ = EncodeItemStack(buf, s)
}

// DecodeSlot reads a bounded item stack from a protocol buffer.
func DecodeSlot(buf *protocol.Buffer) (ItemStack, error) {
	return DecodeItemStack(buf)
}

// DecodeSlotSkip reads a bounded item stack.
func DecodeSlotSkip(buf *protocol.Buffer) (ItemStack, error) {
	return DecodeItemStack(buf)
}

// DecodeSlotWithoutComponents is retained for compatibility and decodes components.
func DecodeSlotWithoutComponents(buf *protocol.Buffer) (ItemStack, error) {
	return DecodeItemStack(buf)
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
	return c.slots[index].Clone()
}

// Set stores a slot at the given index.
func (c *Container) Set(index int, slot Slot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.slots) {
		return
	}
	c.slots[index] = slot.Clone()
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
	for i := range c.slots {
		out[i] = c.slots[i].Clone()
	}
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
		if pred(s.Clone()) {
			return i
		}
	}
	return -1
}

// Insert adds as much of stack as possible to the selected slots. Existing
// compatible stacks are filled before empty slots. An empty slot list selects
// the entire container. The returned stack is the uninserted remainder.
func (c *Container) Insert(stack ItemStack, maxStack int32, slots []int) (ItemStack, []int) {
	maxStack = min(maxStack, stack.MaxStackSize())
	if stack.Empty() || maxStack <= 0 {
		return stack.Clone(), nil
	}
	if stack.ItemCount < 0 {
		return stack.Clone(), nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	indices := slots
	if len(indices) == 0 {
		indices = make([]int, len(c.slots))
		for i := range indices {
			indices[i] = i
		}
	}
	valid := make([]int, 0, len(indices))
	seen := make(map[int]struct{}, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(c.slots) {
			continue
		}
		if _, duplicate := seen[index]; duplicate {
			continue
		}
		seen[index] = struct{}{}
		valid = append(valid, index)
	}

	remainder := stack.Clone()
	changed := make([]int, 0, len(valid))
	for _, index := range valid {
		existing := c.slots[index]
		if existing.Empty() || !existing.StackableWith(remainder) || existing.ItemCount >= maxStack {
			continue
		}
		move := min(maxStack-existing.ItemCount, remainder.ItemCount)
		existing.ItemCount += move
		remainder.ItemCount -= move
		c.slots[index] = existing
		changed = append(changed, index)
		if remainder.Empty() {
			return EmptySlot(), changed
		}
	}
	for _, index := range valid {
		if !c.slots[index].Empty() {
			continue
		}
		move := min(maxStack, remainder.ItemCount)
		inserted := remainder.Clone()
		inserted.ItemCount = move
		c.slots[index] = inserted
		remainder.ItemCount -= move
		changed = append(changed, index)
		if remainder.Empty() {
			return EmptySlot(), changed
		}
	}
	return remainder, changed
}

// RemoveComponent removes an override and explicitly suppresses the base value.
func (s *ItemStack) RemoveComponent(typeID int32) {
	entries := s.ComponentPatch.Added[:0]
	for _, entry := range s.ComponentPatch.Added {
		if entry.TypeID != typeID {
			entries = append(entries, entry)
		}
	}
	s.ComponentPatch.Added = entries
	s.clearRemoved(typeID)
	s.ComponentPatch.Removed = append(s.ComponentPatch.Removed, typeID)
	if typeID == datacomponent.ComponentDamage {
		s.Damage = 0
	}
	if typeID == datacomponent.ComponentEnchantments {
		s.Enchantments = nil
	}
	s.RawComponents = nil
}
