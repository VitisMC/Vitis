package inventory

import (
	"sync"
	"sync/atomic"
)

const (
	ClickTypeNormalClick int32 = 0
	ClickTypeShiftClick  int32 = 1
	ClickTypeNumberKey   int32 = 2
	ClickTypeMiddleClick int32 = 3
	ClickTypeDrop        int32 = 4
	ClickTypeDrag        int32 = 5
	ClickTypeDoubleClick int32 = 6

	ButtonLeftClick  int32 = 0
	ButtonRightClick int32 = 1
)

var nextWindowID atomic.Int32

// AllocWindowID allocates a unique non-zero window ID.
func AllocWindowID() int32 {
	for {
		id := nextWindowID.Add(1)
		if id == 0 {
			continue
		}
		if id > 100 {
			nextWindowID.Store(1)
			return 1
		}
		return id
	}
}

// WindowType constants for Minecraft 1.21.4 container types.
const (
	WindowTypeGeneric9x1 int32 = 0
	WindowTypeGeneric9x2 int32 = 1
	WindowTypeGeneric9x3 int32 = 2
	WindowTypeGeneric9x4 int32 = 3
	WindowTypeGeneric9x5 int32 = 4
	WindowTypeGeneric9x6 int32 = 5
	WindowTypeGeneric3x3 int32 = 6
	WindowTypeCrafting   int32 = 12
)

// Window represents an open container window.
type Window struct {
	ID        int32
	Type      int32
	Title     string
	Container *Container
}

// CraftMatcher matches a crafting grid to a recipe result.
type CraftMatcher func(grid []int32, gridWidth int) (resultID int32, count int32)

// WindowManager tracks the player's open windows and cursor item.
type WindowManager struct {
	mu         sync.RWMutex
	inventory  *Container
	openWindow *Window
	cursorItem Slot
	stateID    int32
	heldSlot   int32

	dragButton   int32
	dragSlots    []int16
	dragging     bool
	craftMatcher CraftMatcher
}

// NewWindowManager creates a window manager with a player inventory.
// stateID starts at 1 to match the bootstrap's initial SetContainerContent.
func NewWindowManager() *WindowManager {
	return &WindowManager{
		inventory: NewPlayerInventory(),
		stateID:   1,
	}
}

// Inventory returns the player's inventory container.
func (wm *WindowManager) Inventory() *Container {
	return wm.inventory
}

// StateID returns and increments the state ID for synchronization.
func (wm *WindowManager) StateID() int32 {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.stateID++
	return wm.stateID
}

// CurrentStateID returns the current state ID without incrementing.
func (wm *WindowManager) CurrentStateID() int32 {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.stateID
}

// HeldSlot returns the currently selected hotbar slot (0-8).
func (wm *WindowManager) HeldSlot() int32 {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.heldSlot
}

// SetHeldSlot changes the selected hotbar slot.
func (wm *WindowManager) SetHeldSlot(slot int32) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if slot >= 0 && slot <= 8 {
		wm.heldSlot = slot
	}
}

// HeldItem returns the item in the currently selected hotbar slot.
func (wm *WindowManager) HeldItem() Slot {
	return wm.inventory.HotbarSlot(int(wm.HeldSlot()))
}

// GetSlot returns the slot at the given index from the player inventory.
func (wm *WindowManager) GetSlot(index int) Slot {
	return wm.inventory.Get(index)
}

// ConsumeHeldItem decrements the held item count by one.
// Returns the updated slot after consumption.
func (wm *WindowManager) ConsumeHeldItem() Slot {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	idx := HotbarStart + int(wm.heldSlot)
	s := wm.inventory.Get(idx)
	if s.Empty() {
		return s
	}
	s.ItemCount--
	if s.ItemCount <= 0 {
		s = EmptySlot()
	}
	wm.inventory.Set(idx, s)
	return s
}

// CursorItem returns the item currently attached to the cursor.
func (wm *WindowManager) CursorItem() Slot {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.cursorItem
}

// SetCursorItem sets the item on the cursor.
func (wm *WindowManager) SetCursorItem(item Slot) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.cursorItem = item
}

// OpenWindow opens a container window.
func (wm *WindowManager) OpenWindow(windowType int32, title string, container *Container) *Window {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	w := &Window{
		ID:        AllocWindowID(),
		Type:      windowType,
		Title:     title,
		Container: container,
	}
	wm.openWindow = w
	return w
}

// CloseWindow closes the currently open window.
func (wm *WindowManager) CloseWindow() {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.openWindow = nil
	wm.cursorItem = EmptySlot()
}

