package enchantment

import (
	enchdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
)

// Entry represents a single enchantment with its level on an item.
type Entry struct {
	ID    int32
	Level int32
}

// List holds all enchantments applied to an item.
type List struct {
	entries []Entry
}

// NewList creates an empty enchantment list.
func NewList() *List {
	return &List{}
}

// FromEntries creates a list from the given entries.
func FromEntries(entries []Entry) *List {
	l := &List{entries: make([]Entry, len(entries))}
	copy(l.entries, entries)
	return l
}

// Add adds or upgrades an enchantment. Returns false if incompatible or exceeds max level.
func (l *List) Add(id, level int32) bool {
	info := enchdata.EnchantmentByID(id)
	if info == nil {
		return false
	}
	if level < 1 || level > info.MaxLevel {
		return false
	}

	for _, existing := range l.entries {
		if existing.ID == id {
			continue
		}
		if AreExclusive(existing.ID, id) {
			return false
		}
	}

	for i, e := range l.entries {
		if e.ID == id {
			l.entries[i].Level = level
			return true
		}
	}

	l.entries = append(l.entries, Entry{ID: id, Level: level})
	return true
}

// Set forces an enchantment to a given level, bypassing compatibility checks.
func (l *List) Set(id, level int32) {
	for i, e := range l.entries {
		if e.ID == id {
			if level <= 0 {
				l.entries = append(l.entries[:i], l.entries[i+1:]...)
			} else {
				l.entries[i].Level = level
			}
			return
		}
	}
	if level > 0 {
		l.entries = append(l.entries, Entry{ID: id, Level: level})
	}
}

// Remove removes an enchantment by ID. Returns true if it existed.
func (l *List) Remove(id int32) bool {
	for i, e := range l.entries {
		if e.ID == id {
			l.entries = append(l.entries[:i], l.entries[i+1:]...)
			return true
		}
	}
	return false
}

// Has returns true if the list contains the given enchantment.
func (l *List) Has(id int32) bool {
	for _, e := range l.entries {
		if e.ID == id {
			return true
		}
	}
	return false
}

// Level returns the level of an enchantment, or 0 if not present.
func (l *List) Level(id int32) int32 {
	for _, e := range l.entries {
		if e.ID == id {
			return e.Level
		}
	}
	return 0
}

// Entries returns a copy of all entries.
func (l *List) Entries() []Entry {
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Count returns the number of enchantments.
func (l *List) Count() int {
	return len(l.entries)
}

// Clear removes all enchantments.
func (l *List) Clear() {
	l.entries = l.entries[:0]
}

const (
	GroupDamage     = iota
	GroupProtection
	GroupFortune
	GroupRiptide
	GroupInfinity
	GroupCrossbow
)

var exclusiveGroups = map[int32]int{
	enchdata.EnchantmentIDByName("minecraft:sharpness"):             GroupDamage,
	enchdata.EnchantmentIDByName("minecraft:smite"):                 GroupDamage,
	enchdata.EnchantmentIDByName("minecraft:bane_of_arthropods"):    GroupDamage,
	enchdata.EnchantmentIDByName("minecraft:breach"):                GroupDamage,
	enchdata.EnchantmentIDByName("minecraft:density"):               GroupDamage,
	enchdata.EnchantmentIDByName("minecraft:protection"):            GroupProtection,
	enchdata.EnchantmentIDByName("minecraft:fire_protection"):       GroupProtection,
	enchdata.EnchantmentIDByName("minecraft:blast_protection"):      GroupProtection,
	enchdata.EnchantmentIDByName("minecraft:projectile_protection"): GroupProtection,
	enchdata.EnchantmentIDByName("minecraft:fortune"):               GroupFortune,
	enchdata.EnchantmentIDByName("minecraft:silk_touch"):            GroupFortune,
	enchdata.EnchantmentIDByName("minecraft:riptide"):               GroupRiptide,
	enchdata.EnchantmentIDByName("minecraft:loyalty"):               GroupRiptide,
	enchdata.EnchantmentIDByName("minecraft:channeling"):            GroupRiptide,
	enchdata.EnchantmentIDByName("minecraft:infinity"):              GroupInfinity,
	enchdata.EnchantmentIDByName("minecraft:mending"):               GroupInfinity,
	enchdata.EnchantmentIDByName("minecraft:multishot"):             GroupCrossbow,
	enchdata.EnchantmentIDByName("minecraft:piercing"):              GroupCrossbow,
}

// AreExclusive returns true if two enchantments cannot coexist on the same item.
func AreExclusive(a, b int32) bool {
	ga, oka := exclusiveGroups[a]
	gb, okb := exclusiveGroups[b]
	if !oka || !okb {
		return false
	}
	return ga == gb
}

// ProtectionFactor calculates the total protection factor from an enchantment list.
// Each protection enchantment adds (level * typeMultiplier) capped at 20.
func ProtectionFactor(l *List) float64 {
	if l == nil {
		return 0
	}
	var factor float64
	protID := enchdata.EnchantmentIDByName("minecraft:protection")
	if lvl := l.Level(protID); lvl > 0 {
		factor += float64(lvl)
	}
	return factor
}

// FireProtectionFactor returns the fire protection bonus.
func FireProtectionFactor(l *List) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:fire_protection")
	lvl := l.Level(id)
	return float64(lvl) * 2
}

