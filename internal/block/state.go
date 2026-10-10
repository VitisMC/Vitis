// Package block provides version-specific block state lookup and validation.
package block

import (
	"fmt"

	genblock "github.com/vitismc/vitis/internal/data/generated/block"
)

// BlockState is an immutable, validated block state for the active version.
// Its zero value is air.
type BlockState struct {
	id int32
}

// BlockStateFromID resolves a protocol state ID.
func BlockStateFromID(id int32) (BlockState, error) {
	if id < 0 || id >= genblock.TotalStates {
		return BlockState{}, fmt.Errorf("invalid block state ID %d", id)
	}
	return BlockState{id: id}, nil
}

// DefaultBlockState resolves the default state of a registered block.
func DefaultBlockState(name string) (BlockState, error) {
	info := InfoByName(name)
	if info == nil {
		return BlockState{}, fmt.Errorf("unknown block %q", name)
	}
	return BlockStateFromID(info.DefaultState)
}

// BlockStateFromProperties resolves a block with validated property values.
// Unspecified properties retain their default values.
func BlockStateFromProperties(name string, properties map[string]string) (BlockState, error) {
	info := InfoByName(name)
	if info == nil {
		return BlockState{}, fmt.Errorf("unknown block %q", name)
	}
	for key, value := range properties {
		found := false
		for _, property := range info.Properties {
			if property.Name != key {
				continue
			}
			found = true
			valid := false
			for _, allowed := range property.Values {
				if allowed == value {
					valid = true
					break
				}
			}
			if !valid {
				return BlockState{}, fmt.Errorf("invalid value %q for %s[%s]", value, name, key)
			}
			break
		}
		if !found {
			return BlockState{}, fmt.Errorf("unknown property %s[%s]", name, key)
		}
	}
	id := StateID(name, properties)
	if id < info.MinStateID || id > info.MaxStateID {
		return BlockState{}, fmt.Errorf("invalid state for block %q", name)
	}
	return BlockState{id: id}, nil
}

// ID returns the protocol state ID.
func (s BlockState) ID() int32 { return s.id }

// Name returns the namespaced block name.
func (s BlockState) Name() string { return NameFromState(s.id) }

// Properties returns a detached copy of the state's properties.
func (s BlockState) Properties() map[string]string { return PropertiesFromState(s.id) }

// Property returns one property value when the block defines it.
func (s BlockState) Property(name string) (string, bool) {
	info := InfoByName(s.Name())
	if info == nil {
		return "", false
	}
	offset := s.id - info.MinStateID
	for i, property := range info.Properties {
		stride := int32(1)
		for _, next := range info.Properties[i+1:] {
			stride *= int32(len(next.Values))
		}
		index := offset / stride
		offset %= stride
		if property.Name == name {
			if index < 0 || int(index) >= len(property.Values) {
				return "", false
			}
			return property.Values[index], true
		}
	}
	return "", false
}

// WithProperty returns a new state with one property changed.
func (s BlockState) WithProperty(name, value string) (BlockState, error) {
	if _, ok := s.Property(name); !ok {
		return BlockState{}, fmt.Errorf("unknown property %s[%s]", s.Name(), name)
	}
	properties := s.Properties()
	properties[name] = value
	return BlockStateFromProperties(s.Name(), properties)
}
