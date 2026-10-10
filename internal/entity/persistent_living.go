package entity

import (
	"fmt"
	"math"
	"sort"
	"strings"

	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	damagetype "github.com/vitismc/vitis/internal/data/generated/damage_type"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
)

// PersistentNBT captures a mob's living state, attributes, and active effects.
func (m *MobEntity) PersistentNBT() (*nbt.Compound, error) {
	root, err := m.PersistentBaseNBT("minecraft:" + strings.TrimPrefix(m.TypeName(), "minecraft:"))
	if err != nil {
		return nil, err
	}
	l := m.LivingEntity
	if m.TypeName() == "slime" {
		root.PutInt("Size", int32(m.SlimeSize()-1))
	}
	if l.lastDamageType < DamageTypeGeneric || l.lastDamageType > DamageTypeSweetBerryBush || damagetype.DamageTypeIDByName(l.LastDamageRegistryName()) < 0 {
		return nil, fmt.Errorf("invalid persistent damage type")
	}
	if !finite(float64(l.lastDamageAmount)) || l.lastDamageAmount < 0 || l.fallDistance < 0 || l.fallDistance > math.MaxFloat32 || l.hurtCooldown < 0 || l.hurtTime < 0 || l.deathTime < 0 {
		return nil, fmt.Errorf("invalid persistent combat state")
	}
	if !finite(float64(l.health)) || !finite(float64(l.maxHealth)) || l.maxHealth < 1 || !finite(float64(l.absorption)) || !finite(l.fallDistance) || l.health < 0 || l.absorption < 0 || l.hurtTime > math.MaxInt16 || l.deathTime > math.MaxInt16 || l.airSupply < math.MinInt16 || l.airSupply > math.MaxInt16 || l.fireTicks < math.MinInt16 || l.fireTicks > math.MaxInt16 {
		return nil, fmt.Errorf("invalid persistent living state")
	}
	root.PutFloat("Health", l.health).PutFloat("AbsorptionAmount", l.absorption).PutShort("HurtTime", int16(l.hurtTime)).PutShort("DeathTime", int16(l.deathTime)).PutShort("Air", int16(l.airSupply)).PutShort("Fire", int16(l.fireTicks)).PutFloat("FallDistance", float32(l.fallDistance))
	root.PutBool("NoAI", m.noAI).PutLong("vitis:age_ticks", m.ageTicks).PutInt("vitis:hurt_cooldown", l.hurtCooldown).PutBool("vitis:death_handled", l.deathHandled)
	root.PutBool("PersistenceRequired", m.persistenceRequired).PutInt("vitis:no_action_time", m.noActionTime)
	root.PutBool("CanPickUpLoot", m.canPickUpLoot)
	root.PutFloat("vitis:max_health", l.maxHealth).PutFloat("vitis:last_damage", l.lastDamageAmount).PutString("vitis:last_damage_source", l.lastDamageSource)
	root.PutInt("vitis:last_damage_type", int32(l.lastDamageType)).PutString("vitis:last_damage_registry", l.LastDamageRegistryName())
	root.PutBool("vitis:last_damage_direct", l.LastDamageIsDirect())
	root.PutInt("vitis:player_credit_ticks", l.playerCreditTicks).PutString("vitis:last_attacker_type", l.lastAttackerType)
	root.PutString("vitis:last_direct_attacker_type", l.lastDirectType)
	weapon, err := inventory.ItemStackToNBT(l.deathWeapon)
	if err != nil {
		return nil, fmt.Errorf("snapshot death weapon: %w", err)
	}
	root.Delete("vitis:death_weapon")
	if weapon != nil {
		root.PutCompound("vitis:death_weapon", weapon)
	}
	if err := m.saveEquipmentNBT(root); err != nil {
		return nil, err
	}
	attributes, err := l.attributes.persistentNBT(root, m.equipmentModifiers)
	if err != nil {
		return nil, err
	}
	root.PutList("attributes", attributes)
	active := nbt.NewList(nbt.TagCompound)
	previous := indexedCompounds(root, "active_effects", "id")
	if old, ok := root.GetList("active_effects"); ok {
		for _, value := range old.Elements() {
			raw, ok := value.(*nbt.Compound)
			if !ok {
				continue
			}
			name, _ := raw.GetString("id")
			if persistentEffectID(name) < 0 {
				active.Add(raw.Clone())
			}
		}
	}
	effects := l.effects.Active()
	sort.Slice(effects, func(i, j int) bool { return effects[i].ID < effects[j].ID })
	for _, inst := range effects {
		name := persistentEffectName(inst.ID)
		if name == "" || inst.Amplifier < 0 || inst.Amplifier > 255 || inst.Duration <= 0 {
			return nil, fmt.Errorf("invalid persistent effect")
		}
		raw := previous[name].Clone()
		if raw == nil {
			raw = nbt.NewCompound()
		}
		raw.PutString("id", name).PutByte("amplifier", int8(inst.Amplifier)).PutInt("duration", inst.Duration).PutBool("ambient", inst.IsAmbient()).PutBool("show_particles", inst.ShowParticles()).PutBool("show_icon", inst.ShowIcon())
		active.Add(raw)
	}
	root.PutList("active_effects", active)
	return root, nil
}

