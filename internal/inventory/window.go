package inventory

import (
	"sync"
	"sync/atomic"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/data/generated/fuels"
	"github.com/vitismc/vitis/internal/item"
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
	WindowTypeGeneric9x1   int32 = 0
	WindowTypeGeneric9x2   int32 = 1
	WindowTypeGeneric9x3   int32 = 2
	WindowTypeGeneric9x4   int32 = 3
	WindowTypeGeneric9x5   int32 = 4
	WindowTypeGeneric9x6   int32 = 5
	WindowTypeGeneric3x3   int32 = 6
	WindowTypeBlastFurnace int32 = 10
	WindowTypeCrafting     int32 = 12
	WindowTypeFurnace      int32 = 14
	WindowTypeSmoker       int32 = 22
	WindowTypeStonecutter  int32 = 24
)

// Window represents an open container window.
type Window struct {
	ID          int32
	Type        int32
	Title       string
	Container   *Container
	Backing     *Container
	LeftHalf    *Container
	RightHalf   *Container
	OwnedSlots  int
	BlockX      int
	BlockY      int
	BlockZ      int
	OtherX      int
	OtherY      int
	OtherZ      int
	DoubleChest bool
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

	dragButton            int32
	dragSlots             []int16
	dragging              bool
	craftMatcher          CraftMatcher
	craftEvaluator        CraftEvaluator
	craftCommit           func(ItemStack) (ItemStack, error)
	craftDrops            []ItemStack
	craftedRecipes        []string
	stonecuttingMatcher   StonecuttingMatcher
	stonecuttingInput     int32
	stonecuttingRecipe    string
	stonecuttingSelection int16
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

// DropHeldItem removes one item or the whole held stack for a world drop.
func (wm *WindowManager) DropHeldItem(all bool) Slot {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	idx := HotbarStart + int(wm.heldSlot)
	held := wm.inventory.Get(idx)
	if held.Empty() {
		return EmptySlot()
	}
	dropped := held.Clone()
	if !all {
		dropped.ItemCount = 1
	}
	held.ItemCount -= dropped.ItemCount
	if held.Empty() {
		held = EmptySlot()
	}
	wm.inventory.Set(idx, held)
	wm.stateID++
	return dropped
}

// CursorItem returns the item currently attached to the cursor.
func (wm *WindowManager) CursorItem() Slot {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.cursorItem.Clone()
}

// SetCursorItem sets the item on the cursor.
func (wm *WindowManager) SetCursorItem(item Slot) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.cursorItem = item.Clone()
}

// OpenWindow opens a container window.
func (wm *WindowManager) OpenWindow(windowType int32, title string, container *Container) *Window {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if container == nil {
		return nil
	}
	owned := container.Size()
	view := NewContainer(owned + 36)
	for i, slot := range container.Slots() {
		view.Set(i, slot)
	}
	for i := 0; i < 36; i++ {
		view.Set(owned+i, wm.inventory.Get(MainInventoryStart+i))
	}
	w := &Window{
		ID:         AllocWindowID(),
		Type:       windowType,
		Title:      title,
		Container:  view,
		Backing:    container,
		OwnedSlots: owned,
	}
	wm.openWindow = w
	wm.dragging, wm.dragSlots = false, nil
	wm.stonecuttingInput, wm.stonecuttingRecipe, wm.stonecuttingSelection = 0, "", -1
	return w
}

// OpenDoubleChest opens a 54-slot view backed by two independent 27-slot chests.
func (wm *WindowManager) OpenDoubleChest(title string, left, right *Container) *Window {
	if left == nil || right == nil || left.Size() != 27 || right.Size() != 27 {
		return nil
	}
	combined := NewContainer(54)
	for i := 0; i < 27; i++ {
		combined.Set(i, left.Get(i))
		combined.Set(i+27, right.Get(i))
	}
	w := wm.OpenWindow(WindowTypeGeneric9x6, title, combined)
	w.LeftHalf, w.RightHalf, w.DoubleChest = left, right, true
	return w
}

// CloseWindow closes the currently open window.
func (wm *WindowManager) CloseWindow() {
	drops := wm.CloseWindowWithDrops()
	wm.mu.Lock()
	wm.craftDrops = append(wm.craftDrops, drops...)
	wm.mu.Unlock()
}

