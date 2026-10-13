package worldgen

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/data/generated/collision"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
)

const maxTemplateBytes = 16 << 20

// structureTemplate owns immutable palettes and entity payloads from one resource snapshot.
type structureTemplate struct {
	size     blockPosition
	palettes [][]templateBlock
	entities []templateEntity
}

// templateBlock records local geometry and an optional immutable block entity payload.
type templateBlock struct {
	position blockPosition
	state    int32
	data     *nbt.Compound
}

// templateEntity retains continuous positions and NBT for later structure entity integration.
type templateEntity struct {
	position [3]float64
	block    blockPosition
	data     *nbt.Compound
}

// structureTemplates resolves each resource once without sharing mutable data across reloads.
type structureTemplates struct {
	snapshot *datapack.Snapshot
	entries  map[string]*templateResource
}

// templateResource serializes lazy decoding of one indexed template.
type templateResource struct {
	path  string
	once  sync.Once
	value *structureTemplate
	err   error
}

// newStructureTemplates indexes template resources without eagerly decoding large structures.
func newStructureTemplates(snapshot *datapack.Snapshot) *structureTemplates {
	store := &structureTemplates{snapshot: snapshot, entries: make(map[string]*templateResource)}
	if snapshot == nil {
		return store
	}
	for _, path := range snapshot.Names() {
		parts := strings.SplitN(path, "/", 4)
		if len(parts) != 4 || parts[0] != "data" || parts[2] != "structure" || !strings.HasSuffix(parts[3], ".nbt") {
			continue
		}
		id := parts[1] + ":" + strings.TrimSuffix(parts[3], ".nbt")
		store.entries[id] = &templateResource{path: path}
	}
	return store
}

// load returns an immutable template or the same cached validation error to all callers.
func (s *structureTemplates) load(id string) (*structureTemplate, error) {
	id = normalizeResourceID(id)
	entry := s.entries[id]
	if entry == nil {
		return nil, fmt.Errorf("unknown structure template %s", id)
	}
	entry.once.Do(func() {
		data, ok := s.snapshot.Get(entry.path)
		if !ok {
			entry.err = fmt.Errorf("missing resource %s", entry.path)
			return
		}
		entry.value, entry.err = decodeStructureTemplate(data)
		if entry.err != nil {
			entry.err = fmt.Errorf("structure template %s: %w", id, entry.err)
		}
	})
	return entry.value, entry.err
}

// decodeStructureTemplate bounds decompression and validates palettes before publishing a template.
func decodeStructureTemplate(data []byte) (*structureTemplate, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip header: %w", err)
		}
		decoded, readErr := io.ReadAll(io.LimitReader(reader, maxTemplateBytes+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("gzip body: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("gzip close: %w", closeErr)
		}
		data = decoded
	}
	decoder := nbt.NewDecoderWithLimits(data, nbt.DecoderLimits{MaxBytes: maxTemplateBytes, MaxDepth: 64, MaxNodes: 1 << 20, MaxCollectionLength: 1 << 20})
	_, root, err := decoder.ReadNamedRootCompound()
	if err != nil {
		return nil, err
	}
	if decoder.Pos() != len(data) {
		return nil, fmt.Errorf("trailing NBT data")
	}
	size, err := templateIntPosition(root, "size")
	if err != nil || size.x < 1 || size.y < 1 || size.z < 1 || size.x > 512 || size.y > 512 || size.z > 512 || int64(size.x)*int64(size.y)*int64(size.z) > 16<<20 {
		return nil, fmt.Errorf("invalid template size %v: %v", size, err)
	}
	template := &structureTemplate{size: size}
	var paletteTags []*nbt.List
	if value, _, exists := root.Get("palettes"); exists {
		list, ok := value.(*nbt.List)
		if !ok || list.ElementType() != nbt.TagList || list.Len() == 0 || list.Len() > 256 {
			return nil, fmt.Errorf("invalid template palettes")
		}
		for _, value := range list.Elements() {
			palette, ok := value.(*nbt.List)
			if !ok {
				return nil, fmt.Errorf("invalid template palette")
			}
			paletteTags = append(paletteTags, palette)
		}
	} else {
		palette, ok := root.GetList("palette")
		if !ok {
			return nil, fmt.Errorf("missing template palette")
		}
		paletteTags = append(paletteTags, palette)
	}
	blocks, ok := root.GetList("blocks")
	if !ok || blocks.Len() != 0 && blocks.ElementType() != nbt.TagCompound {
		return nil, fmt.Errorf("invalid template blocks")
	}
	if int64(blocks.Len())*int64(len(paletteTags)) > 1<<20 {
		return nil, fmt.Errorf("template palette expansion exceeds block limit")
	}
	for _, tags := range paletteTags {
		states, err := decodeTemplatePalette(tags)
		if err != nil {
			return nil, err
		}
		palette := make([]templateBlock, 0, blocks.Len())
		for index, tag := range blocks.Elements() {
			entry, ok := tag.(*nbt.Compound)
			if !ok {
				return nil, fmt.Errorf("invalid template block %d", index)
			}
			position, err := templateIntPosition(entry, "pos")
			stateIndex, valid := entry.GetInt("state")
			if err != nil || position.x < 0 || position.y < 0 || position.z < 0 || position.x >= size.x || position.y >= size.y || position.z >= size.z || !valid || stateIndex < 0 || int(stateIndex) >= len(states) {
				return nil, fmt.Errorf("invalid template block %d position or palette index", index)
			}
			payload, err := templateOptionalCompound(entry, "nbt")
			if err != nil {
				return nil, err
			}
			palette = append(palette, templateBlock{position: position, state: states[stateIndex], data: payload})
		}
		// Static full blocks precede other blocks, followed by block entities; each group is Y/X/Z ordered.
		sort.SliceStable(palette, func(i, j int) bool {
			a, b := palette[i], palette[j]
			ac, bc := templateBlockClass(a), templateBlockClass(b)
			if ac != bc {
				return ac < bc
			}
			if a.position.y != b.position.y {
				return a.position.y < b.position.y
			}
			if a.position.x != b.position.x {
				return a.position.x < b.position.x
			}
			return a.position.z < b.position.z
		})
		template.palettes = append(template.palettes, palette)
	}
	if value, _, exists := root.Get("entities"); exists {
		entities, ok := value.(*nbt.List)
		if !ok || entities.Len() != 0 && entities.ElementType() != nbt.TagCompound {
			return nil, fmt.Errorf("invalid template entities")
		}
		for index, value := range entities.Elements() {
			entry, ok := value.(*nbt.Compound)
			if !ok {
				return nil, fmt.Errorf("invalid template entity %d", index)
			}
			position, ok := entry.GetList("pos")
			if !ok || position.ElementType() != nbt.TagDouble || position.Len() != 3 {
				return nil, fmt.Errorf("invalid template entity position %d", index)
			}
			entity := templateEntity{}
			for axis, value := range position.Elements() {
				coordinate, ok := value.(float64)
				if !ok || math.IsNaN(coordinate) || math.IsInf(coordinate, 0) || math.Abs(coordinate) > 1024 {
					return nil, fmt.Errorf("invalid template entity coordinate %d", index)
				}
				entity.position[axis] = coordinate
			}
			entity.block, err = templateIntPosition(entry, "blockPos")
			if err != nil {
				return nil, err
			}
			entity.data, err = templateOptionalCompound(entry, "nbt")
			if err != nil || entity.data == nil {
				return nil, fmt.Errorf("invalid template entity NBT %d", index)
			}
			template.entities = append(template.entities, entity)
		}
	}
	return template, nil
}

