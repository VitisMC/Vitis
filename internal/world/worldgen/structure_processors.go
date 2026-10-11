package worldgen

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	blockentity "github.com/vitismc/vitis/internal/block/entity"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	worldrandom "github.com/vitismc/vitis/internal/world/worldgen/random"
)

// templateProcessor preserves the configured order of block filtering and replacement.
type templateProcessor struct {
	kind      string
	integrity float32
	states    map[int32]struct{}
	rules     []templateRule
}

// templateRule combines input and terrain predicates with a replacement block state.
type templateRule struct {
	input, location templateRuleTest
	output          int32
}

// templateRuleTest stores resolved states and an optional probability predicate.
type templateRuleTest struct {
	states map[int32]struct{}
	random bool
	chance float32
}

// matches consumes predicate RNG only after its block condition succeeds.
func (p templateRuleTest) matches(state int32, random worldrandom.Source) bool {
	if p.states != nil {
		if _, ok := p.states[state]; !ok {
			return false
		}
	}
	return !p.random || random.NextFloat() < p.chance
}

// templateBlockStates resolves all states of a named block or a snapshot-scoped block tag.
func templateBlockStates(snapshot *datapack.Snapshot, reference string) (map[int32]struct{}, error) {
	names := []string{normalizeResourceID(reference)}
	if strings.HasPrefix(reference, "#") {
		var err error
		names, err = snapshot.ResolveTag("block", strings.TrimPrefix(reference, "#"))
		if err != nil {
			return nil, err
		}
	}
	states := make(map[int32]struct{})
	for _, name := range names {
		info := block.InfoByName(normalizeResourceID(name))
		if info == nil {
			return nil, fmt.Errorf("unknown template processor block %s", name)
		}
		for state := info.MinStateID; state <= info.MaxStateID; state++ {
			states[state] = struct{}{}
		}
	}
	return states, nil
}