// CloseWindowWithDrops returns temporary menu inputs and cursor overflow once.
func (wm *WindowManager) CloseWindowWithDrops() []Slot {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	var drops []Slot
	if wm.openWindow != nil {
		wm.refreshOpenWindowLocked()
		start, end := 1, 0
		switch wm.openWindow.Type {
		case WindowTypeCrafting:
			start, end = 1, 9
		case WindowTypeStonecutter:
			start, end = 0, 0
			wm.openWindow.Backing.Clear(1)
		}
		if start <= end {
			indices := make([]int, 0, 36)
			for i := MainInventoryStart; i <= HotbarEnd; i++ {
				indices = append(indices, i)
			}
			for i := start; i <= end; i++ {
				stack := wm.openWindow.Backing.Get(i)
				if stack.Empty() {
					continue
				}
				remainder, _ := wm.inventory.Insert(stack, stack.MaxStackSize(), indices)
				if !remainder.Empty() {
					drops = append(drops, remainder)
				}
				wm.openWindow.Backing.Clear(i)
			}
		}
	}
	wm.openWindow = nil
	wm.dragging, wm.dragSlots = false, nil
	wm.stonecuttingInput, wm.stonecuttingRecipe, wm.stonecuttingSelection = 0, "", -1
	wm.returnCursorToInventoryLocked()
	if !wm.cursorItem.Empty() {
		drops = append(drops, wm.cursorItem.Clone())
		wm.cursorItem = EmptySlot()
	}
	return drops
}

// DropAllOnDeath closes the menu and removes all carried and equipped items once.
func (wm *WindowManager) DropAllOnDeath() []Slot {
	drops := wm.CloseWindowWithDrops()
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for index := 1; index < wm.inventory.Size(); index++ {
		stack := wm.inventory.Get(index)
		if !stack.Empty() {
			drops = append(drops, stack)
			wm.inventory.Clear(index)
		}
	}
	if !wm.cursorItem.Empty() {
		drops = append(drops, wm.cursorItem)
		wm.cursorItem = EmptySlot()
	}
	wm.stateID++
	return drops
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
	accepted, _ := wm.HandleClickWithDrop(windowID, stateID, slotIndex, button, mode, changedSlots, carriedItem)
	return accepted
}