// templateIntPosition validates a three-element integer coordinate list.
func templateIntPosition(root *nbt.Compound, name string) (blockPosition, error) {
	list, ok := root.GetList(name)
	if !ok || list.ElementType() != nbt.TagInt || list.Len() != 3 {
		return blockPosition{}, fmt.Errorf("invalid %s coordinate list", name)
	}
	var result [3]int
	for index, value := range list.Elements() {
		integer, ok := value.(int32)
		if !ok {
			return blockPosition{}, fmt.Errorf("invalid %s coordinate", name)
		}
		result[index] = int(integer)
	}
	return blockPosition{result[0], result[1], result[2]}, nil
}

// templateOptionalCompound rejects incorrectly typed block and entity payloads.
func templateOptionalCompound(root *nbt.Compound, name string) (*nbt.Compound, error) {
	value, _, exists := root.Get(name)
	if !exists {
		return nil, nil
	}
	compound, ok := value.(*nbt.Compound)
	if !ok {
		return nil, fmt.Errorf("invalid template %s compound", name)
	}
	return compound, nil
}

// decodeTemplatePalette resolves every state and property without silently substituting air.
func decodeTemplatePalette(list *nbt.List) ([]int32, error) {
	if list.Len() == 0 {
		return nil, nil
	}
	if list.ElementType() != nbt.TagCompound {
		return nil, fmt.Errorf("invalid template state palette")
	}
	states := make([]int32, list.Len())
	for index, value := range list.Elements() {
		entry, ok := value.(*nbt.Compound)
		if !ok {
			return nil, fmt.Errorf("invalid palette entry %d", index)
		}
		name, ok := entry.GetString("Name")
		if !ok {
			return nil, fmt.Errorf("missing palette block name %d", index)
		}
		properties := make(map[string]string)
		props, err := templateOptionalCompound(entry, "Properties")
		if err != nil {
			return nil, err
		}
		if props != nil {
			properties = props.GetAllStrings()
			if len(properties) != len(props.Entries()) {
				return nil, fmt.Errorf("invalid palette property type")
			}
		}
		state, err := block.BlockStateFromProperties(normalizeResourceID(name), properties)
		if err != nil {
			return nil, fmt.Errorf("palette entry %d: %w", index, err)
		}
		states[index] = state.ID()
	}
	return states, nil
}

// templateBlockClass preserves placement order for solids, other blocks, and NBT-bearing blocks.
func templateBlockClass(entry templateBlock) int {
	if entry.data != nil {
		return 2
	}
	name := block.NameFromState(entry.state)
	if strings.HasSuffix(name, "shulker_box") || name == "minecraft:moving_piston" || name == "minecraft:bamboo" || name == "minecraft:scaffolding" || name == "minecraft:powder_snow" || name == "minecraft:pointed_dripstone" {
		return 1
	}
	for _, shape := range collision.ShapesForStateID(entry.state) {
		if shape.MinX == 0 && shape.MinY == 0 && shape.MinZ == 0 && shape.MaxX == 1 && shape.MaxY == 1 && shape.MaxZ == 1 {
			return 0
		}
	}
	return 1
}
