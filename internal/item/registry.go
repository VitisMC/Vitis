package item

import (
	genitem "github.com/vitismc/vitis/internal/data/generated/item"
)

// Info returns the ItemInfo for an item by its numeric ID.
func Info(itemID int32) *genitem.ItemInfo {
	if itemID < 0 || itemID >= int32(len(genitem.Items)) {
		return nil
	}
	return &genitem.Items[itemID]
}

// InfoByName returns the ItemInfo for an item by its namespaced name.
func InfoByName(name string) *genitem.ItemInfo {
	id, ok := genitem.ItemByName[name]
	if !ok {
		return nil
	}
	return &genitem.Items[id]
}

// IDByName returns the item ID for a namespaced name, or -1 if not found.
func IDByName(name string) int32 {
	id, ok := genitem.ItemByName[name]
	if !ok {
		return -1
	}
	return id
}

// NameByID returns the item name for a given ID.
func NameByID(itemID int32) string {
	info := Info(itemID)
	if info == nil {
		return ""
	}
	return info.Name
}

// StackSize returns the max stack size for the given item ID.
func StackSize(itemID int32) int32 {
	info := Info(itemID)
	if info == nil {
		return 0
	}
	return info.StackSize
}

// IsAir returns true if the item ID is air (0).
func IsAir(itemID int32) bool {
	return itemID == 0
}
