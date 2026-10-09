package inventory

import "slices"

// PlaceRecipe atomically returns the grid and places complete ingredient stacks.
func (wm *WindowManager) PlaceRecipe(windowID int32, requirements [][]int32, makeAll bool) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	target := wm.containerForWindow(windowID)
	if target == nil || windowID != 0 && (wm.openWindow == nil || wm.openWindow.Type != WindowTypeCrafting) {
		return false
	}
	width := 2
	if windowID != 0 {
		width = 3
		wm.refreshOpenWindowLocked()
	}
	if len(requirements) != width*width {
		return false
	}
	// Work only on snapshots until every return and allocation has succeeded.
	slots := make([]ItemStack, HotbarEnd-MainInventoryStart+1)
	for i := range slots {
		slots[i] = wm.inventory.Get(MainInventoryStart + i).Clone()
	}
	current := int32(0)
	for i, ids := range requirements {
		if len(ids) == 0 {
			continue
		}
		stack := target.Get(i + 1)
		if stack.Empty() || !slices.Contains(ids, stack.ItemID) {
			current = 0
			break
		}
		if current == 0 || stack.ItemCount < current {
			current = stack.ItemCount
		}
	}
	for i := 1; i <= width*width; i++ {
		if !insertPlacementSnapshot(slots, target.Get(i).Clone()) {
			return false
		}
	}
	maxCount := int32(1)
	if makeAll {
		maxCount = 99
	} else if current > 0 {
		maxCount = current + 1
	}
	var placed []ItemStack
	var remaining []ItemStack
	for count := maxCount; count >= 1; count-- {
		candidate := make([]ItemStack, len(slots))
		for i := range slots {
			candidate[i] = slots[i].Clone()
		}
		grid := make([]ItemStack, len(requirements))
		budget := 16384
		if assignPlacement(candidate, requirements, grid, 0, count, &budget) {
			placed, remaining = grid, candidate
			break
		}
	}
	if placed == nil {
		return false
	}
	for i, stack := range remaining {
		wm.inventory.Set(MainInventoryStart+i, stack)
		if windowID != 0 {
			target.Set(wm.openWindow.OwnedSlots+i, stack)
		}
	}
	for i, stack := range placed {
		target.Set(i+1, stack)
	}
	wm.syncOpenWindowLocked(windowID)
	result, ok := wm.evaluateCraftLocked(target, 1, width)
	if ok {
		target.Set(0, result.Output)
	} else {
		target.Clear(0)
	}
	wm.syncOpenWindowLocked(windowID)
	wm.stateID++
	return true
}

// insertPlacementSnapshot returns a stack into a simulated player inventory.
func insertPlacementSnapshot(slots []ItemStack, stack ItemStack) bool {
	if stack.Empty() {
		return true
	}
	for pass := 0; pass < 2; pass++ {
		for i, destination := range slots {
			if pass == 0 && (destination.Empty() || !destination.StackableWith(stack)) || pass == 1 && !destination.Empty() {
				continue
			}
			room := stack.MaxStackSize() - destination.ItemCount
			amount := min(room, stack.ItemCount)
			if amount <= 0 {
				continue
			}
			if destination.Empty() {
				destination = stack.Clone()
				destination.ItemCount = 0
			}
			destination.ItemCount += amount
			slots[i] = destination
			stack.ItemCount -= amount
			if stack.Empty() {
				return true
			}
		}
	}
	return false
}

// assignPlacement backtracks over overlapping ingredient sets and stack metadata.
func assignPlacement(slots []ItemStack, requirements [][]int32, grid []ItemStack, index int, count int32, budget *int) bool {
	*budget--
	if *budget < 0 {
		return false
	}
	if index == len(requirements) {
		return true
	}
	ids := requirements[index]
	if len(ids) == 0 {
		return assignPlacement(slots, requirements, grid, index+1, count, budget)
	}
	for source, stack := range slots {
		if stack.Empty() || !slices.Contains(ids, stack.ItemID) || count > stack.MaxStackSize() {
			continue
		}
		duplicate := false
		for previous := 0; previous < source; previous++ {
			if !slots[previous].Empty() && slots[previous].StackableWith(stack) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		total := int32(0)
		for _, candidate := range slots {
			if !candidate.Empty() && candidate.StackableWith(stack) {
				total += candidate.ItemCount
			}
		}
		if total < count {
			continue
		}
		before := make([]ItemStack, len(slots))
		copy(before, slots)
		needed := count
		for i, candidate := range slots {
			if candidate.Empty() || !candidate.StackableWith(stack) {
				continue
			}
			take := min(needed, candidate.ItemCount)
			candidate.ItemCount -= take
			needed -= take
			if candidate.Empty() {
				candidate = EmptySlot()
			}
			slots[i] = candidate
			if needed == 0 {
				break
			}
		}
		placed := stack.Clone()
		placed.ItemCount = count
		grid[index] = placed
		if assignPlacement(slots, requirements, grid, index+1, count, budget) {
			return true
		}
		copy(slots, before)
		grid[index] = EmptySlot()
	}
	return false
}