// HandleClickWithDrop applies a transaction and returns an item removed from the inventory.
func (wm *WindowManager) HandleClickWithDrop(windowID, stateID int32, slotIndex int16, button, mode int32, changedSlots map[int16]Slot, carriedItem Slot) (bool, Slot) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	_ = changedSlots
	_ = carriedItem

	target := wm.containerForWindow(windowID)
	if target == nil || stateID != wm.stateID {
		return false, EmptySlot()
	}
	if windowID != 0 {
		wm.refreshOpenWindowLocked()
	}

	idx := int(slotIndex)
	if windowID != 0 && wm.openWindow.Type == WindowTypeStonecutter && idx == 1 {
		accepted, dropped := wm.takeStonecuttingOutputLocked(target, button, mode)
		if accepted {
			wm.syncOpenWindowLocked(windowID)
			wm.stateID++
		}
		return accepted, dropped
	}
	furnaceOutput := windowID != 0 && wm.openWindow != nil && (wm.openWindow.Type == WindowTypeFurnace || wm.openWindow.Type == WindowTypeBlastFurnace || wm.openWindow.Type == WindowTypeSmoker) && idx == 2
	craftOutput := idx == CraftOutputSlot && (windowID == 0 || (wm.openWindow != nil && wm.openWindow.Type == WindowTypeCrafting))
	if craftOutput {
		if mode != ClickTypeNormalClick && mode != ClickTypeShiftClick {
			return false, EmptySlot()
		}
		if mode == ClickTypeNormalClick && button != ButtonLeftClick && button != ButtonRightClick {
			return false, EmptySlot()
		}
		if !wm.takeCraftOutput(target, windowID, mode == ClickTypeShiftClick) {
			return false, EmptySlot()
		}
		wm.syncOpenWindowLocked(windowID)
		wm.stateID++
		return true, EmptySlot()
	}

	switch mode {
	case ClickTypeNormalClick:
		if idx == -999 {
			if button != ButtonLeftClick && button != ButtonRightClick {
				return false, EmptySlot()
			}
			dropped := wm.cursorItem.Clone()
			if button == ButtonRightClick && !dropped.Empty() {
				dropped.ItemCount = 1
			}
			wm.cursorItem.ItemCount -= dropped.ItemCount
			if wm.cursorItem.Empty() {
				wm.cursorItem = EmptySlot()
			}
			wm.stateID++
			return true, dropped
		}
		if idx < 0 || idx >= target.Size() || (button != ButtonLeftClick && button != ButtonRightClick) {
			return false, EmptySlot()
		}
		clicked := target.Get(idx)
		if furnaceOutput && !wm.cursorItem.Empty() {
			if clicked.Empty() || !clicked.StackableWith(wm.cursorItem) || clicked.ItemCount+wm.cursorItem.ItemCount > clicked.MaxStackSize() {
				return false, EmptySlot()
			}
			wm.cursorItem.ItemCount += clicked.ItemCount
			target.Clear(idx)
			wm.syncOpenWindowLocked(windowID)
			wm.stateID++
			return true, EmptySlot()
		}
		if wm.cursorItem.Empty() && clicked.Empty() {
			return true, EmptySlot()
		}
		if wm.cursorItem.Empty() {
			if button == ButtonLeftClick {
				wm.cursorItem = clicked
				target.Set(idx, EmptySlot())
			} else {
				half := (clicked.ItemCount + 1) / 2
				wm.cursorItem = clicked
				wm.cursorItem.ItemCount = half
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
				placed := wm.cursorItem
				placed.ItemCount = 1
				target.Set(idx, placed)
				wm.cursorItem.ItemCount--
				if wm.cursorItem.ItemCount <= 0 {
					wm.cursorItem = EmptySlot()
				}
			}
		} else if wm.cursorItem.StackableWith(clicked) {
			maxStack := clicked.MaxStackSize()
			if maxStack <= 0 {
				return false, EmptySlot()
			}
			if button == ButtonLeftClick {
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
				if clicked.ItemCount < maxStack {
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
			return false, EmptySlot()
		}
		clicked := target.Get(idx)
		if clicked.Empty() {
			return true, EmptySlot()
		}
		moved := wm.tryShiftMove(target, idx, clicked)
		if moved <= 0 {
			return false, EmptySlot()
		}
		clicked.ItemCount -= moved
		if clicked.ItemCount <= 0 {
			target.Set(idx, EmptySlot())
		} else {
			target.Set(idx, clicked)
		}

	case ClickTypeNumberKey:
		if idx < 0 || idx >= target.Size() || ((button < 0 || button > 8) && button != 40) {
			return false, EmptySlot()
		}
		hotbarIdx := HotbarStart + int(button)
		if button == 40 {
			hotbarIdx = OffhandSlot
		}
		if windowID != 0 && button != 40 {
			hotbarIdx = wm.openWindow.OwnedSlots + 27 + int(button)
		}
		clicked := target.Get(idx)
		hotbarItem := target.Get(hotbarIdx)
		if button == 40 && windowID != 0 {
			hotbarItem = wm.inventory.Get(OffhandSlot)
		}
		if furnaceOutput && !hotbarItem.Empty() {
			return false, EmptySlot()
		}
		target.Set(idx, hotbarItem)
		if button == 40 && windowID != 0 {
			wm.inventory.Set(OffhandSlot, clicked)
		} else {
			target.Set(hotbarIdx, clicked)
		}

	case ClickTypeDrop:
		if idx < 0 || idx >= target.Size() || (button != 0 && button != 1) {
			return false, EmptySlot()
		}
		dropped := target.Get(idx)
		if dropped.Empty() {
			return true, EmptySlot()
		}
		if button == 0 {
			dropped.ItemCount = 1
		}
		remaining := target.Get(idx)
		remaining.ItemCount -= dropped.ItemCount
		if remaining.Empty() {
			remaining = EmptySlot()
		}
		target.Set(idx, remaining)
		wm.syncOpenWindowLocked(windowID)
		wm.stateID++
		return true, dropped
	case ClickTypeDrag:
		if !wm.handleDrag(target, idx, button, windowID) {
			return false, EmptySlot()
		}
		if button == 0 || button == 1 || button == 4 || button == 5 {
			return true, EmptySlot()
		}
	case ClickTypeDoubleClick:
		if button != 0 || wm.cursorItem.Empty() {
			return false, EmptySlot()
		}
		remaining := wm.cursorItem.MaxStackSize() - wm.cursorItem.ItemCount
		for slot := 0; slot < target.Size() && remaining > 0; slot++ {
			if windowID != 0 && wm.openWindow.Type == WindowTypeStonecutter && slot == 1 {
				continue
			}
			if slot == CraftOutputSlot && (windowID == 0 || (wm.openWindow != nil && wm.openWindow.Type == WindowTypeCrafting)) {
				continue
			}
			stack := target.Get(slot)
			if !stack.Empty() && stack.StackableWith(wm.cursorItem) {
				move := min(remaining, stack.ItemCount)
				stack.ItemCount -= move
				wm.cursorItem.ItemCount += move
				remaining -= move
				if stack.Empty() {
					stack = EmptySlot()
				}
				target.Set(slot, stack)
			}
		}
	default:
		return false, EmptySlot()
	}

	wm.syncOpenWindowLocked(windowID)
	wm.stateID++
	return true, EmptySlot()
}

func (wm *WindowManager) syncOpenWindowLocked(windowID int32) {
	w := wm.openWindow
	if windowID == 0 || w == nil || w.ID != windowID || w.Backing == nil {
		return
	}
	for i := 0; i < w.OwnedSlots; i++ {
		w.Backing.Set(i, w.Container.Get(i))
	}
	if w.DoubleChest {
		for i := 0; i < 27; i++ {
			w.LeftHalf.Set(i, w.Backing.Get(i))
			w.RightHalf.Set(i, w.Backing.Get(i+27))
		}
	}
	for i := 0; i < 36; i++ {
		wm.inventory.Set(MainInventoryStart+i, w.Container.Get(w.OwnedSlots+i))
	}
	wm.updateStonecuttingOutputLocked()
}

func (wm *WindowManager) refreshOpenWindowLocked() {
	w := wm.openWindow
	if w == nil || w.Backing == nil {
		return
	}
	if w.DoubleChest {
		for i := 0; i < 27; i++ {
			w.Backing.Set(i, w.LeftHalf.Get(i))
			w.Backing.Set(i+27, w.RightHalf.Get(i))
		}
	}
	for i := 0; i < w.OwnedSlots; i++ {
		w.Container.Set(i, w.Backing.Get(i))
	}
	for i := 0; i < 36; i++ {
		w.Container.Set(w.OwnedSlots+i, wm.inventory.Get(MainInventoryStart+i))
	}
	wm.updateStonecuttingOutputLocked()
}

func (wm *WindowManager) handleDrag(target *Container, slotIdx int, button, windowID int32) bool {
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
		if !wm.dragging || (button == 1 && wm.dragButton != 0) || (button == 5 && wm.dragButton != 1) {
			return false
		}
		outputSlot := slotIdx == 0 && (windowID == 0 || wm.openWindow != nil && wm.openWindow.Type == WindowTypeCrafting)
		furnaceOutput := windowID != 0 && slotIdx == 2 && wm.openWindow != nil && (wm.openWindow.Type == WindowTypeFurnace || wm.openWindow.Type == WindowTypeBlastFurnace || wm.openWindow.Type == WindowTypeSmoker)
		stonecuttingOutput := windowID != 0 && slotIdx == 1 && wm.openWindow != nil && wm.openWindow.Type == WindowTypeStonecutter
		if slotIdx >= 0 && slotIdx < target.Size() && !outputSlot && !furnaceOutput && !stonecuttingOutput {
			for _, existing := range wm.dragSlots {
				if int(existing) == slotIdx {
					return true
				}
			}
			wm.dragSlots = append(wm.dragSlots, int16(slotIdx))
		}
	case 2, 6:
		if !wm.dragging || (button == 2 && wm.dragButton != 0) || (button == 6 && wm.dragButton != 1) {
			wm.dragging = false
			return false
		}
		wm.dragging = false
		if wm.cursorItem.Empty() || len(wm.dragSlots) == 0 {
			return true
		}
		original := wm.cursorItem.ItemCount
		perSlot := int32(1)
		if button == 2 {
			perSlot = original / int32(len(wm.dragSlots))
		}
		if perSlot == 0 {
			return true
		}
		for _, si := range wm.dragSlots {
			if wm.cursorItem.Empty() {
				break
			}
			idx := int(si)
			existing := target.Get(idx)
			if !existing.Empty() && !existing.StackableWith(wm.cursorItem) {
				continue
			}
			space := wm.cursorItem.MaxStackSize() - existing.ItemCount
			give := min(perSlot, space, wm.cursorItem.ItemCount)
			if give > 0 {
				if existing.Empty() {
					existing = wm.cursorItem.Clone()
					existing.ItemCount = 0
				}
				existing.ItemCount += give
				target.Set(idx, existing)
				wm.cursorItem.ItemCount -= give
			}
		}
		if wm.cursorItem.Empty() {
			wm.cursorItem = EmptySlot()
		}
	default:
		wm.dragging = false
		return false
	}
	return true
}

