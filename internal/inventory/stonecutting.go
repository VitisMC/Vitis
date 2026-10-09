package inventory

// StonecuttingChoice identifies an ordered recipe and its complete result stack.
type StonecuttingChoice struct {
	RecipeID string
	Output   ItemStack
}

// StonecuttingMatcher reads current resource recipes in their client display order.
type StonecuttingMatcher func(inputID int32) []StonecuttingChoice

// SetStonecuttingMatcher binds the menu to immutable resource recipe snapshots.
func (wm *WindowManager) SetStonecuttingMatcher(matcher StonecuttingMatcher) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.stonecuttingMatcher = matcher
}

// SelectStonecuttingRecipe selects only an existing recipe in the active menu.
func (wm *WindowManager) SelectStonecuttingRecipe(windowID, button int32) bool {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	w := wm.openWindow
	if w == nil || w.ID != windowID || w.Type != WindowTypeStonecutter || wm.stonecuttingMatcher == nil {
		return false
	}
	wm.refreshOpenWindowLocked()
	input := w.Container.Get(0)
	if input.Empty() {
		return false
	}
	choices := wm.stonecuttingMatcher(input.ItemID)
	if button < 0 || int(button) >= len(choices) || button > 32767 || !validStonecuttingChoice(choices[button]) {
		return false
	}
	wm.stonecuttingRecipe = choices[button].RecipeID
	wm.updateStonecuttingOutputLocked()
	wm.stateID++
	return wm.stonecuttingSelection >= 0
}

// SelectedStonecuttingRecipe returns the current authoritative property value.
func (wm *WindowManager) SelectedStonecuttingRecipe() int16 {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.refreshOpenWindowLocked()
	return wm.stonecuttingSelection
}

// validStonecuttingChoice bounds a single-input recipe transaction.
func validStonecuttingChoice(choice StonecuttingChoice) bool {
	return choice.RecipeID != "" && !choice.Output.Empty() && choice.Output.ItemCount <= choice.Output.MaxStackSize()
}

// updateStonecuttingOutputLocked refreshes previews without consuming inputs.
func (wm *WindowManager) updateStonecuttingOutputLocked() {
	w := wm.openWindow
	if w == nil || w.Type != WindowTypeStonecutter {
		return
	}
	input := w.Container.Get(0)
	id := input.ItemID
	if input.Empty() {
		id = 0
	}
	if id != wm.stonecuttingInput {
		wm.stonecuttingInput, wm.stonecuttingRecipe = id, ""
	}
	wm.stonecuttingSelection = -1
	output := EmptySlot()
	if !input.Empty() && wm.stonecuttingRecipe != "" && wm.stonecuttingMatcher != nil {
		for i, choice := range wm.stonecuttingMatcher(id) {
			if choice.RecipeID == wm.stonecuttingRecipe && i <= 32767 && validStonecuttingChoice(choice) {
				wm.stonecuttingSelection, output = int16(i), choice.Output.Clone()
				break
			}
		}
	}
	if wm.stonecuttingSelection < 0 {
		wm.stonecuttingRecipe = ""
	}
	w.Container.Set(1, output)
	w.Backing.Set(1, output)
}

// takeStonecuttingOutputLocked atomically obtains results and consumes one input each.
func (wm *WindowManager) takeStonecuttingOutputLocked(target *Container, button, mode int32) (bool, ItemStack) {
	if (mode == ClickTypeNormalClick || mode == ClickTypeDrop || mode == ClickTypeShiftClick) && button != 0 && button != 1 {
		return false, EmptySlot()
	}
	if mode != ClickTypeNormalClick && mode != ClickTypeShiftClick && mode != ClickTypeDrop && mode != ClickTypeNumberKey {
		return false, EmptySlot()
	}
	indices := make([]int, 36)
	for i := range indices {
		indices[i] = wm.openWindow.OwnedSlots + i
	}
	taken := false
	dropped := EmptySlot()
	for attempts := 0; attempts < 64; attempts++ {
		wm.updateStonecuttingOutputLocked()
		result := target.Get(1)
		if result.Empty() || result.ItemCount > result.MaxStackSize() {
			break
		}
		switch mode {
		case ClickTypeNormalClick:
			if !wm.cursorItem.Empty() {
				if !wm.cursorItem.StackableWith(result) || wm.cursorItem.ItemCount+result.ItemCount > result.MaxStackSize() {
					return false, EmptySlot()
				}
				wm.cursorItem.ItemCount += result.ItemCount
			} else {
				if button == ButtonRightClick {
					result.ItemCount = (result.ItemCount + 1) / 2
				}
				wm.cursorItem = result.Clone()
			}
		case ClickTypeShiftClick:
			capacity := int32(0)
			for _, i := range indices {
				stack := target.Get(i)
				if stack.Empty() {
					capacity += result.MaxStackSize()
				} else if stack.StackableWith(result) {
					capacity += max(int32(0), result.MaxStackSize()-stack.ItemCount)
				}
			}
			if capacity < result.ItemCount {
				return taken, EmptySlot()
			}
			target.Insert(result, result.MaxStackSize(), indices)
		case ClickTypeDrop:
			if button == 0 {
				result.ItemCount = 1
			}
			dropped = result.Clone()
		case ClickTypeNumberKey:
			if button < 0 || button > 8 && button != 40 {
				return false, EmptySlot()
			}
			if button == 40 {
				if !wm.inventory.Get(OffhandSlot).Empty() {
					return false, EmptySlot()
				}
				wm.inventory.Set(OffhandSlot, result)
			} else {
				index := wm.openWindow.OwnedSlots + 27 + int(button)
				if !target.Get(index).Empty() {
					return false, EmptySlot()
				}
				target.Set(index, result)
			}
		}
		input := target.Get(0)
		input.ItemCount--
		if input.Empty() {
			input = EmptySlot()
		}
		target.Set(0, input)
		wm.craftedRecipes = append(wm.craftedRecipes, wm.stonecuttingRecipe)
		taken = true
		if mode != ClickTypeShiftClick {
			break
		}
	}
	wm.updateStonecuttingOutputLocked()
	return taken, dropped
}
