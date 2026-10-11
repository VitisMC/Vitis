package level

import (
	"fmt"
	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/crafting"
	"os"
	"path/filepath"
	"slices"
	"strings"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	enchantmentdata "github.com/vitismc/vitis/internal/data/generated/enchantment"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/storage"
	"maps"
)

// InventorySlot represents a single item in the player's inventory for persistence.
type InventorySlot struct {
	Stack        inventory.ItemStack
	Slot         int8
	ItemName     string
	Count        int32
	Damage       int32
	Enchantments map[int32]int32
	raw          *nbt.Compound
}

// PlayerData holds persistent player state loaded from/saved to playerdata NBT files.
type PlayerData struct {
	DataVersion            int32
	UUID                   string
	PosX                   float64
	PosY                   float64
	PosZ                   float64
	Yaw                    float32
	Pitch                  float32
	OnGround               bool
	GameMode               int32
	Health                 float32
	FoodLevel              int32
	FoodSat                float32
	FoodExhaustion         float32
	Dimension              string
	PortalCooldown         int32
	SelectedItemSlot       int32
	XPLevel                int32
	XPTotal                int32
	XPBar                  float32
	Effects                []effect.Instance
	SpawnX, SpawnY, SpawnZ int32
	HasSpawnPoint          bool
	SpawnDimension         string
	SpawnAngle             float32
	SpawnForced            bool
	Inventory              []InventorySlot
	RecipeBook             crafting.BookState
	RecipeProgress         advancement.RecipeProgress
	parseError             error
	raw                    *nbt.Compound
}

// DefaultPlayerData returns a new player data with sensible defaults.
func DefaultPlayerData(uuid string, spawnX, spawnY, spawnZ float64, gameMode int32) *PlayerData {
	return &PlayerData{
		DataVersion: protocol.CurrentDataVersion,
		UUID:        uuid,
		PosX:        spawnX,
		PosY:        spawnY,
		PosZ:        spawnZ,
		GameMode:    gameMode,
		Health:      20.0,
		FoodLevel:   20,
		FoodSat:     5.0,
		Dimension:   "minecraft:overworld",
	}
}

// LoadPlayerData reads player data from a gzipped NBT file in the world directory.
func LoadPlayerData(worldDir, uuid string) (*PlayerData, error) {
	path := playerDataPath(worldDir, uuid)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("playerdata: open %s: %w", path, err)
	}
	defer f.Close()

	data, err := readGzipNBT(f)
	if err != nil {
		return nil, fmt.Errorf("playerdata: read: %w", err)
	}

	dec := nbt.NewDecoder(data)
	_, root, err := dec.ReadNamedRootCompound()
	if err != nil {
		return nil, fmt.Errorf("playerdata: decode: %w", err)
	}

	pd := &PlayerData{UUID: uuid, raw: root.Clone()}
	pd.parseCompound(root)
	if pd.parseError != nil {
		return nil, pd.parseError
	}
	if pd.DataVersion != protocol.CurrentDataVersion {
		return nil, fmt.Errorf("playerdata: unsupported DataVersion %d", pd.DataVersion)
	}
	return pd, nil
}