// takeCraftOutput commits only freshly evaluated recipes and returns leftovers safely.
func (wm *WindowManager) takeCraftOutput(target *Container, windowID int32, shift bool) bool {
	start, width := CraftInputStart, 2
	if windowID != 0 {
		start, width = 1, 3
	}
	crafted := false
	first := MainInventoryStart
	if windowID != 0 {
		first = wm.openWindow.OwnedSlots
	}
	indices := make([]int, 36)
	for i := range indices {
		indices[i] = first + i
	}
	for attempts := 0; attempts < 64; attempts++ {
		evaluation, ok := wm.evaluateCraftLocked(target, start, width)
		if !ok || validCraftResult(evaluation, target, start, width) != nil {
			break
		}
		result := evaluation.Output.Clone()
		if shift {
			capacity := int32(0)
			for _, i := range indices {
				s := target.Get(i)
				if s.Empty() {
					capacity += result.MaxStackSize()
				} else if _, postprocess := result.Component(dc.ComponentMapPostProcessing); !postprocess && s.StackableWith(result) {
					capacity += max(int32(0), result.MaxStackSize()-s.ItemCount)
				}
			}
			if capacity < result.ItemCount {
				break
			}
		} else if !wm.cursorItem.Empty() && (hasMapPostprocessing(result) || !wm.cursorItem.StackableWith(result) || wm.cursorItem.ItemCount+result.ItemCount > result.MaxStackSize()) {
			break
		}
		if wm.craftCommit != nil {
			var err error
			result, err = wm.craftCommit(result)
			if err != nil || result.Empty() {
				break
			}
		}
		if shift {
			remainder, _ := target.Insert(result, result.MaxStackSize(), indices)
			if !remainder.Empty() {
				break
			}
		} else if wm.cursorItem.Empty() {
			wm.cursorItem = result.Clone()
		} else {
			wm.cursorItem.ItemCount += result.ItemCount
		}
		for i, n := range evaluation.Consume {
			stack := target.Get(start + i)
			stack.ItemCount -= n
			if stack.Empty() {
				stack = EmptySlot()
			}
			target.Set(start+i, stack)
			remainder := evaluation.Remainders[i].Clone()
			if remainder.Empty() {
				continue
			}
			current := target.Get(start + i)
			if current.Empty() {
				count := min(remainder.ItemCount, remainder.MaxStackSize())
				placed := remainder.Clone()
				placed.ItemCount = count
				target.Set(start+i, placed)
				remainder.ItemCount -= count
			} else if current.StackableWith(remainder) {
				count := min(remainder.ItemCount, max(int32(0), current.MaxStackSize()-current.ItemCount))
				current.ItemCount += count
				target.Set(start+i, current)
				remainder.ItemCount -= count
			}
			if !remainder.Empty() {
				remainder, _ = target.Insert(remainder, remainder.MaxStackSize(), indices)
				if !remainder.Empty() {
					wm.craftDrops = append(wm.craftDrops, remainder)
				}
			}
		}
		crafted = true
		if evaluation.RecipeID != "" {
			wm.craftedRecipes = append(wm.craftedRecipes, evaluation.RecipeID)
		}
		if next, ok := wm.evaluateCraftLocked(target, start, width); ok {
			target.Set(0, next.Output)
		} else {
			target.Clear(0)
		}
		if !shift {
			break
		}
	}
	return crafted
}

