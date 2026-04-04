package block

import (
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

// Info returns the BlockInfo for a block by its numeric ID.
func Info(blockID int32) *genblock.BlockInfo {
	if blockID < 0 || blockID >= int32(len(genblock.Blocks)) {
		return nil
	}
	return &genblock.Blocks[blockID]
}

// InfoByName returns the BlockInfo for a block by its namespaced name.
func InfoByName(name string) *genblock.BlockInfo {
	id, ok := genblock.BlockByName[name]
	if !ok {
		return nil
	}
	return &genblock.Blocks[id]
}

// DefaultStateID returns the default state ID for a block name, or -1 if not found.
func DefaultStateID(name string) int32 {
	info := InfoByName(name)
	if info == nil {
		return -1
	}
	return info.DefaultState
}

// BlockIDFromState returns the block ID for a given state ID.
func BlockIDFromState(stateID int32) int32 {
	if stateID < 0 || stateID >= genblock.TotalStates {
		return -1
	}
	return genblock.StateToBlock[stateID]
}

// NameFromState returns the block name for a given state ID.
func NameFromState(stateID int32) string {
	bid := BlockIDFromState(stateID)
	if bid < 0 {
		return ""
	}
	return genblock.Blocks[bid].Name
}

// IsAir returns true if the state ID corresponds to air (state 0).
func IsAir(stateID int32) bool {
	return stateID == genblock.AirStateID
}

// IsSolid returns true if the block at the given state ID has a full bounding box.
func IsSolid(stateID int32) bool {
	bid := BlockIDFromState(stateID)
	if bid < 0 {
		return false
	}
	return genblock.Blocks[bid].Solid
}

// StateID computes the state ID for a block name with the given property values.
// Returns -1 if the block is not found or properties are invalid.
func StateID(name string, props map[string]string) int32 {
	info := InfoByName(name)
	if info == nil {
		return -1
	}
	if len(info.Properties) == 0 {
		return info.DefaultState
	}

	stateID := info.MinStateID
	for i, prop := range info.Properties {
		stride := int32(1)
		for j := i + 1; j < len(info.Properties); j++ {
			stride *= int32(len(info.Properties[j].Values))
		}

		val, ok := props[prop.Name]
		if !ok {
			defOffset := info.DefaultState - info.MinStateID
			rem := defOffset
			for k := 0; k < i; k++ {
				s := int32(1)
				for l := k + 1; l < len(info.Properties); l++ {
					s *= int32(len(info.Properties[l].Values))
				}
				rem %= s
			}
			valIdx := rem / stride
			stateID += valIdx * stride
			continue
		}

		found := false
		for vi, v := range prop.Values {
			if v == val {
				stateID += int32(vi) * stride
				found = true
				break
			}
		}
		if !found {
			return -1
		}
	}
	return stateID
}

// PropertiesFromState extracts the property values for a given state ID.
// Returns nil if the block has no properties.
func PropertiesFromState(stateID int32) map[string]string {
	bid := BlockIDFromState(stateID)
	if bid < 0 {
		return nil
	}
	info := &genblock.Blocks[bid]
	if len(info.Properties) == 0 {
		return nil
	}

	offset := stateID - info.MinStateID
	props := make(map[string]string, len(info.Properties))

	for i, prop := range info.Properties {
		stride := int32(1)
		for j := i + 1; j < len(info.Properties); j++ {
			stride *= int32(len(info.Properties[j].Values))
		}
		valIdx := offset / stride
		offset %= stride
		if int(valIdx) < len(prop.Values) {
			props[prop.Name] = prop.Values[valIdx]
		}
	}
	return props
}