func (pd *PlayerData) parseCompound(c *nbt.Compound) {
	pd.parseRecipeBook(c)
	if v, ok := c.GetInt("DataVersion"); ok {
		pd.DataVersion = v
	}
	if v, ok := c.GetInt("playerGameType"); ok {
		pd.GameMode = v
	}
	if v, ok := c.GetFloat("Health"); ok {
		pd.Health = v
	}
	if v, ok := c.GetInt("foodLevel"); ok {
		pd.FoodLevel = v
	}
	if v, ok := c.GetFloat("foodSaturationLevel"); ok {
		pd.FoodSat = v
	}
	if v, ok := c.GetFloat("foodExhaustionLevel"); ok {
		pd.FoodExhaustion = v
	}
	if _, _, exists := c.Get("Dimension"); exists {
		value, ok := c.GetString("Dimension")
		if !ok || value == "" {
			pd.parseError = fmt.Errorf("invalid player dimension")
			return
		}
		pd.Dimension = value
	}
	if _, _, exists := c.Get("PortalCooldown"); exists {
		value, ok := c.GetInt("PortalCooldown")
		if !ok || value < 0 {
			pd.parseError = fmt.Errorf("invalid player portal cooldown")
			return
		}
		pd.PortalCooldown = value
	}
	if _, _, exists := c.Get("SelectedItemSlot"); exists {
		value, ok := c.GetInt("SelectedItemSlot")
		if !ok || value < 0 || value > 8 {
			pd.parseError = fmt.Errorf("invalid player selected item slot")
			return
		}
		pd.SelectedItemSlot = value
	}
	if v, ok := c.GetBool("OnGround"); ok {
		pd.OnGround = v
	}
	if v, ok := c.GetInt("XpLevel"); ok {
		pd.XPLevel = v
	}
	if v, ok := c.GetInt("XpTotal"); ok {
		pd.XPTotal = v
	}
	if v, ok := c.GetFloat("XpP"); ok {
		pd.XPBar = v
	}
	pd.SpawnDimension, _ = c.GetString("SpawnDimension")
	if pd.SpawnDimension == "" {
		pd.SpawnDimension = "minecraft:overworld"
	}
	pd.SpawnAngle, _ = c.GetFloat("SpawnAngle")
	pd.SpawnForced, _ = c.GetBool("SpawnForced")
	if x, ok := c.GetInt("SpawnX"); ok {
		if y, ok := c.GetInt("SpawnY"); ok {
			if z, ok := c.GetInt("SpawnZ"); ok {
				pd.SpawnX, pd.SpawnY, pd.SpawnZ = x, y, z
				pd.HasSpawnPoint = true
			}
		}
	}
	if active, ok := c.GetList("active_effects"); ok {
		for _, value := range active.Elements() {
			entry, ok := value.(*nbt.Compound)
			if !ok {
				continue
			}
			name, ok := entry.GetString("id")
			if !ok {
				continue
			}
			id := effectID(name)
			duration, valid := entry.GetInt("duration")
			if id < 0 || !valid || duration <= 0 {
				continue
			}
			amplifier := int32(0)
			if value, ok := entry.GetByte("amplifier"); ok {
				amplifier = int32(uint8(value))
			} else if value, ok := entry.GetInt("amplifier"); ok {
				amplifier = value
			}
			if amplifier < 0 || amplifier > 255 {
				continue
			}
			inst := effect.Instance{ID: id, Amplifier: amplifier, Duration: duration, Flags: effect.DefaultFlags()}
			if value, ok := entry.GetBool("ambient"); ok && value {
				inst.Flags |= effect.FlagAmbient
			}
			if value, ok := entry.GetBool("show_particles"); ok && !value {
				inst.Flags &^= effect.FlagParticles
			}
			if value, ok := entry.GetBool("show_icon"); ok && !value {
				inst.Flags &^= effect.FlagIcon
			}
			pd.Effects = append(pd.Effects, inst)
		}
	}

	if posList, ok := c.GetList("Pos"); ok && posList.Len() >= 3 {
		elems := posList.Elements()
		if x, ok := elems[0].(float64); ok {
			pd.PosX = x
		}
		if y, ok := elems[1].(float64); ok {
			pd.PosY = y
		}
		if z, ok := elems[2].(float64); ok {
			pd.PosZ = z
		}
	}

	if rotList, ok := c.GetList("Rotation"); ok && rotList.Len() >= 2 {
		elems := rotList.Elements()
		if yaw, ok := elems[0].(float32); ok {
			pd.Yaw = yaw
		}
		if pitch, ok := elems[1].(float32); ok {
			pd.Pitch = pitch
		}
	}

	if _, tag, present := c.Get("Inventory"); present && tag != nbt.TagList {
		pd.parseError = fmt.Errorf("invalid player inventory type")
		return
	}
	if invList, ok := c.GetList("Inventory"); ok {
		if invList.Len() > 41 {
			pd.parseError = fmt.Errorf("player inventory exceeds slot limit")
			return
		}
		seenSlots := make(map[int]bool)
		for _, elem := range invList.Elements() {
			slotComp, ok := elem.(*nbt.Compound)
			if !ok {
				pd.parseError = fmt.Errorf("invalid player inventory entry")
				return
			}
			is := InventorySlot{raw: slotComp.Clone()}
			v, hasSlot := slotComp.GetByte("Slot")
			windowSlot, valid := NBTSlotToWindow(v)
			if !hasSlot || !valid || seenSlots[windowSlot] {
				pd.parseError = fmt.Errorf("invalid or duplicate player inventory slot %d", v)
				return
			}
			is.Slot = v
			seenSlots[windowSlot] = true
			if v, ok := slotComp.GetString("id"); ok {
				is.ItemName = v
			}
			is.Count = 1 // The persistent item codec defaults an omitted count to one.
			if v, ok := slotComp.GetInt("count"); ok {
				is.Count = v
			} else if v, ok := slotComp.GetByte("Count"); ok {
				is.Count = int32(v)
			}
			modernDamage := false
			modernEnchantments := false
			if components, ok := slotComp.GetCompound("components"); ok {
				if v, exists := components.GetInt("minecraft:damage"); exists {
					is.Damage = v
					modernDamage = true
				}
				if enchantments, exists := components.GetCompound("minecraft:enchantments"); exists {
					if levels, exists := enchantments.GetCompound("levels"); exists {
						modernEnchantments = true
						is.Enchantments = make(map[int32]int32)
						for name, level := range levels.GetAllInts() {
							if id := enchantmentdata.EnchantmentIDByName(name); id >= 0 && level > 0 {
								is.Enchantments[id] = level
							}
						}
					}
				}
			}
			if !modernDamage {
				if v, ok := slotComp.GetInt("Damage"); ok {
					is.Damage = v
				}
			}
			if enchList, ok := slotComp.GetList("Enchantments"); ok && !modernEnchantments {
				is.Enchantments = make(map[int32]int32)
				for _, ee := range enchList.Elements() {
					ec, ok := ee.(*nbt.Compound)
					if !ok {
						continue
					}
					eid, _ := ec.GetInt("id")
					elvl, _ := ec.GetInt("lvl")
					if eid > 0 || elvl > 0 {
						is.Enchantments[eid] = elvl
					}
				}
			}
			if is.ItemName == "" || is.Count <= 0 {
				pd.parseError = fmt.Errorf("invalid player inventory item in slot %d", is.Slot)
				return
			}
			if is.ItemName != "" && is.Count > 0 {
				decoded, err := inventory.ItemStackFromNBT(slotComp)
				if err != nil {
					pd.parseError = fmt.Errorf("player inventory slot %d: %w", is.Slot, err)
					return
				}
				is.Stack = decoded
				pd.Inventory = append(pd.Inventory, is)
			}
		}
	}
}

