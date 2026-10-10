package entity

import (
	"math"
	"reflect"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	genattr "github.com/vitismc/vitis/internal/data/generated/attribute"
	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestPersistentEntitiesRoundTrip verifies runtime state and unknown NBT ownership.
func TestPersistentEntitiesRoundTrip(t *testing.T) {
	uuid := protocol.UUID{0xf123456789abcdef, 0x12345678fedcba98}
	position := Vec3{-17.25, 64.5, 33.75}
	stack, err := inventory.ItemStackFromNBT(nbt.NewCompound().PutString("id", "minecraft:diamond_pickaxe").PutInt("count", 1).PutCompound("components", nbt.NewCompound().PutInt("minecraft:damage", 7).PutCompound("example:unknown", nbt.NewCompound().PutString("keep", "item"))).PutString("extra", "stack"))
	if err != nil {
		t.Fatal(err)
	}
	item := NewItemEntityWithVelocity(1, uuid, position, stack, Vec3{0.1, -0.2, 0.3}, 27)
	item.age, item.health = 123, 3
	item.SetThrower(protocol.UUID{8, 9})
	orb := NewXPOrb(2, uuid, position, 19)
	orb.age = 43
	tnt := NewTNTEntityWithFuse(3, uuid, position, -1, 39)
	tnt.SetPower(6)
	state, err := block.BlockStateFromProperties("minecraft:oak_log", map[string]string{"axis": "x"})
	if err != nil {
		t.Fatal(err)
	}
	falling := NewFallingBlock(4, uuid, position, state.ID())
	falling.ticksExisted = 71
	mob := NewMobEntityByName(5, uuid, "zombie", position)
	mob.SetMaxHealth(40)
	mob.SetHealth(13)
	mob.SetNoAI(true)
	mob.ageTicks = 97
	mob.Attributes().AddModifier(genattr.AttrMovementSpeed, Modifier{ID: "example:speed", Amount: 0.2, Operation: 2})
	mob.Effects().Add(effect.Instance{ID: effectdata.EffectStrength, Amplifier: 2, Duration: 321, Flags: effect.DefaultFlags()})
	for _, original := range []PersistentEntity{item, orb, tnt, falling, mob} {
		raw, err := original.PersistentNBT()
		if err != nil {
			t.Fatal(err)
		}
		name, _ := raw.GetString("id")
		t.Run(name, func(t *testing.T) {
			raw.PutCompound("example:extra", nbt.NewCompound().PutString("keep", "entity"))
			decoded, err := DecodePersistentEntity(0, raw)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.BaseEntity().UUID() != uuid || decoded.BaseEntity().Position() != position || decoded.BaseEntity().Velocity() != original.BaseEntity().Velocity() {
				t.Fatal("identity or movement lost")
			}
			if !decoded.BaseEntity().BindRuntimeID(17) || decoded.BaseEntity().BindRuntimeID(18) {
				t.Fatal("runtime ID ownership failed")
			}
			again, err := decoded.PersistentNBT()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(raw, again) {
				t.Fatalf("persistent state changed: %#v -> %#v", raw.Entries(), again.Entries())
			}
			extra, _ := raw.GetCompound("example:extra")
			extra.PutString("keep", "mutated")
			check, _ := again.GetCompound("example:extra")
			if value, _ := check.GetString("keep"); value != "entity" {
				t.Fatal("snapshot shares input NBT")
			}
		})
	}
}

// TestPersistentEntityRejectsMalformedState covers hostile movement and typed payloads.
func TestPersistentEntityRejectsMalformedState(t *testing.T) {
	item := NewItemEntity(1, protocol.UUID{1, 2}, Vec3{1, 64, 1}, inventory.NewSlot(1, 1))
	raw, err := item.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*nbt.Compound){
		"uuid": func(c *nbt.Compound) { c.PutIntArray("UUID", []int32{1, 2}) },
		"position": func(c *nbt.Compound) {
			c.PutList("Pos", nbt.NewList(nbt.TagDouble).Add(math.NaN()).Add(float64(64)).Add(float64(0)))
		},
		"position type": func(c *nbt.Compound) { c.PutInt("Pos", 1) },
		"position bounds": func(c *nbt.Compound) {
			c.PutList("Pos", nbt.NewList(nbt.TagDouble).Add(float64(30000001)).Add(float64(64)).Add(float64(0)))
		},
		"motion": func(c *nbt.Compound) {
			c.PutList("Motion", nbt.NewList(nbt.TagDouble).Add(float64(0)).Add(math.Inf(1)).Add(float64(0)))
		},
		"motion bounds": func(c *nbt.Compound) {
			c.PutList("Motion", nbt.NewList(nbt.TagDouble).Add(float64(11)).Add(float64(0)).Add(float64(0)))
		},
		"rotation": func(c *nbt.Compound) {
			c.PutList("Rotation", nbt.NewList(nbt.TagFloat).Add(float32(math.NaN())).Add(float32(0)))
		},
		"age type":      func(c *nbt.Compound) { c.PutInt("Age", 1) },
		"missing stack": func(c *nbt.Compound) { c.Delete("Item") },
		"stack": func(c *nbt.Compound) {
			c.PutCompound("Item", nbt.NewCompound().PutString("id", "minecraft:stone").PutInt("count", -1))
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			c := raw.Clone()
			change(c)
			if _, err := DecodePersistentEntity(0, c); err == nil {
				t.Fatal("malformed entity accepted")
			}
		})
	}
}

