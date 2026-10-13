package entity

import (
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/data/generated/fuels"
	"github.com/vitismc/vitis/internal/data/generated/smelting"
	"github.com/vitismc/vitis/internal/data/generated/tag"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
)

type FurnaceType int

var furnaceItemTags = tag.Data()["minecraft:item"]

const (
	FurnaceTypeNormal FurnaceType = iota
	FurnaceTypeBlast
	FurnaceTypeSmoker
)

const (
	FurnaceSlotInput  = 0
	FurnaceSlotFuel   = 1
	FurnaceSlotOutput = 2
)

const (
	PropertyLitTimeRemaining = 0
	PropertyLitTotalTime     = 1
	PropertyCookingProgress  = 2
	PropertyCookingTotalTime = 3
)

type FurnaceLikeBlockEntity struct {
	*BaseBlockEntity

	mu sync.RWMutex

	furnaceType FurnaceType
	container   *inventory.Container

	litTimeRemaining int16
	litTotalTime     int16
	cookingProgress  int16
	cookingTotalTime int16

	recipesUsed    map[string]int
	dirty          bool
	raw            *nbt.Compound
	rawItems       map[int]*nbt.Compound
	inventoryValid bool

	viewers []FurnaceViewer
}

type FurnaceViewer interface {
	SendContainerProperty(windowID int8, property, value int16)
	SendContainerSlot(windowID int8, stateID int32, slotIdx int16, slot inventory.Slot)
}

func NewFurnaceLikeBlockEntity(typeName string, furnaceType FurnaceType, x, y, z int32) *FurnaceLikeBlockEntity {
	return &FurnaceLikeBlockEntity{
		BaseBlockEntity:  NewBaseBlockEntity(typeName, x, y, z),
		furnaceType:      furnaceType,
		inventoryValid:   true,
		container:        inventory.NewContainer(3),
		cookingTotalTime: 200,
		recipesUsed:      make(map[string]int),
	}
}

func NewFurnaceBlockEntity(x, y, z int32) *FurnaceLikeBlockEntity {
	return NewFurnaceLikeBlockEntity("minecraft:furnace", FurnaceTypeNormal, x, y, z)
}

func NewBlastFurnaceBlockEntity(x, y, z int32) *FurnaceLikeBlockEntity {
	return NewFurnaceLikeBlockEntity("minecraft:blast_furnace", FurnaceTypeBlast, x, y, z)
}

func NewSmokerBlockEntity(x, y, z int32) *FurnaceLikeBlockEntity {
	return NewFurnaceLikeBlockEntity("minecraft:smoker", FurnaceTypeSmoker, x, y, z)
}

func (f *FurnaceLikeBlockEntity) Container() *inventory.Container {
	return f.container
}

// InventoryValid reports whether persisted slots can be safely mutated.
func (f *FurnaceLikeBlockEntity) InventoryValid() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.inventoryValid
}

func (f *FurnaceLikeBlockEntity) FurnaceType() FurnaceType {
	return f.furnaceType
}

func (f *FurnaceLikeBlockEntity) IsBurning() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.litTimeRemaining > 0
}

func (f *FurnaceLikeBlockEntity) GetProperty(index int) int16 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	switch index {
	case PropertyLitTimeRemaining:
		return f.litTimeRemaining
	case PropertyLitTotalTime:
		return f.litTotalTime
	case PropertyCookingProgress:
		return f.cookingProgress
	case PropertyCookingTotalTime:
		return f.cookingTotalTime
	default:
		return 0
	}
}

func (f *FurnaceLikeBlockEntity) AddViewer(v FurnaceViewer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.viewers = append(f.viewers, v)
}

func (f *FurnaceLikeBlockEntity) RemoveViewer(v FurnaceViewer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, viewer := range f.viewers {
		if viewer == v {
			f.viewers = append(f.viewers[:i], f.viewers[i+1:]...)
			return
		}
	}
}