// effectName maps a status-effect registry ID to its persistent identifier.
func effectName(id int32) string {
	if id < 0 || int(id) >= len(effectdata.Effects) {
		return ""
	}
	name := effectdata.Effects[id].Name
	var result strings.Builder
	result.WriteString("minecraft:")
	for i, r := range name {
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

// effectID maps a persistent identifier to a status-effect registry ID.
func effectID(name string) int32 {
	for _, info := range effectdata.Effects {
		if effectName(info.ID) == name {
			return info.ID
		}
	}
	return -1
}

// writePlayerEffects merges active effects while retaining unsupported NBT entries.
func writePlayerEffects(root, original *nbt.Compound, effects []effect.Instance) {
	active := nbt.NewList(nbt.TagCompound)
	knownRaw := make(map[string]*nbt.Compound)
	if original != nil {
		if old, ok := original.GetList("active_effects"); ok {
			for _, value := range old.Elements() {
				entry, ok := value.(*nbt.Compound)
				if !ok {
					continue
				}
				name, ok := entry.GetString("id")
				if !ok || effectID(name) < 0 {
					active.Add(entry.Clone())
				} else {
					knownRaw[name] = entry
				}
			}
		}
	}
	for _, inst := range effects {
		name := effectName(inst.ID)
		if name == "" || inst.Duration <= 0 || inst.Amplifier < 0 || inst.Amplifier > 255 {
			continue
		}
		entry := knownRaw[name].Clone()
		if entry == nil {
			entry = nbt.NewCompound()
		}
		entry.PutString("id", name).PutByte("amplifier", int8(inst.Amplifier)).PutInt("duration", inst.Duration)
		entry.PutBool("ambient", inst.Flags&effect.FlagAmbient != 0)
		entry.PutBool("show_particles", inst.Flags&effect.FlagParticles != 0)
		entry.PutBool("show_icon", inst.Flags&effect.FlagIcon != 0)
		active.Add(entry)
	}
	if active.Len() > 0 {
		root.PutList("active_effects", active)
	} else {
		root.Delete("active_effects")
	}
}

// ReplaceInventoryPreservingRaw retains unknown item tags for unchanged slot items.
func (pd *PlayerData) ReplaceInventoryPreservingRaw(current []InventorySlot) {
	if pd == nil {
		return
	}
	previous := make(map[int]InventorySlot, len(pd.Inventory))
	for _, slot := range pd.Inventory {
		if windowSlot, ok := NBTSlotToWindow(slot.Slot); ok {
			previous[windowSlot] = slot
		}
	}
	for i := range current {
		if !current[i].Stack.Empty() {
			continue
		}
		windowSlot, valid := NBTSlotToWindow(current[i].Slot)
		if old, ok := previous[windowSlot]; valid && ok && old.ItemName == current[i].ItemName {
			current[i].raw = old.raw.Clone()
		}
	}
	pd.Inventory = current
}

// NBTSlotToWindow maps persistent player inventory indices to protocol slots.
func NBTSlotToWindow(slot int8) (int, bool) {
	switch {
	case slot >= 0 && slot <= 8:
		return int(slot) + 36, true
	case slot >= 9 && slot <= 35:
		return int(slot), true
	case slot >= 100 && slot <= 103:
		return 108 - int(slot), true
	case slot == -106:
		return 45, true
	case slot >= 36 && slot <= 45:
		return int(slot), true // Legacy Vitis playerdata.
	default:
		return 0, false
	}
}

// WindowSlotToNBT maps protocol player inventory slots to persistent indices.
func WindowSlotToNBT(slot int) (int8, bool) {
	switch {
	case slot >= 36 && slot <= 44:
		return int8(slot - 36), true
	case slot >= 9 && slot <= 35:
		return int8(slot), true
	case slot >= 5 && slot <= 8:
		return int8(108 - slot), true
	case slot == 45:
		return -106, true
	default:
		return 0, false
	}
}

// Save writes player data to a gzipped NBT file in the world directory.
func (pd *PlayerData) Save(worldDir string) error {
	if pd == nil {
		return fmt.Errorf("playerdata: nil player data")
	}
	if pd.DataVersion != protocol.CurrentDataVersion {
		return fmt.Errorf("playerdata: unsupported DataVersion %d", pd.DataVersion)
	}
	if pd.PortalCooldown < 0 {
		return fmt.Errorf("playerdata: invalid portal cooldown")
	}
	if pd.SelectedItemSlot < 0 || pd.SelectedItemSlot > 8 {
		return fmt.Errorf("playerdata: invalid selected item slot")
	}
	dir := filepath.Join(worldDir, "playerdata")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("playerdata: mkdir: %w", err)
	}

	c := pd.raw.Clone()
	if c == nil {
		c = nbt.NewCompound()
	}
	c.PutInt("DataVersion", protocol.CurrentDataVersion)
	c.PutInt("playerGameType", pd.GameMode)
	c.PutFloat("Health", pd.Health)
	c.PutInt("foodLevel", pd.FoodLevel)
	c.PutFloat("foodSaturationLevel", pd.FoodSat)
	c.PutFloat("foodExhaustionLevel", pd.FoodExhaustion)
	c.PutString("Dimension", pd.Dimension)
	c.PutInt("PortalCooldown", pd.PortalCooldown)
	c.PutInt("SelectedItemSlot", pd.SelectedItemSlot)
	c.PutBool("OnGround", pd.OnGround)
	c.PutInt("XpLevel", pd.XPLevel)
	c.PutInt("XpTotal", pd.XPTotal)
	c.PutFloat("XpP", pd.XPBar)
	pd.writeRecipeBook(c)
	if pd.HasSpawnPoint {
		c.PutInt("SpawnX", pd.SpawnX).PutInt("SpawnY", pd.SpawnY).PutInt("SpawnZ", pd.SpawnZ)
		c.PutString("SpawnDimension", pd.SpawnDimension).PutFloat("SpawnAngle", pd.SpawnAngle).PutBool("SpawnForced", pd.SpawnForced)
	} else {
		c.Delete("SpawnX")
		c.Delete("SpawnY")
		c.Delete("SpawnZ")
		c.Delete("SpawnDimension")
		c.Delete("SpawnAngle")
		c.Delete("SpawnForced")
	}
	writePlayerEffects(c, pd.raw, pd.Effects)

	pos := nbt.NewList(nbt.TagDouble)
	pos.Add(pd.PosX)
	pos.Add(pd.PosY)
	pos.Add(pd.PosZ)
	c.PutList("Pos", pos)

	rot := nbt.NewList(nbt.TagFloat)
	rot.Add(pd.Yaw)
	rot.Add(pd.Pitch)
	c.PutList("Rotation", rot)

	invList := nbt.NewList(nbt.TagCompound)
	for _, is := range pd.Inventory {
		if !is.Stack.Empty() {
			sc, err := inventory.ItemStackToNBT(is.Stack)
			if err != nil {
				return fmt.Errorf("player inventory: %w", err)
			}
			sc.PutByte("Slot", is.Slot)
			invList.Add(sc)
			continue
		}
		sc := is.raw.Clone()
		if sc == nil {
			sc = nbt.NewCompound()
		}
		sc.PutByte("Slot", is.Slot)
		sc.PutString("id", is.ItemName)
		if is.Count <= 0 || is.Count > 99 || is.ItemName == "" {
			return fmt.Errorf("playerdata: invalid inventory item at slot %d", is.Slot)
		}
		sc.Delete("Count")
		sc.Delete("Damage")
		sc.Delete("Enchantments")
		sc.PutInt("count", is.Count)
		components, _ := sc.GetCompound("components")
		components = components.Clone()
		if components == nil {
			components = nbt.NewCompound()
		}
		if is.Damage > 0 {
			components.PutInt("minecraft:damage", is.Damage)
		} else if _, exists := components.GetInt("minecraft:damage"); exists {
			components.PutInt("minecraft:damage", 0)
		}
		mergeInventoryEnchantments(components, is.Enchantments)
		if len(components.Entries()) > 0 {
			sc.PutCompound("components", components)
		} else {
			sc.Delete("components")
		}
		invList.Add(sc)
	}
	c.PutList("Inventory", invList)

	path := playerDataPath(worldDir, pd.UUID)
	payload, err := encodeGzipNBT(c, 2048)
	if err != nil {
		return fmt.Errorf("playerdata: %w", err)
	}
	if err := backupCurrentFile(path, path+"_old"); err != nil {
		return fmt.Errorf("playerdata: %w", err)
	}
	if err := storage.WriteFileAtomic(path, payload, 0644); err != nil {
		return fmt.Errorf("playerdata: save: %w", err)
	}
	pd.raw = c.Clone()
	return nil
}

// mergeInventoryEnchantments updates known enchantments without dropping unknown IDs.
func mergeInventoryEnchantments(components *nbt.Compound, current map[int32]int32) {
	enchantments, _ := components.GetCompound("minecraft:enchantments")
	enchantments = enchantments.Clone()
	if enchantments == nil && len(current) == 0 {
		return
	}
	if enchantments == nil {
		enchantments = nbt.NewCompound()
	}
	levels, _ := enchantments.GetCompound("levels")
	levels = levels.Clone()
	if levels == nil {
		levels = nbt.NewCompound()
	}
	for name := range levels.GetAllInts() {
		if id := enchantmentdata.EnchantmentIDByName(name); id >= 0 && current[id] <= 0 {
			levels.Delete(name)
		}
	}
	ids := make([]int32, 0, len(current))
	for id := range current {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		level := current[id]
		if info := enchantmentdata.EnchantmentByID(id); info != nil && level > 0 {
			levels.PutInt(info.Name, level)
		}
	}
	if len(levels.Entries()) == 0 {
		components.Delete("minecraft:enchantments")
		return
	}
	enchantments.PutCompound("levels", levels)
	components.PutCompound("minecraft:enchantments", enchantments)
}

func playerDataPath(worldDir, uuid string) string {
	return filepath.Join(worldDir, "playerdata", uuid+".dat")
}

// Clone returns an independent snapshot safe for background persistence.
func (pd *PlayerData) Clone() *PlayerData {
	if pd == nil {
		return nil
	}
	clone := *pd
	clone.RecipeBook = cloneBookState(pd.RecipeBook)
	clone.RecipeProgress = pd.RecipeProgress.Clone()
	clone.raw = pd.raw.Clone()
	clone.Inventory = make([]InventorySlot, len(pd.Inventory))
	for i, v := range pd.Inventory {
		v.raw = v.raw.Clone()
		v.Stack = v.Stack.Clone()
		v.Enchantments = maps.Clone(v.Enchantments)
		clone.Inventory[i] = v
	}
	clone.Effects = slices.Clone(pd.Effects)
	return &clone
}
