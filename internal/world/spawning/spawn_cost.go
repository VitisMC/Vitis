package spawning

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	entitydata "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity"
)

// spawnCost bounds the potential energy added by a biome's entity placement.
type spawnCost struct {
	charge float64
	budget float64
}

// spawnCharge records a contributing entity's block position and biome charge.
type spawnCharge struct {
	position entity.Vec3
	charge   float64
}

// spawnPotential reserves biome energy for existing mobs and accepted requests.
type spawnPotential []spawnCharge

// compileSpawnCosts validates finite biome cost definitions before reload.
func compileSpawnCosts(definitions map[string]json.RawMessage) (map[string]spawnCost, error) {
	costs := make(map[string]spawnCost, len(definitions))
	for name, data := range definitions {
		typeName := strings.TrimPrefix(name, "minecraft:")
		if entitydata.EntityByName(typeName) == nil {
			return nil, fmt.Errorf("unknown spawn cost entity %q", name)
		}
		var raw struct {
			Charge *float64 `json:"charge"`
			Budget *float64 `json:"energy_budget"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("spawn cost %s: %w", name, err)
		}
		if raw.Charge == nil || raw.Budget == nil || math.IsNaN(*raw.Charge) || math.IsInf(*raw.Charge, 0) || math.IsNaN(*raw.Budget) || math.IsInf(*raw.Budget, 0) {
			return nil, fmt.Errorf("spawn cost %s: charge and energy budget must be finite", name)
		}
		if _, exists := costs[typeName]; exists {
			return nil, fmt.Errorf("duplicate spawn cost for %s", typeName)
		}
		costs[typeName] = spawnCost{charge: *raw.Charge, budget: *raw.Budget}
	}
	return costs, nil
}

// costAt reads a species' cost from the resident biome in one compiled snapshot.
func (definitions *spawnDefinitions) costAt(environment Environment, name string, pos entity.Vec3) spawnCost {
	biome := environment.SpawnBiome(int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z)))
	return definitions.costs[biome][name]
}

// add reserves a nonzero charge at its integer block coordinates.
func (potential *spawnPotential) add(pos entity.Vec3, charge float64) {
	if charge == 0 {
		return
	}
	position := entity.Vec3{X: math.Floor(pos.X), Y: math.Floor(pos.Y), Z: math.Floor(pos.Z)}
	*potential = append(*potential, spawnCharge{position: position, charge: charge})
}

// allows checks inverse-distance potential against the candidate's biome budget.
func (potential spawnPotential) allows(pos entity.Vec3, cost spawnCost) bool {
	if cost.charge == 0 {
		return cost.budget >= 0
	}
	position := entity.Vec3{X: math.Floor(pos.X), Y: math.Floor(pos.Y), Z: math.Floor(pos.Z)}
	energy := 0.0
	for _, point := range potential {
		distance := distanceSquared(point.position, position)
		if distance == 0 {
			energy += math.Inf(1)
		} else {
			energy += point.charge / math.Sqrt(distance)
		}
	}
	return energy*cost.charge <= cost.budget
}