// ActiveWindow returns the currently open window, or nil.
func (wm *WindowManager) ActiveWindow() *Window {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.openWindow
}

// HandleCreativeSet handles creative mode slot setting.
func (wm *WindowManager) HandleCreativeSet(slotIndex int16, item Slot) {
	idx := int(slotIndex)
	if idx >= 0 && idx < PlayerInventorySize {
		wm.inventory.Set(idx, item)
	}
}

// HandleClick processes a container click with server-authoritative logic.
// Returns true if the click was accepted, false if the client should be re-synced.
func (wm *WindowManager) HandleClick(windowID, stateID int32, slotIndex int16, button, mode int32, changedSlots map[int16]Slot, carriedItem Slot) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	target := wm.containerForWindow(windowID)
	if target == nil {
		return false
	}

	idx := int(slotIndex)

	switch mode {
	case ClickTypeNormalClick:
		if idx == -999 {
			if button == ButtonLeftClick {
				wm.cursorItem = EmptySlot()
			} else if button == ButtonRightClick && !wm.cursorItem.Empty() {
				wm.cursorItem.ItemCount--
				if wm.cursorItem.ItemCount <= 0 {
					wm.cursorItem = EmptySlot()
				}
			}
			return true
		}
		if idx < 0 || idx >= target.Size() {
			return false
		}
		clicked := target.Get(idx)
		if wm.cursorItem.Empty() && clicked.Empty() {
			return true
		}
		if wm.cursorItem.Empty() {
			if button == ButtonLeftClick {
				wm.cursorItem = clicked
				target.Set(idx, EmptySlot())
			} else {
				half := (clicked.ItemCount + 1) / 2
				wm.cursorItem = Slot{ItemID: clicked.ItemID, ItemCount: half, RawComponents: clicked.RawComponents}
				clicked.ItemCount -= half
				if clicked.ItemCount <= 0 {
					target.Set(idx, EmptySlot())
				} else {
					target.Set(idx, clicked)
				}
			}
		} else if clicked.Empty() {
			if button == ButtonLeftClick {
				target.Set(idx, wm.cursorItem)
				wm.cursorItem = EmptySlot()
			} else {
				target.Set(idx, Slot{ItemID: wm.cursorItem.ItemID, ItemCount: 1, RawComponents: wm.cursorItem.RawComponents})
				wm.cursorItem.ItemCount--
				if wm.cursorItem.ItemCount <= 0 {
					wm.cursorItem = EmptySlot()
				}
			}
		} else if wm.cursorItem.ItemID == clicked.ItemID {
			if button == ButtonLeftClick {
				maxStack := int32(64)
				space := maxStack - clicked.ItemCount
				transfer := wm.cursorItem.ItemCount
				if transfer > space {
					transfer = space
				}
				clicked.ItemCount += transfer
				target.Set(idx, clicked)
				wm.cursorItem.ItemCount -= transfer
				if wm.cursorItem.ItemCount <= 0 {
					wm.cursorItem = EmptySlot()
				}
			} else {
				if clicked.ItemCount < 64 {
					clicked.ItemCount++
					target.Set(idx, clicked)
					wm.cursorItem.ItemCount--
					if wm.cursorItem.ItemCount <= 0 {
						wm.cursorItem = EmptySlot()
					}
				}
			}
		} else {
			target.Set(idx, wm.cursorItem)
			wm.cursorItem = clicked
		}

	case ClickTypeShiftClick:
		if idx < 0 || idx >= target.Size() {
			return false
		}
		clicked := target.Get(idx)
		if clicked.Empty() {
			return true
		}
		moved := wm.tryShiftMove(target, idx, clicked)
		if moved {
			target.Set(idx, EmptySlot())
		}

	case ClickTypeNumberKey:
		if idx < 0 || idx >= target.Size() {
			return false
		}
		hotbarIdx := HotbarStart + int(button)
		if button == 40 {
			hotbarIdx = OffhandSlot
		}
		clicked := target.Get(idx)
		hotbarItem := wm.inventory.Get(hotbarIdx)
		target.Set(idx, hotbarItem)
		wm.inventory.Set(hotbarIdx, clicked)

	case ClickTypeDrop:
		if idx < 0 || idx >= target.Size() {
			return false
		}
		clicked := target.Get(idx)
		if clicked.Empty() {
			return true
		}
		if button == ButtonLeftClick {
			clicked.ItemCount--
			if clicked.ItemCount <= 0 {
				target.Set(idx, EmptySlot())
			} else {
				target.Set(idx, clicked)
			}
		} else {
			target.Set(idx, EmptySlot())
		}

	case ClickTypeDrag:
		wm.handleDrag(target, idx, button)

	case ClickTypeDoubleClick:
		if wm.cursorItem.Empty() {
			return true
		}
		maxStack := int32(64)
		for i := 0; i < target.Size() && wm.cursorItem.ItemCount < maxStack; i++ {
			s := target.Get(i)
			if !s.Empty() && s.ItemID == wm.cursorItem.ItemID {
				take := maxStack - wm.cursorItem.ItemCount
				if take > s.ItemCount {
					take = s.ItemCount
				}
				wm.cursorItem.ItemCount += take
				s.ItemCount -= take
				if s.ItemCount <= 0 {
					target.Set(i, EmptySlot())
				} else {
					target.Set(i, s)
				}
			}
		}

	default:
		for slotIdx, item := range changedSlots {
			if int(slotIdx) >= 0 && int(slotIdx) < target.Size() {
				target.Set(int(slotIdx), item)
			}
		}
		wm.cursorItem = carriedItem
	}

	wm.stateID++
	return true
}

