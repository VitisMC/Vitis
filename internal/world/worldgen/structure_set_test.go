package worldgen

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestStructureRandomSpreadCoordinates checks linear and triangular placement.
func TestStructureRandomSpreadCoordinates(t *testing.T) {
	structures := map[string]json.RawMessage{"minecraft:test": json.RawMessage(`{}`)}
	tests := []struct {
		name         string
		raw          string
		x, z         int32
		wantX, wantZ int32
	}{
		{
			name: "linear with negative region",
			raw:  `{"structures":[{"structure":"minecraft:test","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":34,"separation":8,"salt":10387312}}`,
			x:    -68, z: 102, wantX: -49, wantZ: 121,
		},
		{
			name: "triangular with negative region",
			raw:  `{"structures":[{"structure":"minecraft:test","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":20,"separation":11,"salt":10387313,"spread_type":"triangular"}}`,
			x:    -20, z: -20, wantX: -16, wantZ: -17,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set, err := decodeStructureSet(json.RawMessage(test.raw), structures)
			if err != nil {
				t.Fatal(err)
			}
			x, z, ok := set.placement.potentialChunk(123456789, test.x, test.z)
			if !ok || x != test.wantX || z != test.wantZ {
				t.Fatalf("candidate = (%d,%d,%t), want (%d,%d,true)", x, z, ok, test.wantX, test.wantZ)
			}
		})
	}
}

// TestStructureSetValidationRejectsMissingReferences checks reload input validation.
func TestStructureSetValidationRejectsMissingReferences(t *testing.T) {
	set := &ResourceSet{resources: map[string]map[string]json.RawMessage{
		"structure":     {"minecraft:test": json.RawMessage(`{}`)},
		"structure_set": {"minecraft:set": json.RawMessage(`{"structures":[{"structure":"minecraft:test","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":32,"separation":8,"salt":1,"exclusion_zone":{"other_set":"minecraft:missing","chunk_count":10}}}`)},
	}}
	if err := compileStructureSets(set); err == nil || !strings.Contains(err.Error(), "unknown excluded set minecraft:missing") {
		t.Fatalf("missing exclusion error = %v", err)
	}
	set.resources["structure_set"]["minecraft:set"] = json.RawMessage(`{"structures":[{"structure":"minecraft:missing","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":32,"separation":8,"salt":1}}`)
	if err := compileStructureSets(set); err == nil || !strings.Contains(err.Error(), "unknown structure minecraft:missing") {
		t.Fatalf("missing structure error = %v", err)
	}
}

// TestStructureExclusionZone checks nearby placements and unrelated regions.
func TestStructureExclusionZone(t *testing.T) {
	resources := &ResourceSet{structureSets: map[string]*structureSet{
		"minecraft:outer": {placement: structurePlacement{
			kind: "minecraft:random_spread", spacing: 1, frequency: 1,
			exclusionSet: "minecraft:inner", exclusionRadius: 1,
		}},
		"minecraft:inner": {placement: structurePlacement{
			kind: "minecraft:random_spread", spacing: 8, frequency: 1,
		}},
	}}
	seed := int64(123456789)
	inner := resources.structureSets["minecraft:inner"].placement
	x, z, ok := inner.potentialChunk(seed, 0, 0)
	if !ok || !resources.isStructureChunk("minecraft:inner", seed, x, z) {
		t.Fatalf("inner candidate (%d,%d) missing", x, z)
	}
	for dx := int32(-1); dx <= 1; dx++ {
		for dz := int32(-1); dz <= 1; dz++ {
			if resources.isStructureChunk("minecraft:outer", seed, x+dx, z+dz) {
				t.Fatalf("outer placement at (%d,%d) ignored exclusion", x+dx, z+dz)
			}
		}
	}
	if !resources.isStructureChunk("minecraft:outer", seed, x+2, z) {
		t.Fatal("outer placement outside exclusion was rejected")
	}
}

// TestStructureSetValidationRejectsExclusionCycles prevents unbounded recursion.
func TestStructureSetValidationRejectsExclusionCycles(t *testing.T) {
	resources := &ResourceSet{resources: map[string]map[string]json.RawMessage{
		"structure": {"minecraft:test": json.RawMessage(`{}`)},
		"structure_set": {
			"minecraft:a": json.RawMessage(`{"structures":[{"structure":"minecraft:test","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":1,"separation":0,"salt":1,"exclusion_zone":{"other_set":"minecraft:b","chunk_count":1}}}`),
			"minecraft:b": json.RawMessage(`{"structures":[{"structure":"minecraft:test","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":1,"separation":0,"salt":2,"exclusion_zone":{"other_set":"minecraft:a","chunk_count":1}}}`),
		},
	}}
	if err := compileStructureSets(resources); err == nil || !strings.Contains(err.Error(), "cyclic exclusion zone") {
		t.Fatalf("cyclic exclusion error = %v", err)
	}
}

// TestConcentricRingPlacementAcceptsBiomeLists checks supported holder-set forms.
func TestConcentricRingPlacementAcceptsBiomeLists(t *testing.T) {
	structures := map[string]json.RawMessage{"minecraft:test": json.RawMessage(`{}`)}
	for _, preferred := range []string{`"#minecraft:stronghold_biased_to"`, `["minecraft:plains","minecraft:forest"]`} {
		raw := `{"structures":[{"structure":"minecraft:test","weight":1}],"placement":{"type":"minecraft:concentric_rings","salt":0,"distance":0,"spread":0,"count":1,"preferred_biomes":` + preferred + `}}`
		set, err := decodeStructureSet(json.RawMessage(raw), structures)
		if err != nil {
			t.Fatalf("preferred biomes %s: %v", preferred, err)
		}
		if set.placement.kind != "minecraft:concentric_rings" {
			t.Fatalf("placement kind = %q", set.placement.kind)
		}
	}
}