func (f *FurnaceLikeBlockEntity) Tick() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.inventoryValid {
		return false
	}
	wasBurning := f.litTimeRemaining > 0
	stateChanged := false

	if f.litTimeRemaining > 0 {
		f.litTimeRemaining--
	}

	inputSlot := f.container.Get(FurnaceSlotInput)
	fuelSlot := f.container.Get(FurnaceSlotFuel)
	outputSlot := f.container.Get(FurnaceSlotOutput)

	recipe := f.getRecipe(inputSlot)

	if f.litTimeRemaining == 0 && recipe != nil && f.canAcceptOutput(recipe, outputSlot) {
		burnTime := f.getFuelBurnTime(fuelSlot)
		if burnTime > 0 {
			f.litTimeRemaining = int16(burnTime)
			f.litTotalTime = int16(burnTime)

			newFuelSlot := fuelSlot
			newFuelSlot.ItemCount--
			if newFuelSlot.ItemCount <= 0 {
				newFuelSlot = inventory.Slot{}
			}
			f.container.Set(FurnaceSlotFuel, newFuelSlot)
			f.notifySlotChange(FurnaceSlotFuel, newFuelSlot)
			stateChanged = true
		}
	}

	if f.litTimeRemaining > 0 && recipe != nil && f.canAcceptOutput(recipe, outputSlot) {
		f.cookingProgress++
		f.cookingTotalTime = int16(recipe.CookingTime)

		if f.cookingProgress >= f.cookingTotalTime {
			f.craftRecipe(recipe, inputSlot, outputSlot)
			f.cookingProgress = 0
			stateChanged = true
		}
	} else if recipe == nil || !f.canAcceptOutput(recipe, outputSlot) {
		if f.cookingProgress > 0 {
			f.cookingProgress--
		}
	}

	if wasBurning != (f.litTimeRemaining > 0) {
		stateChanged = true
	}

	f.syncPropertiesToViewers()

	return stateChanged || f.litTimeRemaining > 0 || f.cookingProgress > 0 || wasBurning
}

// LoadPersistentNBT restores the furnace inventory and progress from chunk NBT.
func (f *FurnaceLikeBlockEntity) LoadPersistentNBT(root *nbt.Compound) {
	if root == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.raw = root.Clone()
	f.rawItems, f.inventoryValid = readContainerItems(f.container, root)
	f.litTimeRemaining = shortField(root, "lit_time_remaining")
	f.litTotalTime = shortField(root, "lit_total_time")
	f.cookingProgress = shortField(root, "cooking_time_spent")
	f.cookingTotalTime = shortField(root, "cooking_total_time")
	if f.cookingTotalTime <= 0 {
		f.cookingTotalTime = 200
	}
	if f.litTotalTime <= 0 {
		f.litTotalTime = f.litTimeRemaining
	}
	if used, ok := root.GetCompound("RecipesUsed"); ok {
		f.recipesUsed = make(map[string]int)
		for name, count := range used.GetAllInts() {
			if count > 0 {
				f.recipesUsed[name] = int(count)
			}
		}
	}
}

func shortField(root *nbt.Compound, name string) int16 {
	value, tag, ok := root.Get(name)
	if ok && tag == nbt.TagShort {
		if number, ok := value.(int16); ok {
			return number
		}
	}
	return 0
}

// PersistentNBT merges furnace-owned fields into the original chunk NBT.
func (f *FurnaceLikeBlockEntity) PersistentNBT() *nbt.Compound {
	f.mu.RLock()
	defer f.mu.RUnlock()
	root := f.raw.Clone()
	if root == nil {
		root = nbt.NewCompound()
	}
	if !f.inventoryValid {
		return root
	}
	x, y, z := f.Position()
	root.PutString("id", f.TypeName()).PutInt("x", x).PutInt("y", y).PutInt("z", z)
	root.PutList("Items", writeContainerItems(f.container, f.rawItems))
	root.PutShort("lit_time_remaining", f.litTimeRemaining)
	root.PutShort("lit_total_time", f.litTotalTime)
	root.PutShort("cooking_time_spent", f.cookingProgress)
	root.PutShort("cooking_total_time", f.cookingTotalTime)
	used := nbt.NewCompound()
	for name, count := range f.recipesUsed {
		used.PutInt(name, int32(count))
	}
	root.PutCompound("RecipesUsed", used)
	return root
}

