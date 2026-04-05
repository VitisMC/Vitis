package equipment

// ArmorProperties holds the defensive values for an armor item.
type ArmorProperties struct {
	Defense   float64
	Toughness float64
	Slot      int
}

const (
	SlotHelmet     = 0
	SlotChestplate = 1
	SlotLeggings   = 2
	SlotBoots      = 3
)

var armorRegistry = map[string]ArmorProperties{
	"minecraft:leather_helmet":      {Defense: 1, Toughness: 0, Slot: SlotHelmet},
	"minecraft:leather_chestplate":  {Defense: 3, Toughness: 0, Slot: SlotChestplate},
	"minecraft:leather_leggings":    {Defense: 2, Toughness: 0, Slot: SlotLeggings},
	"minecraft:leather_boots":       {Defense: 1, Toughness: 0, Slot: SlotBoots},
	"minecraft:chainmail_helmet":    {Defense: 2, Toughness: 0, Slot: SlotHelmet},
	"minecraft:chainmail_chestplate": {Defense: 5, Toughness: 0, Slot: SlotChestplate},
	"minecraft:chainmail_leggings":  {Defense: 4, Toughness: 0, Slot: SlotLeggings},
	"minecraft:chainmail_boots":     {Defense: 1, Toughness: 0, Slot: SlotBoots},
	"minecraft:iron_helmet":         {Defense: 2, Toughness: 0, Slot: SlotHelmet},
	"minecraft:iron_chestplate":     {Defense: 6, Toughness: 0, Slot: SlotChestplate},
	"minecraft:iron_leggings":       {Defense: 5, Toughness: 0, Slot: SlotLeggings},
	"minecraft:iron_boots":          {Defense: 2, Toughness: 0, Slot: SlotBoots},
	"minecraft:golden_helmet":       {Defense: 2, Toughness: 0, Slot: SlotHelmet},
	"minecraft:golden_chestplate":   {Defense: 5, Toughness: 0, Slot: SlotChestplate},
	"minecraft:golden_leggings":     {Defense: 3, Toughness: 0, Slot: SlotLeggings},
	"minecraft:golden_boots":        {Defense: 1, Toughness: 0, Slot: SlotBoots},
	"minecraft:diamond_helmet":      {Defense: 3, Toughness: 2, Slot: SlotHelmet},
	"minecraft:diamond_chestplate":  {Defense: 8, Toughness: 2, Slot: SlotChestplate},
	"minecraft:diamond_leggings":    {Defense: 6, Toughness: 2, Slot: SlotLeggings},
	"minecraft:diamond_boots":       {Defense: 3, Toughness: 2, Slot: SlotBoots},
	"minecraft:netherite_helmet":    {Defense: 3, Toughness: 3, Slot: SlotHelmet},
	"minecraft:netherite_chestplate": {Defense: 8, Toughness: 3, Slot: SlotChestplate},
	"minecraft:netherite_leggings":  {Defense: 6, Toughness: 3, Slot: SlotLeggings},
	"minecraft:netherite_boots":     {Defense: 3, Toughness: 3, Slot: SlotBoots},
	"minecraft:turtle_helmet":       {Defense: 2, Toughness: 0, Slot: SlotHelmet},
}

// GetArmor returns armor properties for an item name, or nil if not armor.
func GetArmor(itemName string) *ArmorProperties {
	if p, ok := armorRegistry[itemName]; ok {
		return &p
	}
	return nil
}

// IsArmor returns true if the item is an armor piece.
func IsArmor(itemName string) bool {
	_, ok := armorRegistry[itemName]
	return ok
}

// CalculateDamageReduction computes damage after armor reduction.
// Uses the vanilla Minecraft formula:
//
//	damage * (1 - min(20, max(defensePoints/5, defensePoints - 4*damage/(toughness+8))) / 25)
func CalculateDamageReduction(damage, defensePoints, toughness float64) float64 {
	if defensePoints <= 0 {
		return damage
	}
	a := defensePoints / 5.0
	b := defensePoints - 4.0*damage/(toughness+8.0)
	effective := a
	if b > effective {
		effective = b
	}
	if effective > 20 {
		effective = 20
	}
	return damage * (1.0 - effective/25.0)
}