// restoreLivingNBT validates supported living fields before publishing a mob.
func (m *MobEntity) restoreLivingNBT(root *nbt.Compound) error {
	if err := ValidatePersistentTags(root, map[string]byte{"PersistenceRequired": nbt.TagByte, "CanPickUpLoot": nbt.TagByte, "vitis:no_action_time": nbt.TagInt}); err != nil {
		return err
	}
	m.persistenceRequired, _ = root.GetBool("PersistenceRequired")
	m.canPickUpLoot, _ = root.GetBool("CanPickUpLoot")
	m.noActionTime, _ = root.GetInt("vitis:no_action_time")
	if m.noActionTime < 0 {
		return fmt.Errorf("invalid despawn lifetime")
	}
	if err := ValidatePersistentTags(root, map[string]byte{"vitis:player_credit_ticks": nbt.TagInt, "vitis:last_attacker_type": nbt.TagString, "vitis:last_direct_attacker_type": nbt.TagString, "vitis:death_weapon": nbt.TagCompound}); err != nil {
		return err
	}
	if err := ValidatePersistentTags(root, map[string]byte{"Health": nbt.TagFloat, "AbsorptionAmount": nbt.TagFloat, "HurtTime": nbt.TagShort, "DeathTime": nbt.TagShort, "Air": nbt.TagShort, "Fire": nbt.TagShort, "FallDistance": nbt.TagFloat, "attributes": nbt.TagList, "active_effects": nbt.TagList, "vitis:hurt_cooldown": nbt.TagInt, "vitis:death_handled": nbt.TagByte}); err != nil {
		return err
	}
	l := m.LivingEntity
	l.playerCreditTicks, _ = root.GetInt("vitis:player_credit_ticks")
	if l.playerCreditTicks < 0 || l.playerCreditTicks > 100 {
		return fmt.Errorf("invalid kill credit lifetime")
	}
	l.lastAttackerType, _ = root.GetString("vitis:last_attacker_type")
	l.lastDirectType, _ = root.GetString("vitis:last_direct_attacker_type")
	if weapon, ok := root.GetCompound("vitis:death_weapon"); ok {
		var err error
		l.deathWeapon, err = inventory.ItemStackFromNBT(weapon)
		if err != nil {
			return fmt.Errorf("restore death weapon: %w", err)
		}
	}
	if err := ValidatePersistentTags(root, map[string]byte{"vitis:max_health": nbt.TagFloat, "vitis:last_damage": nbt.TagFloat, "vitis:last_damage_source": nbt.TagString, "vitis:last_damage_type": nbt.TagInt, "vitis:last_damage_registry": nbt.TagString, "vitis:last_damage_direct": nbt.TagByte}); err != nil {
		return err
	}
	if list, ok := root.GetList("attributes"); ok {
		if err := l.attributes.restorePersistentNBT(list); err != nil {
			return err
		}
	}
	if err := m.restoreEquipmentNBT(root); err != nil {
		return err
	}
	if _, ok := root.GetList("attributes"); ok {
		if _, ok := l.attributes.attrs[genattr.AttrMaxHealth]; ok {
			value := l.attributes.GetValue(genattr.AttrMaxHealth)
			if !finite(value) || value < 1 || value > math.MaxFloat32 {
				return fmt.Errorf("invalid maximum health")
			}
			l.maxHealth = float32(value)
		}
	}
	if maximum, ok := root.GetFloat("vitis:max_health"); ok {
		if !finite(float64(maximum)) || maximum < 1 {
			return fmt.Errorf("invalid saved maximum health")
		}
		l.maxHealth = maximum
	}
	if amount, ok := root.GetFloat("vitis:last_damage"); ok {
		if !finite(float64(amount)) || amount < 0 {
			return fmt.Errorf("invalid saved damage")
		}
		l.lastDamageAmount = amount
	}
	l.lastDamageSource, _ = root.GetString("vitis:last_damage_source")
	if saved, ok := root.GetInt("vitis:last_damage_type"); ok {
		if saved < int32(DamageTypeGeneric) || saved > int32(DamageTypeSweetBerryBush) {
			return fmt.Errorf("invalid saved damage type")
		}
		l.lastDamageType = DamageType(saved)
	} else {
		l.lastDamageType = damageTypeFromSource(l.lastDamageSource)
	}
	l.lastDamageRegistryName = l.lastDamageType.RegistryName()
	if name, ok := root.GetString("vitis:last_damage_registry"); ok {
		if damagetype.DamageTypeIDByName(name) < 0 {
			return fmt.Errorf("invalid saved damage registry name")
		}
		l.lastDamageRegistryName = name
	}
	if direct, ok := root.GetBool("vitis:last_damage_direct"); ok {
		l.lastDamageIsDirect = direct
	} else {
		l.lastDamageIsDirect = !l.lastDamageType.IsProjectile() && l.lastAttackerType == l.lastDirectType
	}
	if health, ok := root.GetFloat("Health"); ok {
		if !finite(float64(health)) || health < 0 {
			return fmt.Errorf("invalid mob health")
		}
		l.SetHealth(health)
	}
	if absorption, ok := root.GetFloat("AbsorptionAmount"); ok {
		if !finite(float64(absorption)) || absorption < 0 {
			return fmt.Errorf("invalid absorption")
		}
		l.absorption = absorption
	}
	if distance, ok := root.GetFloat("FallDistance"); ok {
		if !finite(float64(distance)) || distance < 0 {
			return fmt.Errorf("invalid fall distance")
		}
		l.fallDistance = float64(distance)
	}
	if v, ok := root.GetShort("Air"); ok {
		l.airSupply = int32(v)
	}
	if v, ok := root.GetShort("Fire"); ok {
		l.SetFireTicks(int32(v))
	}
	if v, ok := root.GetShort("HurtTime"); ok {
		if v < 0 {
			return fmt.Errorf("invalid hurt time")
		}
		l.hurtTime = int32(v)
	}
	if v, ok := root.GetShort("DeathTime"); ok {
		if v < 0 {
			return fmt.Errorf("invalid death time")
		}
		l.deathTime = int32(v)
	}
	l.hurtCooldown, _ = root.GetInt("vitis:hurt_cooldown")
	if l.hurtCooldown < 0 {
		return fmt.Errorf("invalid saved hurt cooldown")
	}
	l.deathHandled, _ = root.GetBool("vitis:death_handled")
	if list, ok := root.GetList("active_effects"); ok {
		if list.ElementType() != nbt.TagCompound && !(list.ElementType() == nbt.TagEnd && list.Len() == 0) {
			return fmt.Errorf("invalid active effect list")
		}
		seen := make(map[int32]bool)
		for _, value := range list.Elements() {
			raw, ok := value.(*nbt.Compound)
			if !ok {
				return fmt.Errorf("invalid effect entry")
			}
			name, _ := raw.GetString("id")
			id := persistentEffectID(name)
			if id < 0 {
				continue
			}
			if seen[id] {
				return fmt.Errorf("duplicate effect %s", name)
			}
			seen[id] = true
			if err := ValidatePersistentTags(raw, map[string]byte{"duration": nbt.TagInt, "amplifier": nbt.TagByte, "ambient": nbt.TagByte, "show_particles": nbt.TagByte, "show_icon": nbt.TagByte}); err != nil {
				return err
			}
			duration, ok := raw.GetInt("duration")
			if !ok || duration <= 0 {
				return fmt.Errorf("unsupported effect duration")
			}
			amplifier, _ := raw.GetByte("amplifier")
			inst := effect.Instance{ID: id, Duration: duration, Amplifier: int32(uint8(amplifier)), Flags: effect.DefaultFlags()}
			if v, _ := raw.GetBool("ambient"); v {
				inst.Flags |= effect.FlagAmbient
			}
			if v, ok := raw.GetBool("show_particles"); ok && !v {
				inst.Flags &^= effect.FlagParticles
			}
			if v, ok := raw.GetBool("show_icon"); ok && !v {
				inst.Flags &^= effect.FlagIcon
			}
			l.effects.Add(inst)
		}
	}
	return nil
}