func (wm *WindowManager) tryShiftMove(target *Container, fromIdx int, stack Slot) int32 {
	remaining := stack.ItemCount
	moveInto := func(start, end int) {
		for index := start; index <= end && remaining > 0; index++ {
			if index == fromIdx {
				continue
			}
			existing := target.Get(index)
			if existing.Empty() || !existing.StackableWith(stack) {
				continue
			}
			maxStack := existing.MaxStackSize()
			space := maxStack - existing.ItemCount
			if space <= 0 {
				continue
			}
			moved := remaining
			if moved > space {
				moved = space
			}
			existing.ItemCount += moved
			remaining -= moved
			target.Set(index, existing)
		}
		for index := start; index <= end && remaining > 0; index++ {
			if index == fromIdx || !target.Get(index).Empty() {
				continue
			}
			placed := stack
			placed.ItemCount = min(remaining, stack.MaxStackSize())
			target.Set(index, placed)
			remaining -= placed.ItemCount
		}
	}
	if wm.openWindow != nil && target == wm.openWindow.Container {
		owned := wm.openWindow.OwnedSlots
		if fromIdx < owned {
			moveInto(owned, owned+35)
		} else if wm.openWindow.Type == WindowTypeStonecutter {
			if wm.stonecuttingMatcher != nil && len(wm.stonecuttingMatcher(stack.ItemID)) != 0 {
				moveInto(0, 0)
			} else if fromIdx >= owned+27 {
				moveInto(owned, owned+26)
			} else {
				moveInto(owned+27, owned+35)
			}
		} else if wm.openWindow.Type == WindowTypeCrafting {
			if fromIdx >= owned+27 {
				moveInto(owned, owned+26)
			} else {
				moveInto(owned+27, owned+35)
			}
		} else {
			if wm.openWindow.Type == WindowTypeFurnace || wm.openWindow.Type == WindowTypeBlastFurnace || wm.openWindow.Type == WindowTypeSmoker {
				if fuels.GetBurnTime(item.NameByID(stack.ItemID)) > 0 {
					moveInto(1, 1)
				} else {
					moveInto(0, 0)
				}
			} else {
				moveInto(0, owned-1)
			}
		}
	} else if fromIdx >= HotbarStart && fromIdx <= HotbarEnd {
		moveInto(MainInventoryStart, MainInventoryEnd)
	} else {
		moveInto(HotbarStart, HotbarEnd)
	}
	return stack.ItemCount - remaining
}