// BlastProtectionFactor returns the blast protection bonus.
func BlastProtectionFactor(l *List) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:blast_protection")
	lvl := l.Level(id)
	return float64(lvl) * 2
}

// ProjectileProtectionFactor returns the projectile protection bonus.
func ProjectileProtectionFactor(l *List) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:projectile_protection")
	lvl := l.Level(id)
	return float64(lvl) * 2
}

// SharpnessDamage returns the extra damage from sharpness.
func SharpnessDamage(l *List) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:sharpness")
	lvl := l.Level(id)
	if lvl <= 0 {
		return 0
	}
	return 0.5*float64(lvl) + 0.5
}

// SmiteDamage returns the extra damage from smite (against undead).
func SmiteDamage(l *List) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:smite")
	return float64(l.Level(id)) * 2.5
}

// BaneOfArthropodsDamage returns the extra damage from bane of arthropods.
func BaneOfArthropodsDamage(l *List) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:bane_of_arthropods")
	return float64(l.Level(id)) * 2.5
}

// KnockbackLevel returns the knockback enchantment level.
func KnockbackLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:knockback")
	return l.Level(id)
}

// FireAspectLevel returns the fire aspect enchantment level.
func FireAspectLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:fire_aspect")
	return l.Level(id)
}

// LootingLevel returns the looting enchantment level.
func LootingLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:looting")
	return l.Level(id)
}

// UnbreakingLevel returns the unbreaking enchantment level.
func UnbreakingLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:unbreaking")
	return l.Level(id)
}

// EfficiencyLevel returns the efficiency enchantment level.
func EfficiencyLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:efficiency")
	return l.Level(id)
}

// FortuneLevel returns the fortune enchantment level.
func FortuneLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:fortune")
	return l.Level(id)
}

// HasSilkTouch returns whether the item has silk touch.
func HasSilkTouch(l *List) bool {
	if l == nil {
		return false
	}
	id := enchdata.EnchantmentIDByName("minecraft:silk_touch")
	return l.Has(id)
}

// HasMending returns whether the item has mending.
func HasMending(l *List) bool {
	if l == nil {
		return false
	}
	id := enchdata.EnchantmentIDByName("minecraft:mending")
	return l.Has(id)
}

// FeatherFallingLevel returns the feather falling enchantment level.
func FeatherFallingLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:feather_falling")
	return l.Level(id)
}

// ThornsLevel returns the thorns enchantment level.
func ThornsLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:thorns")
	return l.Level(id)
}

// DepthStriderLevel returns the depth strider enchantment level.
func DepthStriderLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:depth_strider")
	return l.Level(id)
}

// RespirationLevel returns the respiration enchantment level.
func RespirationLevel(l *List) int32 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:respiration")
	return l.Level(id)
}

// PowerDamage returns the extra damage from the power enchantment (bows).
func PowerDamage(l *List, baseDamage float64) float64 {
	if l == nil {
		return 0
	}
	id := enchdata.EnchantmentIDByName("minecraft:power")
	lvl := l.Level(id)
	if lvl <= 0 {
		return 0
	}
	return baseDamage*0.25*float64(lvl+1) + 0.5
}

// ByName returns the enchantment ID for a given name, or -1.
func ByName(name string) int32 {
	return enchdata.EnchantmentIDByName(name)
}

// Info returns the enchantment info for a given ID.
func Info(id int32) *enchdata.EnchantmentInfo {
	return enchdata.EnchantmentByID(id)
}