// ChunkDataNBT encodes the current furnace state for chunk streaming.
func (f *FurnaceLikeBlockEntity) ChunkDataNBT() []byte {
	encoder := nbt.NewEncoder(256)
	if err := encoder.WriteRootCompound(f.PersistentNBT()); err != nil {
		return nil
	}
	return encoder.Bytes()
}

func (f *FurnaceLikeBlockEntity) getRecipe(input inventory.Slot) *smelting.CookingRecipe {
	if input.ItemCount <= 0 || input.ItemID == 0 {
		return nil
	}

	itemName := f.getItemName(input.ItemID)
	if itemName == "" {
		return nil
	}

	var recipe *smelting.CookingRecipe
	var recipeType smelting.RecipeType
	switch f.furnaceType {
	case FurnaceTypeBlast:
		recipe = smelting.GetBlastingRecipe(itemName)
		recipeType = smelting.Blasting
	case FurnaceTypeSmoker:
		recipe = smelting.GetSmokingRecipe(itemName)
		recipeType = smelting.Smoking
	default:
		recipe = smelting.GetSmeltingRecipe(itemName)
		recipeType = smelting.Smelting
	}
	if recipe != nil {
		return recipe
	}
	for i := range smelting.CookingRecipes {
		candidate := &smelting.CookingRecipes[i]
		if candidate.Type != recipeType || !strings.HasPrefix(candidate.Ingredient, "#") {
			continue
		}
		for _, id := range furnaceItemTags[strings.TrimPrefix(candidate.Ingredient, "#")] {
			if id == input.ItemID {
				return candidate
			}
		}
	}
	return nil
}

func (f *FurnaceLikeBlockEntity) getFuelBurnTime(fuel inventory.Slot) int {
	if fuel.ItemCount <= 0 || fuel.ItemID == 0 {
		return 0
	}

	itemName := f.getItemName(fuel.ItemID)
	if itemName == "" {
		return 0
	}

	burnTime := fuels.GetBurnTime(itemName)

	return burnTime
}

func (f *FurnaceLikeBlockEntity) canAcceptOutput(recipe *smelting.CookingRecipe, output inventory.Slot) bool {
	if recipe == nil {
		return false
	}

	resultCount := recipe.ResultCount
	if resultCount == 0 {
		resultCount = 1
	}

	result := inventory.NewSlot(f.getItemID(recipe.Result), int32(resultCount))
	if result.ItemID <= 0 || resultCount < 1 || int32(resultCount) > result.MaxStackSize() {
		return false
	}
	if output.Empty() {
		return true
	}
	return output.StackableWith(result) && output.ItemCount+int32(resultCount) <= output.MaxStackSize()
}

func (f *FurnaceLikeBlockEntity) craftRecipe(recipe *smelting.CookingRecipe, input, output inventory.Slot) {
	newInput := input
	newInput.ItemCount--
	if newInput.ItemCount <= 0 {
		newInput = inventory.Slot{}
	}
	f.container.Set(FurnaceSlotInput, newInput)
	f.notifySlotChange(FurnaceSlotInput, newInput)

	resultCount := recipe.ResultCount
	if resultCount == 0 {
		resultCount = 1
	}

	var newOutput inventory.Slot
	if output.ItemCount == 0 {
		newOutput = inventory.Slot{
			ItemID:    f.getItemID(recipe.Result),
			ItemCount: int32(resultCount),
		}
	} else {
		newOutput = output
		newOutput.ItemCount += int32(resultCount)
	}
	f.container.Set(FurnaceSlotOutput, newOutput)
	f.notifySlotChange(FurnaceSlotOutput, newOutput)

	f.recipesUsed[recipe.RecipeID]++
}

func (f *FurnaceLikeBlockEntity) syncPropertiesToViewers() {
	for _, v := range f.viewers {
		v.SendContainerProperty(0, PropertyLitTimeRemaining, f.litTimeRemaining)
		v.SendContainerProperty(0, PropertyLitTotalTime, f.litTotalTime)
		v.SendContainerProperty(0, PropertyCookingProgress, f.cookingProgress)
		v.SendContainerProperty(0, PropertyCookingTotalTime, f.cookingTotalTime)
	}
}

