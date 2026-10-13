package crafting

import (
	"strings"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	enchantment "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
)

var dyeNames = []string{"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray", "light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black"}
var dyeRGB = []int32{16383998, 16351261, 13061821, 3847130, 16701501, 8439583, 15961002, 4673362, 10329495, 1481884, 8991416, 3949738, 8606770, 6192150, 11546150, 1908001}
var fireworkRGB = []int32{15790320, 15435844, 12801229, 6719955, 14602026, 4312372, 14188952, 4408131, 11250603, 2651799, 8073150, 2437522, 5320730, 3887386, 11743532, 1973019}

// specialOutput evaluates the built-in component-sensitive grid recipes.
func specialOutput(kind string, grid []inventory.ItemStack, width int) (inventory.ItemStack, []inventory.ItemStack, bool) {
	indices := make([]int, 0, len(grid))
	for i, s := range grid {
		if !s.Empty() {
			indices = append(indices, i)
		}
	}
	empty := inventory.ItemStack{}
	if len(indices) < 2 {
		return empty, nil, false
	}
	newStack := func(name string, count int32) inventory.ItemStack {
		return inventory.NewSlot(item.IDByName("minecraft:"+name), count)
	}
	switch kind {
	case "minecraft:crafting_special_repairitem":
		if len(indices) != 2 {
			return empty, nil, false
		}
		a, b := grid[indices[0]], grid[indices[1]]
		if !hasComponent(a, dc.ComponentDamage) || !hasComponent(b, dc.ComponentDamage) || !hasComponent(a, dc.ComponentMaxDamage) || !hasComponent(b, dc.ComponentMaxDamage) {
			return empty, nil, false
		}
		maxD := damageLimit(a)
		if a.ItemID != b.ItemID || a.ItemCount != 1 || b.ItemCount != 1 || maxD <= 0 || damageLimit(b) <= 0 {
			return empty, nil, false
		}
		maxD = max(maxD, damageLimit(b))
		out := inventory.NewSlot(a.ItemID, 1)
		out.SetComponent(dc.ComponentMaxDamage, inventory.VarIntComponent(maxD))
		out.SetDamage(max(int32(0), maxD-((maxD-a.Damage)+(maxD-b.Damage)+maxD*5/100)))
		for _, name := range []string{"minecraft:binding_curse", "minecraft:vanishing_curse"} {
			id := enchantment.EnchantmentIDByName(name)
			if level := max(a.EnchantLevel(id), b.EnchantLevel(id)); level > 0 {
				out.SetEnchant(id, level)
			}
		}
		return out, nil, true
	case "minecraft:crafting_special_armordye":
		var source inventory.ItemStack
		var colors []int32
		for _, i := range indices {
			s := grid[i]
			name := item.NameByID(s.ItemID)
			if strings.HasPrefix(name, "minecraft:leather_") || name == "minecraft:wolf_armor" {
				if !source.Empty() {
					return empty, nil, false
				}
				source = s
			} else if n := dyeIndex(name); n >= 0 {
				colors = append(colors, dyeRGB[n])
			} else {
				return empty, nil, false
			}
		}
		if source.Empty() || len(colors) == 0 {
			return empty, nil, false
		}
		show := true
		if v, ok := source.Component(dc.ComponentDyedColor); ok {
			show = v.(inventory.DyedColorComponent).ShowInTooltip
			colors = append([]int32{v.(inventory.DyedColorComponent).RGB}, colors...)
		}
		out := source.Clone()
		out.ItemCount = 1
		out.SetComponent(dc.ComponentDyedColor, inventory.DyedColorComponent{RGB: blendDyes(colors), ShowInTooltip: show})
		return out, nil, true
	case "minecraft:crafting_special_mapcloning":
		var source inventory.ItemStack
		var count int32
		for _, i := range indices {
			s := grid[i]
			switch {
			case hasComponent(s, dc.ComponentMapId):
				if !source.Empty() {
					return empty, nil, false
				}
				source = s
			case item.NameByID(s.ItemID) == "minecraft:map":
				count++
			default:
				return empty, nil, false
			}
		}
		if source.Empty() || count == 0 || !hasComponent(source, dc.ComponentMapId) {
			return empty, nil, false
		}
		out := source.Clone()
		out.ItemCount = count + 1
		return out, nil, true
	case "minecraft:crafting_special_mapextending":
		if width != 3 || len(indices) != 9 || item.NameByID(grid[4].ItemID) != "minecraft:filled_map" || !hasComponent(grid[4], dc.ComponentMapId) {
			return empty, nil, false
		}
		for i, s := range grid {
			if i != 4 && item.NameByID(s.ItemID) != "minecraft:paper" {
				return empty, nil, false
			}
		}
		out := grid[4].Clone()
		out.ItemCount = 1
		out.SetComponent(dc.ComponentMapPostProcessing, inventory.VarIntComponent(0))
		return out, nil, true
	case "minecraft:crafting_special_bookcloning":
		original := -1
		var count int32
		for _, i := range indices {
			switch item.NameByID(grid[i].ItemID) {
			case "minecraft:written_book":
				if original >= 0 {
					return empty, nil, false
				}
				original = i
			case "minecraft:writable_book":
				count++
			default:
				return empty, nil, false
			}
		}
		if original < 0 || count < 1 || !hasComponent(grid[original], dc.ComponentWrittenBookContent) {
			return empty, nil, false
		}
		source := grid[original]
		fields := componentFields(source, dc.ComponentWrittenBookContent)
		generation, _ := fields["generation"].(int32)
		if generation >= 2 {
			return empty, nil, false
		}
		fields["generation"] = generation + 1
		out := source.Clone()
		out.ItemCount = count
		out.SetComponent(dc.ComponentWrittenBookContent, inventory.StructuredComponent{Data: fields})
		rem := make([]inventory.ItemStack, len(grid))
		rem[original] = source.Clone()
		rem[original].ItemCount = 1
		return out, rem, true
	case "minecraft:crafting_special_bannerduplicate":
		if len(indices) != 2 {
			return empty, nil, false
		}
		a, b := grid[indices[0]], grid[indices[1]]
		name := item.NameByID(a.ItemID)
		if a.ItemID != b.ItemID || !strings.HasSuffix(name, "_banner") {
			return empty, nil, false
		}
		n, m := bannerLayers(a), bannerLayers(b)
		if len(n) == 0 {
			a, b = b, a
			n, m = m, n
		}
		if len(n) == 0 || len(n) > 6 || len(m) > 0 {
			return empty, nil, false
		}
		out := a.Clone()
		out.ItemCount = 1
		rem := make([]inventory.ItemStack, len(grid))
		i := indices[0]
		if len(bannerLayers(grid[i])) == 0 {
			i = indices[1]
		}
		rem[i] = out.Clone()
		return out, rem, true
	case "minecraft:crafting_special_shielddecoration":
		if len(indices) != 2 {
			return empty, nil, false
		}
		var shield, banner inventory.ItemStack
		for _, i := range indices {
			s := grid[i]
			name := item.NameByID(s.ItemID)
			if name == "minecraft:shield" {
				shield = s
			} else if strings.HasSuffix(name, "_banner") {
				banner = s
			} else {
				return empty, nil, false
			}
		}
		if shield.Empty() || banner.Empty() || hasComponent(shield, dc.ComponentBaseColor) || len(bannerLayers(shield)) > 0 {
			return empty, nil, false
		}
		out := shield.Clone()
		out.ItemCount = 1
		copyComponent(&out, banner, dc.ComponentBannerPatterns)
		color := strings.TrimSuffix(strings.TrimPrefix(item.NameByID(banner.ItemID), "minecraft:"), "_banner")
		for i, name := range dyeNames {
			if name == color {
				out.SetComponent(dc.ComponentBaseColor, inventory.VarIntComponent(i))
			}
		}
		return out, nil, true
	case "minecraft:crafting_special_tippedarrow":
		if width != 3 || len(indices) != 9 || item.NameByID(grid[4].ItemID) != "minecraft:lingering_potion" {
			return empty, nil, false
		}
		for i, s := range grid {
			if i != 4 && item.NameByID(s.ItemID) != "minecraft:arrow" {
				return empty, nil, false
			}
		}
		out := newStack("tipped_arrow", 8)
		copyComponent(&out, grid[4], dc.ComponentPotionContents)
		return out, nil, true
	case "minecraft:crafting_decorated_pot":
		if width != 3 || len(indices) != 4 {
			return empty, nil, false
		}
		decorations := make(inventory.PotDecorationsComponent, 4)
		for i, pos := range []int{1, 3, 5, 7} {
			name := item.NameByID(grid[pos].ItemID)
			if grid[pos].Empty() || (name != "minecraft:brick" && !strings.HasSuffix(name, "_pottery_sherd")) {
				return empty, nil, false
			}
			decorations[i] = grid[pos].ItemID
		}
		out := newStack("decorated_pot", 1)
		out.SetComponent(dc.ComponentPotDecorations, decorations)
		return out, nil, true
	case "minecraft:crafting_special_firework_rocket":
		paper, powder := 0, 0
		var explosions []inventory.FireworkExplosionComponent
		for _, i := range indices {
			s := grid[i]
			switch item.NameByID(s.ItemID) {
			case "minecraft:paper":
				paper++
			case "minecraft:gunpowder":
				powder++
			case "minecraft:firework_star":
				v, ok := s.Component(dc.ComponentFireworkExplosion)
				if !ok {
					continue
				}
				explosions = append(explosions, v.(inventory.FireworkExplosionComponent))
			default:
				return empty, nil, false
			}
		}
		if paper != 1 || powder < 1 || powder > 3 {
			return empty, nil, false
		}
		out := newStack("firework_rocket", 3)
		out.SetComponent(dc.ComponentFireworks, inventory.FireworksComponent{FlightDuration: int32(powder), Explosions: explosions})
		return out, nil, true
	case "minecraft:crafting_special_firework_star_fade":
		var source inventory.ItemStack
		var colors []int32
		for _, i := range indices {
			s := grid[i]
			name := item.NameByID(s.ItemID)
			if name == "minecraft:firework_star" {
				if !source.Empty() {
					return empty, nil, false
				}
				source = s
			} else if n := dyeIndex(name); n >= 0 {
				colors = append(colors, fireworkRGB[n])
			} else {
				return empty, nil, false
			}
		}
		if source.Empty() || len(colors) == 0 {
			return empty, nil, false
		}
		v, ok := source.Component(dc.ComponentFireworkExplosion)
		if !ok {
			return empty, nil, false
		}
		explosion := v.(inventory.FireworkExplosionComponent)
		explosion.FadeColors = colors
		out := source.Clone()
		out.ItemCount = 1
		out.SetComponent(dc.ComponentFireworkExplosion, explosion)
		return out, nil, true
	case "minecraft:crafting_special_firework_star":
		var explosion inventory.FireworkExplosionComponent
		powder, shapes, trail, twinkle := 0, 0, 0, 0
		for _, i := range indices {
			name := item.NameByID(grid[i].ItemID)
			if n := dyeIndex(name); n >= 0 {
				explosion.Colors = append(explosion.Colors, fireworkRGB[n])
				continue
			}
			switch name {
			case "minecraft:gunpowder":
				powder++
			case "minecraft:diamond":
				trail++
				explosion.Trail = true
			case "minecraft:glowstone_dust":
				twinkle++
				explosion.Twinkle = true
			case "minecraft:fire_charge":
				shapes++
				explosion.Shape = 1
			case "minecraft:gold_nugget":
				shapes++
				explosion.Shape = 2
			case "minecraft:feather":
				shapes++
				explosion.Shape = 4
			case "minecraft:creeper_head", "minecraft:player_head", "minecraft:zombie_head", "minecraft:skeleton_skull", "minecraft:wither_skeleton_skull", "minecraft:dragon_head", "minecraft:piglin_head":
				shapes++
				explosion.Shape = 3
			default:
				return empty, nil, false
			}
		}
		if powder != 1 || shapes > 1 || trail > 1 || twinkle > 1 || len(explosion.Colors) == 0 {
			return empty, nil, false
		}
		out := newStack("firework_star", 1)
		out.SetComponent(dc.ComponentFireworkExplosion, explosion)
		return out, nil, true
	}
	return empty, nil, false
}

// dyeIndex resolves a vanilla dye item without inspecting translated names.
func dyeIndex(name string) int {
	for i, color := range dyeNames {
		if name == "minecraft:"+color+"_dye" {
			return i
		}
	}
	return -1
}

// bannerLayers returns the independent pattern list carried by a banner or shield.
func bannerLayers(s inventory.ItemStack) []any {
	layers, _ := componentFields(s, dc.ComponentBannerPatterns)["layers"].([]any)
	return layers
}

// blendDyes preserves average brightness when mixing leather colors.
func blendDyes(colors []int32) int32 {
	var r, g, b, brightness int64
	for _, color := range colors {
		x, y, z := int64((color>>16)&255), int64((color>>8)&255), int64(color&255)
		r += x
		g += y
		b += z
		brightness += max(x, y, z)
	}
	n := int64(len(colors))
	r /= n
	g /= n
	b /= n
	highest := max(r, g, b)
	if highest == 0 {
		return 0
	}
	average := float32(brightness) / float32(n)
	return int32(int32(float32(r)*average/float32(highest))<<16 | int32(float32(g)*average/float32(highest))<<8 | int32(float32(b)*average/float32(highest)))
}