// indexedCompounds indexes named list entries while retaining unknown entry data.
func indexedCompounds(root *nbt.Compound, key, identity string) map[string]*nbt.Compound {
	result := make(map[string]*nbt.Compound)
	if list, ok := root.GetList(key); ok {
		for _, value := range list.Elements() {
			if raw, ok := value.(*nbt.Compound); ok {
				if name, ok := raw.GetString(identity); ok {
					result[name] = raw
				}
			}
		}
	}
	return result
}

// persistentNBT snapshots attributes and preserves unknown attribute and modifier tags.
func (a *AttributeContainer) persistentNBT(root *nbt.Compound, equipment ...map[equipmentModifierKey]Modifier) (*nbt.List, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	previous := indexedCompounds(root, "attributes", "id")
	list := nbt.NewList(nbt.TagCompound)
	if old, ok := root.GetList("attributes"); ok {
		for _, value := range old.Elements() {
			raw, ok := value.(*nbt.Compound)
			if !ok {
				continue
			}
			name, _ := raw.GetString("id")
			if genattr.AttributeIDByName(name) < 0 {
				list.Add(raw.Clone())
			}
		}
	}
	ids := make([]int, 0, len(a.attrs))
	for id := range a.attrs {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, id := range ids {
		info := genattr.AttributeByID(int32(id))
		attr := a.attrs[int32(id)]
		if info == nil || attr == nil || !finite(attr.Base) || !finite(attr.Value()) {
			return nil, fmt.Errorf("invalid persistent attribute")
		}
		raw := previous[info.Name].Clone()
		if raw == nil {
			raw = nbt.NewCompound()
		}
		raw.PutString("id", info.Name).PutDouble("base", attr.Base)
		oldModifiers := indexedCompounds(raw, "modifiers", "id")
		modifiers := nbt.NewList(nbt.TagCompound)
		for _, mod := range attr.Modifiers {
			if len(equipment) != 0 {
				if _, transient := equipment[0][equipmentModifierKey{int32(id), mod.ID}]; transient {
					continue
				}
			}
			if mod.ID == "" || !finite(mod.Amount) || mod.Operation < 0 || mod.Operation > 2 {
				return nil, fmt.Errorf("invalid attribute modifier")
			}
			entry := oldModifiers[mod.ID].Clone()
			if entry == nil {
				entry = nbt.NewCompound()
			}
			operation := []string{"add_value", "add_multiplied_base", "add_multiplied_total"}[mod.Operation]
			entry.PutString("id", mod.ID).PutDouble("amount", mod.Amount).PutString("operation", operation)
			modifiers.Add(entry)
		}
		raw.PutList("modifiers", modifiers)
		list.Add(raw)
	}
	return list, nil
}

// restorePersistentNBT validates attributes without exposing partially restored state.
func (a *AttributeContainer) restorePersistentNBT(list *nbt.List) error {
	if list.ElementType() != nbt.TagCompound && !(list.ElementType() == nbt.TagEnd && list.Len() == 0) {
		return fmt.Errorf("invalid attribute list")
	}
	seenAttributes := make(map[int32]bool)
	for _, value := range list.Elements() {
		raw, ok := value.(*nbt.Compound)
		if !ok {
			return fmt.Errorf("invalid attribute entry")
		}
		name, _ := raw.GetString("id")
		id := genattr.AttributeIDByName(name)
		if id < 0 {
			continue
		}
		if seenAttributes[id] {
			return fmt.Errorf("duplicate persistent attribute %s", name)
		}
		seenAttributes[id] = true
		base, ok := raw.GetDouble("base")
		if !ok || !finite(base) {
			return fmt.Errorf("invalid attribute base")
		}
		attr := &Attribute{Base: base}
		if err := ValidatePersistentTags(raw, map[string]byte{"modifiers": nbt.TagList}); err != nil {
			return err
		}
		if modifiers, ok := raw.GetList("modifiers"); ok {
			if modifiers.ElementType() != nbt.TagCompound && !(modifiers.ElementType() == nbt.TagEnd && modifiers.Len() == 0) {
				return fmt.Errorf("invalid modifier list")
			}
			seen := make(map[string]bool)
			for _, value := range modifiers.Elements() {
				entry, ok := value.(*nbt.Compound)
				if !ok {
					return fmt.Errorf("invalid modifier entry")
				}
				name, ok := entry.GetString("id")
				if !ok || name == "" || seen[name] {
					return fmt.Errorf("invalid modifier identity")
				}
				seen[name] = true
				amount, ok := entry.GetDouble("amount")
				if !ok || !finite(amount) {
					return fmt.Errorf("invalid modifier amount")
				}
				operation, _ := entry.GetString("operation")
				op, ok := map[string]int8{"add_value": 0, "add_multiplied_base": 1, "add_multiplied_total": 2}[operation]
				if !ok {
					return fmt.Errorf("unsupported modifier operation")
				}
				attr.Modifiers = append(attr.Modifiers, Modifier{ID: name, Amount: amount, Operation: op})
			}
		}
		a.attrs[id] = attr
	}
	return nil
}

// persistentEffectName converts a registry effect name to its persistent identifier.
func persistentEffectName(id int32) string {
	if id < 0 || int(id) >= len(effectdata.Effects) {
		return ""
	}
	var result strings.Builder
	result.WriteString("minecraft:")
	for i, r := range effectdata.Effects[id].Name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + ('a' - 'A'))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// persistentEffectID resolves a persistent status-effect identifier.
func persistentEffectID(name string) int32 {
	for _, info := range effectdata.Effects {
		if persistentEffectName(info.ID) == name {
			return info.ID
		}
	}
	return -1
}