// decodeTemplateRuleTest compiles the block predicates used by structure replacement rules.
func decodeTemplateRuleTest(snapshot *datapack.Snapshot, raw json.RawMessage) (templateRuleTest, error) {
	var value struct {
		Kind   string     `json:"predicate_type"`
		Block  string     `json:"block"`
		Tag    string     `json:"tag"`
		State  BlockState `json:"block_state"`
		Chance float32    `json:"probability"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return templateRuleTest{}, err
	}
	test := templateRuleTest{}
	var err error
	switch value.Kind {
	case "minecraft:always_true":
	case "minecraft:block_match", "minecraft:random_block_match":
		test.states, err = templateBlockStates(snapshot, value.Block)
	case "minecraft:tag_match":
		test.states, err = templateBlockStates(snapshot, "#"+value.Tag)
	case "minecraft:blockstate_match", "minecraft:random_blockstate_match":
		var state block.BlockState
		state, err = block.BlockStateFromProperties(value.State.Name, value.State.Properties)
		test.states = map[int32]struct{}{state.ID(): {}}
	default:
		return templateRuleTest{}, fmt.Errorf("unsupported template rule predicate %s", value.Kind)
	}
	if err != nil {
		return templateRuleTest{}, err
	}
	test.random = value.Kind == "minecraft:random_block_match" || value.Kind == "minecraft:random_blockstate_match"
	if test.random {
		if math.IsNaN(float64(value.Chance)) || value.Chance < 0 || value.Chance > 1 {
			return templateRuleTest{}, fmt.Errorf("invalid template predicate probability")
		}
		test.chance = value.Chance
	}
	return test, nil
}

// decodeTemplateProcessors resolves a processor list reference or its inline JSON definition.
func decodeTemplateProcessors(snapshot *datapack.Snapshot, raw json.RawMessage) ([]templateProcessor, error) {
	var id string
	if json.Unmarshal(raw, &id) == nil {
		parts := strings.SplitN(normalizeResourceID(id), ":", 2)
		data, ok := snapshot.Get("data/" + parts[0] + "/worldgen/processor_list/" + parts[1] + ".json")
		if !ok {
			return nil, fmt.Errorf("unknown processor list %s", id)
		}
		raw = data
	}
	var value struct {
		Processors []json.RawMessage `json:"processors"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value.Processors == nil {
		return nil, fmt.Errorf("missing template processors")
	}
	processors := make([]templateProcessor, 0, len(value.Processors))
	for index, raw := range value.Processors {
		var entry struct {
			Kind      string          `json:"processor_type"`
			Integrity *float32        `json:"integrity"`
			Rottable  json.RawMessage `json:"rottable_blocks"`
			Value     string          `json:"value"`
			Rules     []struct {
				Input    json.RawMessage `json:"input_predicate"`
				Location json.RawMessage `json:"location_predicate"`
				Position json.RawMessage `json:"position_predicate"`
				Output   BlockState      `json:"output_state"`
				Modifier json.RawMessage `json:"block_entity_modifier"`
			} `json:"rules"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, err
		}
		processor := templateProcessor{kind: entry.Kind}
		var err error
		switch entry.Kind {
		case "minecraft:block_rot":
			if entry.Integrity == nil || *entry.Integrity < 0 || *entry.Integrity > 1 {
				return nil, fmt.Errorf("processor %d: invalid integrity", index)
			}
			processor.integrity = *entry.Integrity
			if len(entry.Rottable) != 0 {
				var tag string
				var names []string
				if json.Unmarshal(entry.Rottable, &tag) == nil {
					names = []string{tag}
				} else if err := json.Unmarshal(entry.Rottable, &names); err != nil {
					return nil, err
				}
				processor.states = make(map[int32]struct{})
				for _, name := range names {
					states, err := templateBlockStates(snapshot, name)
					if err != nil {
						return nil, err
					}
					for state := range states {
						processor.states[state] = struct{}{}
					}
				}
			}
		case "minecraft:protected_blocks":
			if !strings.HasPrefix(entry.Value, "#") {
				return nil, fmt.Errorf("processor %d: expected protected block tag", index)
			}
			processor.states, err = templateBlockStates(snapshot, entry.Value)
		case "minecraft:rule":
			for _, rule := range entry.Rules {
				if len(rule.Position) != 0 {
					var predicate struct {
						Kind string `json:"predicate_type"`
					}
					if err := json.Unmarshal(rule.Position, &predicate); err != nil {
						return nil, err
					}
					if predicate.Kind != "minecraft:always_true" {
						return nil, fmt.Errorf("unsupported template position predicate %s", predicate.Kind)
					}
				}
				if len(rule.Modifier) != 0 {
					var modifier struct {
						Kind string `json:"type"`
					}
					if err := json.Unmarshal(rule.Modifier, &modifier); err != nil {
						return nil, err
					}
					if modifier.Kind != "minecraft:passthrough" {
						return nil, fmt.Errorf("unsupported template NBT modifier %s", modifier.Kind)
					}
				}
				input, err := decodeTemplateRuleTest(snapshot, rule.Input)
				if err != nil {
					return nil, err
				}
				location, err := decodeTemplateRuleTest(snapshot, rule.Location)
				if err != nil {
					return nil, err
				}
				state, err := block.BlockStateFromProperties(rule.Output.Name, rule.Output.Properties)
				if err != nil {
					return nil, err
				}
				processor.rules = append(processor.rules, templateRule{input: input, location: location, output: state.ID()})
			}
		default:
			return nil, fmt.Errorf("unsupported template processor %s", entry.Kind)
		}
		if err != nil {
			return nil, err
		}
		processors = append(processors, processor)
	}
	return processors, nil
}

// apply filters or rewrites a block before placement while reading the unmodified terrain.
func (p templateProcessor) apply(target *section.Chunk, random worldrandom.Source, original, current templateBlock) (templateBlock, bool) {
	switch p.kind {
	case "minecraft:block_rot":
		_, eligible := p.states[original.state]
		if (p.states == nil || eligible) && !(random.NextFloat() <= p.integrity) {
			return current, false
		}
	case "minecraft:protected_blocks":
		if _, protected := p.states[getChunkBlock(target, current.position)]; protected {
			return current, false
		}
	case "minecraft:rule":
		ruleRandom := worldrandom.NewLegacy(worldrandom.PositionSeed(int32(current.position.x), int32(current.position.y), int32(current.position.z)))
		terrain := getChunkBlock(target, current.position)
		for _, rule := range p.rules {
			if rule.input.matches(current.state, ruleRandom) && rule.location.matches(terrain, ruleRandom) {
				current.state = rule.output
				break
			}
		}
	}
	return current, true
}

// place selects a palette, runs processors before writes, and copies block entities per placement.
func (t *structureTemplate) place(target *section.Chunk, random worldrandom.Source, origin blockPosition, transform templateTransform, processors []templateProcessor) bool {
	palette := t.palettes[random.NextIntBound(int32(len(t.palettes)))]
	prepared := make([]templateBlock, 0, len(palette))
	for _, original := range palette {
		position := transform.position(original.position)
		current := templateBlock{position: blockPosition{position.x + origin.x, position.y + origin.y, position.z + origin.z}, state: original.state, data: original.data}
		accepted := true
		for _, processor := range processors {
			current, accepted = processor.apply(target, random, original, current)
			if !accepted {
				break
			}
		}
		if accepted {
			var err error
			current.state, err = transform.state(current.state)
			if err != nil {
				return false
			}
			prepared = append(prepared, current)
		}
	}
	placed := false
	for _, current := range prepared {
		position := current.position
		if !target.FeatureWriteAllowed(position.x, position.y, position.z) {
			continue
		}
		state := current.state
		if block.NameFromState(state) == "minecraft:structure_void" {
			continue
		}
		// Retain ambient water for waterloggable template blocks.
		if block.NameFromState(getChunkBlock(target, position)) == "minecraft:water" {
			base, _ := block.BlockStateFromID(state)
			if _, ok := base.Property("waterlogged"); ok {
				waterlogged, err := base.WithProperty("waterlogged", "true")
				if err == nil {
					state = waterlogged.ID()
				}
			}
		}
		target.RemoveBlockEntity(int32(position.x), int32(position.y), int32(position.z))
		setChunkBlock(target, position, state)
		if current.data != nil {
			data := current.data.Clone().PutInt("x", int32(position.x)).PutInt("y", int32(position.y)).PutInt("z", int32(position.z))
			if templateRandomizableContainer(block.NameFromState(state)) {
				seed := random.NextLong()
				if _, ok := data.GetString("LootTable"); ok {
					data.PutLong("LootTableSeed", seed)
				}
			}
			target.AddBlockEntity(blockentity.NewRawBlockEntity(data))
		}
		placed = true
	}
	return placed
}

// templateRandomizableContainer identifies block entities that consume a placement loot seed.
func templateRandomizableContainer(name string) bool {
	switch name {
	case "minecraft:chest", "minecraft:trapped_chest", "minecraft:barrel", "minecraft:dispenser", "minecraft:dropper", "minecraft:hopper", "minecraft:crafter", "minecraft:decorated_pot":
		return true
	default:
		return strings.HasSuffix(name, "shulker_box")
	}
}
