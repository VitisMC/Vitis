package inventory

import (
	"fmt"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
)

// CraftResult describes one pure recipe evaluation and its per-slot transaction.
type CraftResult struct {
	RecipeID   string
	Output     ItemStack
	Consume    []int32
	Remainders []ItemStack
}

// CraftEvaluator evaluates a complete grid without performing side effects.
type CraftEvaluator func(grid []ItemStack, width int) (CraftResult, bool)

// SetCraftEvaluator installs stack-aware recipe evaluation alongside legacy matching.
func (wm *WindowManager) SetCraftEvaluator(evaluator CraftEvaluator) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.craftEvaluator = evaluator
}

// SetCraftCommit installs commit-only transformations that preserve result count
// and capacity requirements. Map postprocessing may assign a fresh map ID.
func (wm *WindowManager) SetCraftCommit(commit func(ItemStack) (ItemStack, error)) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.craftCommit = commit
}

// TakeCraftDrops drains transaction remainders that did not fit into the inventory.
func (wm *WindowManager) TakeCraftDrops() []ItemStack {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	drops := wm.craftDrops
	wm.craftDrops = nil
	return drops
}

// TakeCraftedRecipes drains IDs of recipes whose outputs were actually obtained.
func (wm *WindowManager) TakeCraftedRecipes() []string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	ids := wm.craftedRecipes
	wm.craftedRecipes = nil
	return ids
}

// evaluateCraftLocked evaluates the current authoritative grid.
func (wm *WindowManager) evaluateCraftLocked(target *Container, start, width int) (CraftResult, bool) {
	grid := make([]ItemStack, width*width)
	for i := range grid {
		grid[i] = target.Get(start + i)
	}
	if wm.craftEvaluator != nil {
		return wm.craftEvaluator(grid, width)
	}
	if wm.craftMatcher == nil {
		return CraftResult{}, false
	}
	ids := make([]int32, len(grid))
	for i, s := range grid {
		if !s.Empty() {
			ids[i] = s.ItemID
		}
	}
	id, count := wm.craftMatcher(ids, width)
	if id == 0 || count <= 0 {
		return CraftResult{}, false
	}
	r := CraftResult{Output: NewSlot(id, count), Consume: make([]int32, len(grid)), Remainders: make([]ItemStack, len(grid))}
	for i, s := range grid {
		if !s.Empty() {
			r.Consume[i] = 1
		}
	}
	return r, true
}

// validCraftResult verifies an evaluator transaction against its input grid.
func validCraftResult(r CraftResult, target *Container, start, width int) error {
	if r.Output.Empty() || r.Output.ItemCount > r.Output.MaxStackSize() || len(r.Consume) != width*width || len(r.Remainders) != width*width {
		return fmt.Errorf("invalid recipe transaction")
	}
	for i, n := range r.Consume {
		if n < 0 || n > target.Get(start+i).ItemCount {
			return fmt.Errorf("invalid recipe consumption")
		}
	}
	return nil
}

// CloseTemporaryCrafting returns menu inputs, the cursor, and the player grid.
func (wm *WindowManager) CloseTemporaryCrafting() []ItemStack {
	drops := wm.CloseWindowWithDrops()
	wm.mu.Lock()
	defer wm.mu.Unlock()
	indices := make([]int, 0, 36)
	for i := MainInventoryStart; i <= HotbarEnd; i++ {
		indices = append(indices, i)
	}
	for i := 1; i <= 4; i++ {
		stack := wm.inventory.Get(i)
		if stack.Empty() {
			continue
		}
		remainder, _ := wm.inventory.Insert(stack, stack.MaxStackSize(), indices)
		if !remainder.Empty() {
			drops = append(drops, remainder)
		}
		wm.inventory.Clear(i)
	}
	wm.inventory.Clear(CraftOutputSlot)
	wm.stateID++
	return drops
}

// hasMapPostprocessing identifies results that receive a new component on commit.
func hasMapPostprocessing(stack ItemStack) bool {
	_, ok := stack.Component(dc.ComponentMapPostProcessing)
	return ok
}
