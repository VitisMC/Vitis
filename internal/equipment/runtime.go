package equipment

import (
	"fmt"
	"strings"

	damage "github.com/vitismc/vitis/internal/data/generated/damage_type"
	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/inventory"
)

const compiledDamageTags = "equipment_damage_tags"

// damageTags holds immutable damage-type tag membership for equipment components.
type damageTags map[string]map[string]bool

// equipmentDefinitions keeps damage resistance and pickup rules in one immutable snapshot.
type equipmentDefinitions struct {
	damage damageTags
	pickup *PickupRules
}

// Runtime resolves equipment damage resistance and pickup from published resources.
type Runtime struct{ resources *datapack.Manager }

// NewRuntime registers equipment definition validation before the first resource reload.
func NewRuntime(resources *datapack.Manager) (*Runtime, error) {
	if resources == nil {
		return nil, fmt.Errorf("create equipment runtime: nil resources")
	}
	if err := resources.RegisterCompiler(compiledDamageTags, compileDamageTags); err != nil {
		return nil, err
	}
	return &Runtime{resources: resources}, nil
}

// ResistsDamage resolves the full stack's effective damage-resistant tag.
func (r *Runtime) ResistsDamage(stack inventory.ItemStack, source string) bool {
	component, present := stack.Component(dc.ComponentDamageResistant)
	value, ok := component.(inventory.StructuredComponent)
	tag, valid := value.Data.(string)
	if !present || !ok || !valid {
		return false
	}
	if r != nil {
		if snapshot := r.resources.Current(); snapshot != nil {
			compiled, _ := snapshot.Compiled(compiledDamageTags)
			definitions, _ := compiled.(*equipmentDefinitions)
			return definitions != nil && definitions.damage[tag][source]
		}
	}
	// Worlds without a resource manager retain the native fire-resistant item rule.
	if tag == "minecraft:is_fire" {
		switch source {
		case "minecraft:in_fire", "minecraft:campfire", "minecraft:on_fire", "minecraft:lava", "minecraft:hot_floor", "minecraft:unattributed_fireball", "minecraft:fireball":
			return true
		}
	}
	return false
}

// compileDamageTags validates damage tags and pickup definitions before publication.
func compileDamageTags(snapshot *datapack.Snapshot) (any, error) {
	tags := damageTags{}
	for _, name := range snapshot.Names() {
		parts := strings.SplitN(name, "/", 3)
		if len(parts) != 3 || parts[0] != "data" || !strings.HasPrefix(parts[2], "tags/damage_type/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := parts[1] + ":" + strings.TrimSuffix(strings.TrimPrefix(parts[2], "tags/damage_type/"), ".json")
		members, err := snapshot.ResolveTag("damage_type", id)
		if err != nil {
			return nil, err
		}
		tags[id] = make(map[string]bool, len(members))
		for _, member := range members {
			if damage.DamageTypeIDByName(member) < 0 {
				return nil, fmt.Errorf("equipment damage tag %s: unknown type %s", id, member)
			}
			tags[id][member] = true
		}
	}
	pickup, err := compilePickupRules(snapshot)
	if err != nil {
		return nil, err
	}
	return &equipmentDefinitions{damage: tags, pickup: pickup}, nil
}
