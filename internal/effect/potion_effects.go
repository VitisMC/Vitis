package effect

import (
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	potiondata "github.com/vitismc/vitis/internal/data/generated/potion"
)

// PotionEffect maps a potion ID to its applied status effects.
type PotionEffect struct {
	Effects []Instance
}

var potionEffectMap map[int32]*PotionEffect

func init() {
	potionEffectMap = buildPotionEffectMap()
}

// PotionEffects returns the effects for a given potion ID, or nil for non-effect potions.
func PotionEffects(potionID int32) *PotionEffect {
	return potionEffectMap[potionID]
}

// SplashDurationFactor returns the duration multiplier for splash potions (0.75).
func SplashDurationFactor() float64 { return 0.75 }

// LingeringDurationFactor returns the duration multiplier for lingering potions (0.25).
func LingeringDurationFactor() float64 { return 0.25 }

func buildPotionEffectMap() map[int32]*PotionEffect {
	flags := DefaultFlags()
	m := map[int32]*PotionEffect{
		potiondata.PotionNightVision: {Effects: []Instance{
			{ID: effectdata.EffectNightVision, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongNightVision: {Effects: []Instance{
			{ID: effectdata.EffectNightVision, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionInvisibility: {Effects: []Instance{
			{ID: effectdata.EffectInvisibility, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongInvisibility: {Effects: []Instance{
			{ID: effectdata.EffectInvisibility, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionLeaping: {Effects: []Instance{
			{ID: effectdata.EffectJumpBoost, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongLeaping: {Effects: []Instance{
			{ID: effectdata.EffectJumpBoost, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionStrongLeaping: {Effects: []Instance{
			{ID: effectdata.EffectJumpBoost, Amplifier: 1, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionFireResistance: {Effects: []Instance{
			{ID: effectdata.EffectFireResistance, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongFireResistance: {Effects: []Instance{
			{ID: effectdata.EffectFireResistance, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionSwiftness: {Effects: []Instance{
			{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongSwiftness: {Effects: []Instance{
			{ID: effectdata.EffectSpeed, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionStrongSwiftness: {Effects: []Instance{
			{ID: effectdata.EffectSpeed, Amplifier: 1, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionSlowness: {Effects: []Instance{
			{ID: effectdata.EffectSlowness, Amplifier: 0, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionLongSlowness: {Effects: []Instance{
			{ID: effectdata.EffectSlowness, Amplifier: 0, Duration: 4800, Flags: flags},
		}},
		potiondata.PotionStrongSlowness: {Effects: []Instance{
			{ID: effectdata.EffectSlowness, Amplifier: 3, Duration: 400, Flags: flags},
		}},
		potiondata.PotionTurtleMaster: {Effects: []Instance{
			{ID: effectdata.EffectSlowness, Amplifier: 3, Duration: 400, Flags: flags},
			{ID: effectdata.EffectResistance, Amplifier: 2, Duration: 400, Flags: flags},
		}},
		potiondata.PotionLongTurtleMaster: {Effects: []Instance{
			{ID: effectdata.EffectSlowness, Amplifier: 3, Duration: 800, Flags: flags},
			{ID: effectdata.EffectResistance, Amplifier: 2, Duration: 800, Flags: flags},
		}},
		potiondata.PotionStrongTurtleMaster: {Effects: []Instance{
			{ID: effectdata.EffectSlowness, Amplifier: 5, Duration: 400, Flags: flags},
			{ID: effectdata.EffectResistance, Amplifier: 3, Duration: 400, Flags: flags},
		}},
		potiondata.PotionWaterBreathing: {Effects: []Instance{
			{ID: effectdata.EffectWaterBreathing, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongWaterBreathing: {Effects: []Instance{
			{ID: effectdata.EffectWaterBreathing, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionHealing: {Effects: []Instance{
			{ID: effectdata.EffectInstantHealth, Amplifier: 0, Duration: 1, Flags: flags},
		}},
		potiondata.PotionStrongHealing: {Effects: []Instance{
			{ID: effectdata.EffectInstantHealth, Amplifier: 1, Duration: 1, Flags: flags},
		}},
		potiondata.PotionHarming: {Effects: []Instance{
			{ID: effectdata.EffectInstantDamage, Amplifier: 0, Duration: 1, Flags: flags},
		}},
		potiondata.PotionStrongHarming: {Effects: []Instance{
			{ID: effectdata.EffectInstantDamage, Amplifier: 1, Duration: 1, Flags: flags},
		}},
		potiondata.PotionPoison: {Effects: []Instance{
			{ID: effectdata.EffectPoison, Amplifier: 0, Duration: 900, Flags: flags},
		}},
		potiondata.PotionLongPoison: {Effects: []Instance{
			{ID: effectdata.EffectPoison, Amplifier: 0, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionStrongPoison: {Effects: []Instance{
			{ID: effectdata.EffectPoison, Amplifier: 1, Duration: 432, Flags: flags},
		}},
		potiondata.PotionRegeneration: {Effects: []Instance{
			{ID: effectdata.EffectRegeneration, Amplifier: 0, Duration: 900, Flags: flags},
		}},
		potiondata.PotionLongRegeneration: {Effects: []Instance{
			{ID: effectdata.EffectRegeneration, Amplifier: 0, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionStrongRegeneration: {Effects: []Instance{
			{ID: effectdata.EffectRegeneration, Amplifier: 1, Duration: 440, Flags: flags},
		}},
		potiondata.PotionStrength: {Effects: []Instance{
			{ID: effectdata.EffectStrength, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionLongStrength: {Effects: []Instance{
			{ID: effectdata.EffectStrength, Amplifier: 0, Duration: 9600, Flags: flags},
		}},
		potiondata.PotionStrongStrength: {Effects: []Instance{
			{ID: effectdata.EffectStrength, Amplifier: 1, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionWeakness: {Effects: []Instance{
			{ID: effectdata.EffectWeakness, Amplifier: 0, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionLongWeakness: {Effects: []Instance{
			{ID: effectdata.EffectWeakness, Amplifier: 0, Duration: 4800, Flags: flags},
		}},
		potiondata.PotionLuck: {Effects: []Instance{
			{ID: effectdata.EffectLuck, Amplifier: 0, Duration: 6000, Flags: flags},
		}},
		potiondata.PotionSlowFalling: {Effects: []Instance{
			{ID: effectdata.EffectSlowFalling, Amplifier: 0, Duration: 1800, Flags: flags},
		}},
		potiondata.PotionLongSlowFalling: {Effects: []Instance{
			{ID: effectdata.EffectSlowFalling, Amplifier: 0, Duration: 4800, Flags: flags},
		}},
		potiondata.PotionWindCharged: {Effects: []Instance{
			{ID: effectdata.EffectWindCharged, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionWeaving: {Effects: []Instance{
			{ID: effectdata.EffectWeaving, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionOozing: {Effects: []Instance{
			{ID: effectdata.EffectOozing, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
		potiondata.PotionInfested: {Effects: []Instance{
			{ID: effectdata.EffectInfested, Amplifier: 0, Duration: 3600, Flags: flags},
		}},
	}
	return m
}

// ScaleEffects returns copies of the effects with durations scaled by factor.
func ScaleEffects(effects []Instance, factor float64) []Instance {
	scaled := make([]Instance, len(effects))
	for i, e := range effects {
		scaled[i] = e
		scaled[i].Duration = int32(float64(e.Duration) * factor)
		if scaled[i].Duration < 1 {
			scaled[i].Duration = 1
		}
	}
	return scaled
}
