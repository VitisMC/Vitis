package mining

// HarvestRule describes which tool is effective for a block and whether a minimum tier is required.
type HarvestRule struct {
	EffectiveTool ToolType
	MinTier       ToolTier
	RequiresTool  bool
}

var harvestRules = map[string]HarvestRule{}

func init() {
	pickReq := func(tier ToolTier) HarvestRule {
		return HarvestRule{EffectiveTool: ToolPickaxe, MinTier: tier, RequiresTool: true}
	}
	pick := func() HarvestRule {
		return HarvestRule{EffectiveTool: ToolPickaxe, MinTier: TierWood, RequiresTool: false}
	}
	axe := func() HarvestRule {
		return HarvestRule{EffectiveTool: ToolAxe, RequiresTool: false}
	}
	shovel := func() HarvestRule {
		return HarvestRule{EffectiveTool: ToolShovel, RequiresTool: false}
	}
	hoe := func() HarvestRule {
		return HarvestRule{EffectiveTool: ToolHoe, RequiresTool: false}
	}

	stoneBlocks := []string{
		"stone", "granite", "polished_granite", "diorite", "polished_diorite",
		"andesite", "polished_andesite", "cobblestone", "mossy_cobblestone",
		"deepslate", "cobbled_deepslate", "polished_deepslate",
		"calcite", "tuff", "dripstone_block",
		"netherrack", "basalt", "polished_basalt", "smooth_basalt",
		"blackstone", "polished_blackstone", "end_stone",
		"sandstone", "red_sandstone", "smooth_sandstone", "smooth_red_sandstone",
		"chiseled_sandstone", "chiseled_red_sandstone", "cut_sandstone", "cut_red_sandstone",
		"prismarine", "prismarine_bricks", "dark_prismarine",
		"purpur_block", "purpur_pillar",
		"stone_bricks", "mossy_stone_bricks", "cracked_stone_bricks", "chiseled_stone_bricks",
		"bricks", "nether_bricks", "red_nether_bricks",
		"terracotta",
		"gravel",
	}
	for _, name := range stoneBlocks {
		harvestRules["minecraft:"+name] = pick()
	}

	oreBlocksWood := []string{
		"coal_ore", "deepslate_coal_ore", "nether_gold_ore",
	}
	for _, name := range oreBlocksWood {
		harvestRules["minecraft:"+name] = pickReq(TierWood)
	}

	oreBlocksStone := []string{
		"iron_ore", "deepslate_iron_ore", "copper_ore", "deepslate_copper_ore",
		"lapis_ore", "deepslate_lapis_ore",
	}
	for _, name := range oreBlocksStone {
		harvestRules["minecraft:"+name] = pickReq(TierStone)
	}

	oreBlocksIron := []string{
		"gold_ore", "deepslate_gold_ore", "redstone_ore", "deepslate_redstone_ore",
		"diamond_ore", "deepslate_diamond_ore", "emerald_ore", "deepslate_emerald_ore",
	}
	for _, name := range oreBlocksIron {
		harvestRules["minecraft:"+name] = pickReq(TierIron)
	}

	harvestRules["minecraft:obsidian"] = pickReq(TierDiamond)
	harvestRules["minecraft:crying_obsidian"] = pickReq(TierDiamond)
	harvestRules["minecraft:ancient_debris"] = pickReq(TierDiamond)
	harvestRules["minecraft:respawn_anchor"] = pickReq(TierDiamond)

	mineralBlocks := []string{
		"iron_block", "gold_block", "diamond_block", "emerald_block",
		"lapis_block", "redstone_block", "copper_block",
		"raw_iron_block", "raw_gold_block", "raw_copper_block",
		"coal_block", "netherite_block",
	}
	for _, name := range mineralBlocks {
		harvestRules["minecraft:"+name] = pickReq(TierStone)
	}
	harvestRules["minecraft:netherite_block"] = pickReq(TierDiamond)

	woodBlocks := []string{
		"oak_planks", "spruce_planks", "birch_planks", "jungle_planks",
		"acacia_planks", "cherry_planks", "dark_oak_planks", "pale_oak_planks",
		"mangrove_planks", "bamboo_planks", "crimson_planks", "warped_planks",
		"oak_log", "spruce_log", "birch_log", "jungle_log",
		"acacia_log", "cherry_log", "dark_oak_log", "pale_oak_log", "mangrove_log",
		"oak_wood", "spruce_wood", "birch_wood", "jungle_wood",
		"acacia_wood", "cherry_wood", "dark_oak_wood", "pale_oak_wood",
		"stripped_oak_log", "stripped_spruce_log", "stripped_birch_log", "stripped_jungle_log",
		"stripped_acacia_log", "stripped_cherry_log", "stripped_dark_oak_log",
		"crafting_table", "chest", "trapped_chest", "barrel",
		"bookshelf", "lectern", "composter",
		"oak_fence", "spruce_fence", "birch_fence", "jungle_fence",
		"oak_fence_gate", "spruce_fence_gate", "birch_fence_gate",
		"oak_door", "spruce_door", "birch_door", "jungle_door",
		"oak_trapdoor", "spruce_trapdoor", "birch_trapdoor",
		"oak_stairs", "spruce_stairs", "birch_stairs", "jungle_stairs",
		"oak_slab", "spruce_slab", "birch_slab", "jungle_slab",
		"oak_pressure_plate", "spruce_pressure_plate", "birch_pressure_plate",
		"oak_button", "spruce_button", "birch_button",
		"oak_sign", "spruce_sign", "birch_sign",
		"note_block", "jukebox",
		"crimson_stem", "warped_stem", "crimson_hyphae", "warped_hyphae",
		"bamboo_block", "bamboo_mosaic",
	}
	for _, name := range woodBlocks {
		harvestRules["minecraft:"+name] = axe()
	}

	soilBlocks := []string{
		"dirt", "coarse_dirt", "podzol", "rooted_dirt", "mud",
		"grass_block", "mycelium",
		"sand", "red_sand", "gravel",
		"clay", "soul_sand", "soul_soil",
		"snow", "snow_block",
		"farmland", "dirt_path",
		"concrete_powder",
	}
	for _, name := range soilBlocks {
		harvestRules["minecraft:"+name] = shovel()
	}

	cropBlocks := []string{
		"wheat", "carrots", "potatoes", "beetroots",
		"melon", "pumpkin", "hay_block",
		"nether_wart_block", "warped_wart_block",
		"shroomlight", "dried_kelp_block",
		"moss_block", "moss_carpet",
		"sculk", "sculk_catalyst", "sculk_sensor", "sculk_shrieker", "sculk_vein",
	}
	for _, name := range cropBlocks {
		harvestRules["minecraft:"+name] = hoe()
	}

	harvestRules["minecraft:furnace"] = pickReq(TierWood)
	harvestRules["minecraft:blast_furnace"] = pickReq(TierWood)
	harvestRules["minecraft:smoker"] = pickReq(TierWood)
	harvestRules["minecraft:anvil"] = pickReq(TierWood)
	harvestRules["minecraft:chipped_anvil"] = pickReq(TierWood)
	harvestRules["minecraft:damaged_anvil"] = pickReq(TierWood)
	harvestRules["minecraft:brewing_stand"] = pickReq(TierWood)
	harvestRules["minecraft:cauldron"] = pickReq(TierWood)
	harvestRules["minecraft:hopper"] = pickReq(TierWood)
	harvestRules["minecraft:iron_bars"] = pickReq(TierWood)
	harvestRules["minecraft:iron_door"] = pickReq(TierWood)
	harvestRules["minecraft:iron_trapdoor"] = pickReq(TierWood)
	harvestRules["minecraft:chain"] = pickReq(TierWood)
	harvestRules["minecraft:lantern"] = pickReq(TierWood)
	harvestRules["minecraft:soul_lantern"] = pickReq(TierWood)

	harvestRules["minecraft:bedrock"] = HarvestRule{RequiresTool: true, MinTier: TierNetherite + 1}
}

// GetHarvestRule returns the harvest rule for a block name.
// If no specific rule exists, returns a zero-value rule (any tool works).
func GetHarvestRule(blockName string) HarvestRule {
	if rule, ok := harvestRules[blockName]; ok {
		return rule
	}
	return HarvestRule{}
}
