package food

// Properties describes the nutritional value of a food item.
type Properties struct {
	Nutrition    int32
	Saturation   float32
	EatDuration  int32
	CanAlwaysEat bool
}

var registry = map[string]Properties{
	"minecraft:apple":             {Nutrition: 4, Saturation: 0.3, EatDuration: 32},
	"minecraft:baked_potato":      {Nutrition: 5, Saturation: 0.6, EatDuration: 32},
	"minecraft:beef":              {Nutrition: 3, Saturation: 0.3, EatDuration: 32},
	"minecraft:beetroot":          {Nutrition: 1, Saturation: 0.6, EatDuration: 32},
	"minecraft:beetroot_soup":     {Nutrition: 6, Saturation: 0.6, EatDuration: 32},
	"minecraft:bread":             {Nutrition: 5, Saturation: 0.6, EatDuration: 32},
	"minecraft:carrot":            {Nutrition: 3, Saturation: 0.6, EatDuration: 32},
	"minecraft:chicken":           {Nutrition: 2, Saturation: 0.3, EatDuration: 32},
	"minecraft:chorus_fruit":      {Nutrition: 4, Saturation: 0.3, EatDuration: 32, CanAlwaysEat: true},
	"minecraft:cod":               {Nutrition: 2, Saturation: 0.1, EatDuration: 32},
	"minecraft:cooked_beef":       {Nutrition: 8, Saturation: 0.8, EatDuration: 32},
	"minecraft:cooked_chicken":    {Nutrition: 6, Saturation: 0.6, EatDuration: 32},
	"minecraft:cooked_cod":        {Nutrition: 5, Saturation: 0.6, EatDuration: 32},
	"minecraft:cooked_mutton":     {Nutrition: 6, Saturation: 0.8, EatDuration: 32},
	"minecraft:cooked_porkchop":   {Nutrition: 8, Saturation: 0.8, EatDuration: 32},
	"minecraft:cooked_rabbit":     {Nutrition: 5, Saturation: 0.6, EatDuration: 32},
	"minecraft:cooked_salmon":     {Nutrition: 6, Saturation: 0.8, EatDuration: 32},
	"minecraft:cookie":            {Nutrition: 2, Saturation: 0.1, EatDuration: 32},
	"minecraft:dried_kelp":        {Nutrition: 1, Saturation: 0.3, EatDuration: 16},
	"minecraft:enchanted_golden_apple": {Nutrition: 4, Saturation: 1.2, EatDuration: 32, CanAlwaysEat: true},
	"minecraft:glow_berries":      {Nutrition: 2, Saturation: 0.1, EatDuration: 32},
	"minecraft:golden_apple":      {Nutrition: 4, Saturation: 1.2, EatDuration: 32, CanAlwaysEat: true},
	"minecraft:golden_carrot":     {Nutrition: 6, Saturation: 1.2, EatDuration: 32},
	"minecraft:honey_bottle":      {Nutrition: 6, Saturation: 0.1, EatDuration: 40},
	"minecraft:melon_slice":       {Nutrition: 2, Saturation: 0.3, EatDuration: 32},
	"minecraft:mushroom_stew":     {Nutrition: 6, Saturation: 0.6, EatDuration: 32},
	"minecraft:mutton":            {Nutrition: 2, Saturation: 0.3, EatDuration: 32},
	"minecraft:poisonous_potato":  {Nutrition: 2, Saturation: 0.3, EatDuration: 32},
	"minecraft:porkchop":          {Nutrition: 3, Saturation: 0.3, EatDuration: 32},
	"minecraft:potato":            {Nutrition: 1, Saturation: 0.3, EatDuration: 32},
	"minecraft:pufferfish":        {Nutrition: 1, Saturation: 0.1, EatDuration: 32},
	"minecraft:pumpkin_pie":       {Nutrition: 8, Saturation: 0.3, EatDuration: 32},
	"minecraft:rabbit":            {Nutrition: 3, Saturation: 0.3, EatDuration: 32},
	"minecraft:rabbit_stew":       {Nutrition: 10, Saturation: 0.6, EatDuration: 32},
	"minecraft:rotten_flesh":      {Nutrition: 4, Saturation: 0.1, EatDuration: 32},
	"minecraft:salmon":            {Nutrition: 2, Saturation: 0.1, EatDuration: 32},
	"minecraft:spider_eye":        {Nutrition: 2, Saturation: 0.8, EatDuration: 32},
	"minecraft:suspicious_stew":   {Nutrition: 6, Saturation: 0.6, EatDuration: 32},
	"minecraft:sweet_berries":     {Nutrition: 2, Saturation: 0.1, EatDuration: 32},
	"minecraft:tropical_fish":     {Nutrition: 1, Saturation: 0.1, EatDuration: 32},
}

// Get returns the food properties for a given item name, or nil if it is not food.
func Get(itemName string) *Properties {
	if p, ok := registry[itemName]; ok {
		return &p
	}
	return nil
}

// IsFood returns true if the item name corresponds to a food item.
func IsFood(itemName string) bool {
	_, ok := registry[itemName]
	return ok
}

// Eat computes the new food level and saturation after consuming food.
// Returns the new food level and new saturation.
func Eat(currentFood int32, currentSat float32, nutrition int32, satMod float32) (int32, float32) {
	newFood := currentFood + nutrition
	if newFood > 20 {
		newFood = 20
	}
	addedSat := float32(nutrition) * satMod * 2.0
	newSat := currentSat + addedSat
	if newSat > float32(newFood) {
		newSat = float32(newFood)
	}
	return newFood, newSat
}