func (f *FurnaceLikeBlockEntity) notifySlotChange(slotIdx int, slot inventory.Slot) {
	for _, v := range f.viewers {
		v.SendContainerSlot(0, 0, int16(slotIdx), slot)
	}
}

func (f *FurnaceLikeBlockEntity) ExtractExperience() float64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	var totalXP float64
	for recipeID, count := range f.recipesUsed {
		for i := range smelting.CookingRecipes {
			r := &smelting.CookingRecipes[i]
			if r.RecipeID == recipeID {
				totalXP += r.Experience * float64(count)
				break
			}
		}
	}

	f.recipesUsed = make(map[string]int)
	return totalXP
}

func (f *FurnaceLikeBlockEntity) WriteNBT() map[string]any {
	f.mu.RLock()
	defer f.mu.RUnlock()

	nbt := f.BaseBlockEntity.WriteNBT()
	nbt["BurnTime"] = f.litTimeRemaining
	nbt["CookTime"] = f.cookingProgress
	nbt["CookTimeTotal"] = f.cookingTotalTime

	items := make([]map[string]any, 0, 3)
	for i := 0; i < 3; i++ {
		slot := f.container.Get(i)
		if slot.ItemCount > 0 {
			items = append(items, map[string]any{
				"Slot":  int8(i),
				"id":    f.getItemName(slot.ItemID),
				"Count": slot.ItemCount,
			})
		}
	}
	nbt["Items"] = items

	if len(f.recipesUsed) > 0 {
		recipes := make([]map[string]any, 0, len(f.recipesUsed))
		for id, count := range f.recipesUsed {
			recipes = append(recipes, map[string]any{
				"id":    id,
				"count": int32(count),
			})
		}
		nbt["RecipesUsed"] = recipes
	}

	return nbt
}

func (f *FurnaceLikeBlockEntity) ReadNBT(data map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.BaseBlockEntity.ReadNBT(data)

	if v, ok := data["BurnTime"].(int16); ok {
		f.litTimeRemaining = v
		f.litTotalTime = v
	}
	if v, ok := data["CookTime"].(int16); ok {
		f.cookingProgress = v
	}
	if v, ok := data["CookTimeTotal"].(int16); ok {
		f.cookingTotalTime = v
	}

	if items, ok := data["Items"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				slotIdx := int8(0)
				if v, ok := m["Slot"].(int8); ok {
					slotIdx = v
				}
				if slotIdx >= 0 && slotIdx < 3 {
					slot := inventory.Slot{}
					if id, ok := m["id"].(string); ok {
						slot.ItemID = f.getItemID(id)
					}
					if count, ok := m["Count"].(int8); ok {
						slot.ItemCount = int32(count)
					}
					f.container.Set(int(slotIdx), slot)
				}
			}
		}
	}

	if recipes, ok := data["RecipesUsed"].([]any); ok {
		f.recipesUsed = make(map[string]int)
		for _, recipe := range recipes {
			if m, ok := recipe.(map[string]any); ok {
				if id, ok := m["id"].(string); ok {
					count := 1
					if c, ok := m["count"].(int32); ok {
						count = int(c)
					}
					f.recipesUsed[id] = count
				}
			}
		}
	}
}

var itemRegistry interface {
	NameByID(id int32) string
	IDByName(name string) int32
}

func SetItemRegistry(registry interface {
	NameByID(id int32) string
	IDByName(name string) int32
}) {
	itemRegistry = registry
}

func (f *FurnaceLikeBlockEntity) getItemName(id int32) string {
	if itemRegistry != nil {
		return itemRegistry.NameByID(id)
	}
	return item.NameByID(id)
}

func (f *FurnaceLikeBlockEntity) getItemID(name string) int32 {
	if itemRegistry != nil {
		return itemRegistry.IDByName(name)
	}
	return item.IDByName(name)
}