func (wm *WindowManager) returnCursorToInventoryLocked() {
	if wm.cursorItem.Empty() {
		return
	}
	remaining := wm.cursorItem.ItemCount
	maxStack := wm.cursorItem.MaxStackSize()
	if maxStack <= 0 {
		return
	}
	for index := MainInventoryStart; index <= HotbarEnd && remaining > 0; index++ {
		existing := wm.inventory.Get(index)
		if existing.Empty() || !existing.StackableWith(wm.cursorItem) {
			continue
		}
		space := maxStack - existing.ItemCount
		if space <= 0 {
			continue
		}
		moved := remaining
		if moved > space {
			moved = space
		}
		existing.ItemCount += moved
		remaining -= moved
		wm.inventory.Set(index, existing)
	}
	for index := MainInventoryStart; index <= HotbarEnd && remaining > 0; index++ {
		if !wm.inventory.Get(index).Empty() {
			continue
		}
		placed := wm.cursorItem
		placed.ItemCount = remaining
		wm.inventory.Set(index, placed)
		remaining = 0
	}
	wm.cursorItem.ItemCount = remaining
	if remaining == 0 {
		wm.cursorItem = EmptySlot()
	}
}

// SnapshotForResync increments the state ID and returns all slots and cursor
// for the given window so the client can be fully re-synchronized.
func (wm *WindowManager) SnapshotForResync(windowID int32) (stateID int32, slots []Slot, cursor Slot) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if windowID != 0 {
		wm.refreshOpenWindowLocked()
	}
	target := wm.containerForWindow(windowID)
	if target == nil {
		return wm.stateID, nil, wm.cursorItem.Clone()
	}
	wm.stateID++
	return wm.stateID, target.Slots(), wm.cursorItem.Clone()
}

// Snapshot returns the current authoritative window state without changing its revision.
func (wm *WindowManager) Snapshot(windowID int32) (stateID int32, slots []Slot, cursor Slot) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if windowID != 0 {
		wm.refreshOpenWindowLocked()
	}
	target := wm.containerForWindow(windowID)
	if target == nil {
		return wm.stateID, nil, wm.cursorItem.Clone()
	}
	return wm.stateID, target.Slots(), wm.cursorItem.Clone()
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
	wm.mu.Lock()
	defer wm.mu.Unlock()
	result, ok := wm.evaluateCraftLocked(wm.inventory, CraftInputStart, 2)
	if !ok {
		wm.inventory.Clear(0)
		return EmptySlot()
	}
	wm.inventory.Set(0, result.Output)
	return result.Output.Clone()
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
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if wm.openWindow == nil || wm.openWindow.Type != WindowTypeCrafting {
		return EmptySlot()
	}
	wm.refreshOpenWindowLocked()
	w := wm.openWindow
	result, ok := wm.evaluateCraftLocked(w.Container, 1, 3)
	if !ok {
		w.Container.Clear(0)
		w.Backing.Clear(0)
		return EmptySlot()
	}
	w.Container.Set(0, result.Output)
	w.Backing.Set(0, result.Output)
	return result.Output.Clone()
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