func (wm *WindowManager) handleDrag(target *Container, slotIdx int, button int32) {
	switch button {
	case 0:
		wm.dragging = true
		wm.dragButton = 0
		wm.dragSlots = wm.dragSlots[:0]
	case 4:
		wm.dragging = true
		wm.dragButton = 1
		wm.dragSlots = wm.dragSlots[:0]
	case 1, 5:
		if wm.dragging && slotIdx >= 0 && slotIdx < target.Size() {
			wm.dragSlots = append(wm.dragSlots, int16(slotIdx))
		}
	case 2:
		if !wm.dragging || wm.cursorItem.Empty() || len(wm.dragSlots) == 0 {
			wm.dragging = false
			return
		}
		perSlot := wm.cursorItem.ItemCount / int32(len(wm.dragSlots))
		if perSlot < 1 {
			perSlot = 1
		}
		remaining := wm.cursorItem.ItemCount
		for _, si := range wm.dragSlots {
			if remaining <= 0 {
				break
			}
			idx := int(si)
			if idx < 0 || idx >= target.Size() {
				continue
			}
			existing := target.Get(idx)
			give := perSlot
			if give > remaining {
				give = remaining
			}
			if existing.Empty() {
				target.Set(idx, Slot{ItemID: wm.cursorItem.ItemID, ItemCount: give, RawComponents: wm.cursorItem.RawComponents})
				remaining -= give
			} else if existing.ItemID == wm.cursorItem.ItemID {
				space := int32(64) - existing.ItemCount
				if give > space {
					give = space
				}
				if give > 0 {
					existing.ItemCount += give
					target.Set(idx, existing)
					remaining -= give
				}
			}
		}
		wm.cursorItem.ItemCount = remaining
		if wm.cursorItem.ItemCount <= 0 {
			wm.cursorItem = EmptySlot()
		}
		wm.dragging = false
	case 6:
		if !wm.dragging || wm.cursorItem.Empty() || len(wm.dragSlots) == 0 {
			wm.dragging = false
			return
		}
		remaining := wm.cursorItem.ItemCount
		for _, si := range wm.dragSlots {
			if remaining <= 0 {
				break
			}
			idx := int(si)
			if idx < 0 || idx >= target.Size() {
				continue
			}
			existing := target.Get(idx)
			if existing.Empty() {
				target.Set(idx, Slot{ItemID: wm.cursorItem.ItemID, ItemCount: 1, RawComponents: wm.cursorItem.RawComponents})
				remaining--
			} else if existing.ItemID == wm.cursorItem.ItemID && existing.ItemCount < 64 {
				existing.ItemCount++
				target.Set(idx, existing)
				remaining--
			}
		}
		wm.cursorItem.ItemCount = remaining
		if wm.cursorItem.ItemCount <= 0 {
			wm.cursorItem = EmptySlot()
		}
		wm.dragging = false
	default:
		wm.dragging = false
	}
}

