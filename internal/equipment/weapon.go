package equipment

// WeaponProperties holds the attack values for a weapon.
type WeaponProperties struct {
	AttackDamage float64
	AttackSpeed  float64
}

var weaponRegistry = map[string]WeaponProperties{
	"minecraft:wooden_sword":    {AttackDamage: 4, AttackSpeed: 1.6},
	"minecraft:stone_sword":     {AttackDamage: 5, AttackSpeed: 1.6},
	"minecraft:iron_sword":      {AttackDamage: 6, AttackSpeed: 1.6},
	"minecraft:golden_sword":    {AttackDamage: 4, AttackSpeed: 1.6},
	"minecraft:diamond_sword":   {AttackDamage: 7, AttackSpeed: 1.6},
	"minecraft:netherite_sword": {AttackDamage: 8, AttackSpeed: 1.6},

	"minecraft:wooden_axe":    {AttackDamage: 7, AttackSpeed: 0.8},
	"minecraft:stone_axe":     {AttackDamage: 9, AttackSpeed: 0.8},
	"minecraft:iron_axe":      {AttackDamage: 9, AttackSpeed: 0.9},
	"minecraft:golden_axe":    {AttackDamage: 7, AttackSpeed: 1.0},
	"minecraft:diamond_axe":   {AttackDamage: 9, AttackSpeed: 1.0},
	"minecraft:netherite_axe": {AttackDamage: 10, AttackSpeed: 1.0},

	"minecraft:wooden_pickaxe":    {AttackDamage: 2, AttackSpeed: 1.2},
	"minecraft:stone_pickaxe":     {AttackDamage: 3, AttackSpeed: 1.2},
	"minecraft:iron_pickaxe":      {AttackDamage: 4, AttackSpeed: 1.2},
	"minecraft:golden_pickaxe":    {AttackDamage: 2, AttackSpeed: 1.2},
	"minecraft:diamond_pickaxe":   {AttackDamage: 5, AttackSpeed: 1.2},
	"minecraft:netherite_pickaxe": {AttackDamage: 6, AttackSpeed: 1.2},

	"minecraft:wooden_shovel":    {AttackDamage: 2.5, AttackSpeed: 1.0},
	"minecraft:stone_shovel":     {AttackDamage: 3.5, AttackSpeed: 1.0},
	"minecraft:iron_shovel":      {AttackDamage: 4.5, AttackSpeed: 1.0},
	"minecraft:golden_shovel":    {AttackDamage: 2.5, AttackSpeed: 1.0},
	"minecraft:diamond_shovel":   {AttackDamage: 5.5, AttackSpeed: 1.0},
	"minecraft:netherite_shovel": {AttackDamage: 6.5, AttackSpeed: 1.0},

	"minecraft:wooden_hoe":    {AttackDamage: 1, AttackSpeed: 1.0},
	"minecraft:stone_hoe":     {AttackDamage: 1, AttackSpeed: 2.0},
	"minecraft:iron_hoe":      {AttackDamage: 1, AttackSpeed: 3.0},
	"minecraft:golden_hoe":    {AttackDamage: 1, AttackSpeed: 1.0},
	"minecraft:diamond_hoe":   {AttackDamage: 1, AttackSpeed: 4.0},
	"minecraft:netherite_hoe": {AttackDamage: 1, AttackSpeed: 4.0},

	"minecraft:trident":  {AttackDamage: 9, AttackSpeed: 1.1},
	"minecraft:mace":     {AttackDamage: 5, AttackSpeed: 1.6},
}

// GetWeapon returns weapon properties for an item name, or nil if not a weapon.
func GetWeapon(itemName string) *WeaponProperties {
	if p, ok := weaponRegistry[itemName]; ok {
		return &p
	}
	return nil
}

// IsWeapon returns true if the item is a weapon.
func IsWeapon(itemName string) bool {
	_, ok := weaponRegistry[itemName]
	return ok
}
