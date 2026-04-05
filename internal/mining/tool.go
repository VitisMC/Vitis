package mining

// ToolType classifies the kind of tool an item represents.
type ToolType uint8

const (
	ToolNone    ToolType = iota
	ToolPickaxe
	ToolAxe
	ToolShovel
	ToolHoe
	ToolSword
	ToolShears
)

// ToolTier represents the material tier of a tool.
type ToolTier uint8

const (
	TierNone      ToolTier = iota
	TierWood
	TierStone
	TierIron
	TierDiamond
	TierGold
	TierNetherite
)

// ToolInfo describes the mining properties of a tool item.
type ToolInfo struct {
	Type  ToolType
	Tier  ToolTier
	Speed float64
}

var toolRegistry = map[string]ToolInfo{}

func init() {
	registerTools("wooden", TierWood, 2.0)
	registerTools("stone", TierStone, 4.0)
	registerTools("iron", TierIron, 6.0)
	registerTools("golden", TierGold, 12.0)
	registerTools("diamond", TierDiamond, 8.0)
	registerTools("netherite", TierNetherite, 9.0)

	toolRegistry["minecraft:shears"] = ToolInfo{Type: ToolShears, Tier: TierNone, Speed: 2.0}
}

func registerTools(prefix string, tier ToolTier, speed float64) {
	toolRegistry["minecraft:"+prefix+"_pickaxe"] = ToolInfo{Type: ToolPickaxe, Tier: tier, Speed: speed}
	toolRegistry["minecraft:"+prefix+"_axe"] = ToolInfo{Type: ToolAxe, Tier: tier, Speed: speed}
	toolRegistry["minecraft:"+prefix+"_shovel"] = ToolInfo{Type: ToolShovel, Tier: tier, Speed: speed}
	toolRegistry["minecraft:"+prefix+"_hoe"] = ToolInfo{Type: ToolHoe, Tier: tier, Speed: speed}
	toolRegistry["minecraft:"+prefix+"_sword"] = ToolInfo{Type: ToolSword, Tier: tier, Speed: speed}
}

// GetToolInfo returns the tool info for the given item name.
// Returns a zero ToolInfo if the item is not a tool.
func GetToolInfo(itemName string) ToolInfo {
	if info, ok := toolRegistry[itemName]; ok {
		return info
	}
	return ToolInfo{}
}

// TierLevel returns the harvest level for a tool tier.
// Wood/Gold=0, Stone=1, Iron=2, Diamond/Netherite=3.
func TierLevel(tier ToolTier) int {
	switch tier {
	case TierWood, TierGold:
		return 0
	case TierStone:
		return 1
	case TierIron:
		return 2
	case TierDiamond, TierNetherite:
		return 3
	default:
		return -1
	}
}
