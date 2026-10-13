package entity

import (
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestSpeciesEffectImmunitiesSurviveRestore checks direct application and persisted hostile effects.
func TestSpeciesEffectImmunitiesSurviveRestore(t *testing.T) {
	for _, name := range []string{"zombie", "husk", "drowned", "skeleton", "stray", "wither_skeleton", "phantom", "spider", "cave_spider", "cow"} {
		t.Run(name, func(t *testing.T) {
			mob := NewMobEntityByName(1, protocol.UUID{1}, name, Vec3{})
			undead := name != "spider" && name != "cave_spider" && name != "cow"
			for _, id := range []int32{effectdata.EffectPoison, effectdata.EffectRegeneration, effectdata.EffectWither, effectdata.EffectSlowness} {
				allowed := !((id == effectdata.EffectPoison && name != "cow") || (id == effectdata.EffectRegeneration && undead) || (id == effectdata.EffectWither && name == "wither_skeleton"))
				if mob.Effects().Add(effect.Instance{ID: id, Duration: 100, Flags: effect.DefaultFlags()}) != allowed {
					t.Fatalf("effect=%d accepted incorrectly", id)
				}
			}
			raw, err := mob.PersistentNBT()
			if err != nil {
				t.Fatal(err)
			}
			poison := nbt.NewCompound().PutString("id", "minecraft:poison").PutByte("amplifier", 0).PutInt("duration", 100)
			raw.PutList("active_effects", nbt.NewList(nbt.TagCompound).Add(poison))
			restored, err := DecodePersistentEntity(2, raw)
			if err != nil {
				t.Fatal(err)
			}
			if restored.(*MobEntity).Effects().Has(effectdata.EffectPoison) != (name == "cow") {
				t.Fatal("restoration bypassed species immunity")
			}
		})
	}
}

// TestBabyExperienceRulesAreStableAcrossRestore checks hostile and passive baby rewards.
func TestBabyExperienceRulesAreStableAcrossRestore(t *testing.T) {
	for _, name := range []string{"zombie", "husk", "drowned", "cow", "pig", "sheep", "chicken"} {
		mob := NewMobEntityByName(1, protocol.UUID{1}, name, Vec3{})
		raw, err := mob.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		if name == "zombie" || name == "husk" || name == "drowned" {
			raw.PutBool("IsBaby", true)
		} else {
			raw.PutInt("Age", -24000)
		}
		restored, err := DecodePersistentEntity(2, raw)
		if err != nil {
			t.Fatal(err)
		}
		baby := restored.(*MobEntity)
		want := int32(0)
		if name == "zombie" || name == "husk" || name == "drowned" {
			want = 12
		}
		if baby.XPReward() != want || baby.XPReward() != want || mob.XPReward() != mob.TypeDef().XPReward {
			t.Fatalf("%s reward mutated or incorrect", name)
		}
	}
}