func (wm *WindowManager) tryShiftMove(target *Container, fromIdx int, item Slot) bool {
	if fromIdx >= HotbarStart && fromIdx <= HotbarEnd-1 {
		for i := MainInventoryStart; i <= MainInventoryEnd; i++ {
			if target.Get(i).Empty() {
				target.Set(i, item)
				return true
			}
		}
		for i := MainInventoryStart; i <= MainInventoryEnd; i++ {
			s := target.Get(i)
			if s.ItemID == item.ItemID && s.ItemCount < 64 {
				space := int32(64) - s.ItemCount
				if item.ItemCount <= space {
					s.ItemCount += item.ItemCount
					target.Set(i, s)
					return true
				}
			}
		}
	} else {
		for i := HotbarStart; i < HotbarEnd; i++ {
			if target.Get(i).Empty() {
				target.Set(i, item)
				return true
			}
		}
		for i := HotbarStart; i < HotbarEnd; i++ {
			s := target.Get(i)
			if s.ItemID == item.ItemID && s.ItemCount < 64 {
				space := int32(64) - s.ItemCount
				if item.ItemCount <= space {
					s.ItemCount += item.ItemCount
					target.Set(i, s)
					return true
				}
			}
		}
	}
	return false
}

// SnapshotForResync increments the state ID and returns all slots and cursor
// for the given window so the client can be fully re-synchronized.
func (wm *WindowManager) SnapshotForResync(windowID int32) (stateID int32, slots []Slot, cursor Slot) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	target := wm.containerForWindow(windowID)
	if target == nil {
		return wm.stateID, nil, wm.cursorItem
	}
	wm.stateID++
	return wm.stateID, target.Slots(), wm.cursorItem
}

func (wm *WindowManager) containerForWindow(windowID int32) *Container {
	if windowID == 0 {
		return wm.inventory
	}
	if wm.openWindow != nil && wm.openWindow.ID == windowID {
		return wm.openWindow.Container
	}
	return nil
}

// SetCraftMatcher sets the function used to match crafting grids to recipes.
func (wm *WindowManager) SetCraftMatcher(m CraftMatcher) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.craftMatcher = m
}

// UpdateCraftOutput checks the player inventory 2x2 crafting grid and updates slot 0.
// Returns the result slot for the caller to send to the client.
func (wm *WindowManager) UpdateCraftOutput() Slot {
	if wm.craftMatcher == nil {
		return EmptySlot()
	}
	grid := make([]int32, 4)
	for i := 0; i < 4; i++ {
		s := wm.inventory.Get(CraftInputStart + i)
		if !s.Empty() {
			grid[i] = s.ItemID
		}
	}
	resultID, count := wm.craftMatcher(grid, 2)
	if resultID == 0 {
		wm.inventory.Set(CraftOutputSlot, EmptySlot())
		return EmptySlot()
	}
	out := NewSlot(resultID, count)
	wm.inventory.Set(CraftOutputSlot, out)
	return out
}

// ConsumeCraftIngredients decrements each non-empty slot in the 2x2 crafting grid by one.
func (wm *WindowManager) ConsumeCraftIngredients() {
	for i := CraftInputStart; i <= CraftInputEnd; i++ {
		s := wm.inventory.Get(i)
		if s.Empty() {
			continue
		}
		s.ItemCount--
		if s.ItemCount <= 0 {
			wm.inventory.Set(i, EmptySlot())
		} else {
			wm.inventory.Set(i, s)
		}
	}
}

// UpdateCraftTableOutput checks a 3x3 crafting table grid and updates slot 0 of the window.
// Returns the result slot.
func (wm *WindowManager) UpdateCraftTableOutput() Slot {
	if wm.craftMatcher == nil || wm.openWindow == nil || wm.openWindow.Container == nil {
		return EmptySlot()
	}
	c := wm.openWindow.Container
	grid := make([]int32, 9)
	for i := 0; i < 9; i++ {
		s := c.Get(1 + i)
		if !s.Empty() {
			grid[i] = s.ItemID
		}
	}
	resultID, count := wm.craftMatcher(grid, 3)
	if resultID == 0 {
		c.Set(0, EmptySlot())
		return EmptySlot()
	}
	out := NewSlot(resultID, count)
	c.Set(0, out)
	return out
}

// ConsumeCraftTableIngredients decrements each non-empty slot in the 3x3 grid by one.
func (wm *WindowManager) ConsumeCraftTableIngredients() {
	if wm.openWindow == nil || wm.openWindow.Container == nil {
		return
	}
	c := wm.openWindow.Container
	for i := 1; i <= 9; i++ {
		s := c.Get(i)
		if s.Empty() {
			continue
		}
		s.ItemCount--
		if s.ItemCount <= 0 {
			c.Set(i, EmptySlot())
		} else {
			c.Set(i, s)
		}
	}
}
