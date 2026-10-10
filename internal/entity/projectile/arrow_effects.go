package projectile

import (
	"fmt"
	"slices"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	potiondata "github.com/vitismc/vitis/internal/data/generated/potion"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
)

const maxArrowCustomEffects = 128

// PickupItem returns a detached single-item stack, including potion components.
func (a *Arrow) PickupItem() inventory.ItemStack { return a.pickupItem.Clone() }

// HitEffects returns detached effects; base potion durations are reduced to one eighth.
func (a *Arrow) HitEffects() []effect.Instance { return slices.Clone(a.hitEffects) }

// SetPickupItem validates and snapshots an arrow's item and compiled hit effects atomically.
func (a *Arrow) SetPickupItem(stack inventory.ItemStack) error {
	if stack.Empty() {
		return fmt.Errorf("missing arrow pickup item")
	}
	if stack.ItemID != item.IDByName("minecraft:arrow") && stack.ItemID != item.IDByName("minecraft:tipped_arrow") {
		return fmt.Errorf("invalid arrow pickup item type")
	}
	stack = stack.Clone()
	stack.ItemCount = 1
	buf := protocol.NewBuffer(128)
	if err := inventory.EncodeItemStack(buf, stack); err != nil {
		return fmt.Errorf("invalid arrow pickup item: %w", err)
	}
	// Normalize structured fields through the bounded component decoder.
	normalized, err := inventory.DecodeSlot(protocol.WrapBuffer(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("invalid arrow pickup components: %w", err)
	}
	effects, color, err := arrowPotionEffects(normalized)
	if err != nil {
		return err
	}
	normalized.PersistentData = stack.PersistentData.Clone()
	a.pickupItem, a.hitEffects, a.potionColor = normalized, effects, color
	a.metadataDirty = true
	return nil
}

// ConsumeMetadataDirty reports and clears pending potion-color synchronization.
func (a *Arrow) ConsumeMetadataDirty() bool {
	dirty := a.metadataDirty
	a.metadataDirty = false
	return dirty
}

// ConsumePotionExpiration reports and clears the grounded-arrow potion decay event.
func (a *Arrow) ConsumePotionExpiration() bool {
	expired := a.potionExpired
	a.potionExpired = false
	return expired
}

// tickPotionDecay removes potion components after six hundred grounded ticks.
func (a *Arrow) tickPotionDecay() {
	if a.Removed() {
		return
	}
	if !a.InGround() {
		a.inGroundTicks = 0
		return
	}
	a.inGroundTicks = min(a.inGroundTicks+1, 1201)
	if a.inGroundTicks < 600 {
		return
	}
	component, present := a.pickupItem.Component(dc.ComponentPotionContents)
	if !present {
		return
	}
	fields := component.(inventory.StructuredComponent).Data.(map[string]any)
	if fields["potionId"] == nil && fields["customColor"] == nil && fields["customName"] == nil && len(fields["customEffects"].([]any)) == 0 {
		return
	}
	a.pickupItem = inventory.NewSlot(item.IDByName("minecraft:arrow"), 1)
	a.hitEffects, a.potionColor = nil, -1
	a.metadataDirty, a.potionExpired = true, true
}

// AddEffect appends a validated custom effect while retaining the complete pickup item.
func (a *Arrow) AddEffect(instance effect.Instance) error {
	if effectdata.EffectByID(instance.ID) == nil || instance.Amplifier < 0 || instance.Amplifier > 255 || instance.Duration <= 0 || instance.Flags & ^byte(7) != 0 {
		return fmt.Errorf("invalid arrow effect")
	}
	stack := a.PickupItem()
	fields := map[string]any{"potionId": nil, "customColor": nil, "customEffects": []any{}, "customName": nil}
	if component, ok := stack.Component(dc.ComponentPotionContents); ok {
		structured, ok := component.(inventory.StructuredComponent)
		if !ok {
			return fmt.Errorf("invalid arrow potion contents")
		}
		fields, ok = structured.Data.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid arrow potion fields")
		}
	}
	custom, ok := fields["customEffects"].([]any)
	if !ok || len(custom) >= maxArrowCustomEffects {
		return fmt.Errorf("arrow custom effect limit exceeded")
	}
	fields["customEffects"] = append(custom, map[string]any{"id": instance.ID, "details": map[string]any{
		"amplifier": instance.Amplifier, "duration": instance.Duration,
		"ambient": instance.IsAmbient(), "showParticles": instance.ShowParticles(), "showIcon": instance.ShowIcon(), "hiddenEffect": nil,
	}})
	stack.SetComponent(dc.ComponentPotionContents, inventory.StructuredComponent{Data: fields})
	return a.SetPickupItem(stack)
}