// TestPersistentMobPreservesUnknownNestedTags verifies attribute and effect merge behavior.
func TestPersistentMobPreservesUnknownNestedTags(t *testing.T) {
	mob := NewMobEntityByName(1, protocol.UUID{2, 3}, "cow", Vec3{1, 64, 1})
	mob.Effects().Add(effect.Instance{ID: effectdata.EffectSpeed, Amplifier: 1, Duration: 100, Flags: effect.DefaultFlags()})
	raw, err := mob.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	attributes, _ := raw.GetList("attributes")
	attributes.Elements()[0].(*nbt.Compound).PutString("extra", "attribute")
	active, _ := raw.GetList("active_effects")
	active.Elements()[0].(*nbt.Compound).PutString("extra", "effect")
	active.Add(nbt.NewCompound().PutString("id", "example:effect").PutString("opaque", "keep"))
	decoded, err := DecodePersistentEntity(0, raw)
	if err != nil {
		t.Fatal(err)
	}
	again, err := decoded.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	attrs, _ := again.GetList("attributes")
	if value, _ := attrs.Elements()[0].(*nbt.Compound).GetString("extra"); value != "attribute" {
		t.Fatal("unknown attribute data lost")
	}
	entries := indexedCompounds(again, "active_effects", "id")
	if value, _ := entries["minecraft:speed"].GetString("extra"); value != "effect" {
		t.Fatal("unknown effect fields lost")
	}
	if value, _ := entries["example:effect"].GetString("opaque"); value != "keep" {
		t.Fatal("unknown effect lost")
	}
}

// TestExperienceOrbInitialVelocityIsFinite covers integer-overflow-sensitive protocol IDs.
func TestExperienceOrbInitialVelocityIsFinite(t *testing.T) {
	for _, id := range []int32{1, 2, 3, 4, 2147483647} {
		orb := NewXPOrb(id, protocol.UUID{1, 2}, Vec3{}, 1)
		velocity := orb.Velocity()
		if !validVector(velocity, 1) || velocity.X < -0.1 || velocity.X > 0.1 || velocity.Z < -0.1 || velocity.Z > 0.1 {
			t.Fatalf("invalid initial velocity for ID %d: %+v", id, velocity)
		}
	}
}

// TestExperienceOrbMergePreservesPersistentRange prevents merged XP from becoming unsavable.
func TestExperienceOrbMergePreservesPersistentRange(t *testing.T) {
	first := NewXPOrb(1, protocol.UUID{1, 1}, Vec3{}, math.MaxInt16)
	second := NewXPOrb(2, protocol.UUID{1, 2}, Vec3{}, 1)
	if first.TryMerge(second) || first.XPValue() != math.MaxInt16 || second.Removed() {
		t.Fatal("overflowing merge changed XP state")
	}
	raw, err := second.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	raw.PutInt("Count", 2)
	if decoded, err := DecodePersistentEntity(0, raw); err != nil || decoded == nil || decoded.(*XPOrb).Count() != 2 {
		t.Fatal("counted orb was not restored")
	}
}

// TestPermanentItemPreservesElapsedAge verifies permanent lifetime without truncating counters.
func TestPermanentItemPreservesElapsedAge(t *testing.T) {
	item := NewItemEntity(1, protocol.UUID{1, 2}, Vec3{}, inventory.NewSlot(1, 1))
	item.age = 40000
	item.SetNoDespawn(true)
	raw, err := item.PersistentNBT()
	if err != nil {
		t.Fatal(err)
	}
	if age, _ := raw.GetShort("Age"); age != -32768 {
		t.Fatal("permanent item lost vanilla sentinel")
	}
	decoded, err := DecodePersistentEntity(0, raw)
	if err != nil {
		t.Fatal(err)
	}
	restored := decoded.(*ItemEntity)
	if !restored.noDespawn || restored.Age() != 40000 {
		t.Fatal("permanent item age was truncated")
	}
}

// FuzzPersistentEntity exercises bounded entity decoding and detached serialization.
func FuzzPersistentEntity(f *testing.F) {
	source := NewTNTEntity(1, protocol.UUID{1, 2}, Vec3{1, 64, 1}, -1)
	raw, _ := source.PersistentNBT()
	encoder := nbt.NewEncoder(512)
	_ = encoder.WriteNamedRootCompound("", raw)
	f.Add(encoder.Bytes())
	for _, source := range []PersistentEntity{NewItemEntity(1, protocol.UUID{2, 3}, Vec3{X: 8, Y: 64, Z: 8}, inventory.NewSlot(1, 2)), NewMobEntityByName(2, protocol.UUID{3, 4}, "zombie", Vec3{X: 8, Y: 64, Z: 8})} {
		raw, err := source.PersistentNBT()
		if err != nil {
			f.Fatal(err)
		}
		encoder := nbt.NewEncoder(512)
		if err := encoder.WriteNamedRootCompound("", raw); err != nil {
			f.Fatal(err)
		}
		f.Add(encoder.Bytes())
	}
	f.Add([]byte{10, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		_, raw, err := nbt.NewDecoder(data).ReadNamedRootCompound()
		if err != nil || raw == nil {
			return
		}
		decoded, err := DecodePersistentEntity(0, raw)
		if err != nil || decoded == nil {
			return
		}
		_, _ = decoded.PersistentNBT()
	})
}