// arrowPotionEffects compiles validated potion fields without mutating shared definitions.
func arrowPotionEffects(stack inventory.ItemStack) ([]effect.Instance, int32, error) {
	component, present := stack.Component(dc.ComponentPotionContents)
	if !present {
		return nil, -1, nil
	}
	structured, ok := component.(inventory.StructuredComponent)
	if !ok {
		return nil, 0, fmt.Errorf("invalid arrow potion component")
	}
	fields, ok := structured.Data.(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("invalid arrow potion fields")
	}
	var effects []effect.Instance
	if raw := fields["potionId"]; raw != nil {
		id, ok := raw.(int32)
		if !ok || id < 0 || id >= potiondata.PotionCount {
			return nil, 0, fmt.Errorf("invalid arrow potion ID")
		}
		if potion := effect.PotionEffects(id); potion != nil {
			effects = effect.ScaleEffects(potion.Effects, .125)
		}
	}
	custom, ok := fields["customEffects"].([]any)
	if !ok || len(custom) > maxArrowCustomEffects {
		return nil, 0, fmt.Errorf("invalid arrow custom effects")
	}
	for _, value := range custom {
		raw, ok := value.(map[string]any)
		if !ok {
			return nil, 0, fmt.Errorf("invalid arrow custom effect")
		}
		id, ok := raw["id"].(int32)
		if !ok || effectdata.EffectByID(id) == nil {
			return nil, 0, fmt.Errorf("unknown arrow effect ID")
		}
		details, ok := raw["details"].(map[string]any)
		if !ok {
			return nil, 0, fmt.Errorf("invalid arrow effect details")
		}
		duration, ok := details["duration"].(int32)
		if !ok || duration < 0 || details["hiddenEffect"] != nil {
			return nil, 0, fmt.Errorf("unsupported arrow effect duration or hidden effect")
		}
		amplifier, ok := details["amplifier"].(int32)
		if !ok || amplifier < 0 || amplifier > 255 {
			return nil, 0, fmt.Errorf("invalid arrow effect amplifier")
		}
		instance := effect.Instance{ID: id, Amplifier: amplifier, Duration: duration}
		if details["ambient"] == true {
			instance.Flags |= effect.FlagAmbient
		}
		if details["showParticles"] == true {
			instance.Flags |= effect.FlagParticles
		}
		if details["showIcon"] == true {
			instance.Flags |= effect.FlagIcon
		}
		effects = append(effects, instance)
	}
	color := arrowEffectColor(effects)
	if raw := fields["customColor"]; raw != nil {
		var ok bool
		color, ok = raw.(int32)
		if !ok {
			return nil, 0, fmt.Errorf("invalid arrow custom color")
		}
	} else if fields["potionId"] == nil && len(custom) == 0 && fields["customName"] == nil {
		color = -1
	}
	return effects, color, nil
}

// arrowEffectColor blends visible effect colors using amplifier-weighted RGB channels.
func arrowEffectColor(effects []effect.Instance) int32 {
	colors := [...]int32{3402751, 9154528, 14270531, 4866583, 16762624, 16262179, 11101546, 16646020, 5578058, 13458603, 9520880, 16750848, 10017472, 16185078, 2039587, 12779366, 5797459, 4738376, 8889187, 7561558, 16284963, 2445989, 16262179, 9740385, 13565951, 5882118, 12624973, 15978425, 1950417, 8954814, 745784, 4521796, 2696993, 1484454, 14565464, 12438015, 7891290, 10092451, 9214860}
	var red, green, blue, weight int32
	for _, instance := range effects {
		if !instance.ShowParticles() {
			continue
		}
		color, count := colors[instance.ID], instance.Amplifier+1
		red += (color >> 16 & 255) * count
		green += (color >> 8 & 255) * count
		blue += (color & 255) * count
		weight += count
	}
	if weight == 0 {
		return -13083194
	}
	return int32(-16777216) | red/weight<<16 | green/weight<<8 | blue/weight
}
