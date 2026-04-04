package vehicle

import (
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
)

// Vehicle is the base interface for mountable entities.
type Vehicle interface {
	EntityID() int32
	Passengers() []int32
	AddPassenger(id int32) bool
	RemovePassenger(id int32) bool
	MaxPassengers() int
	Tick(world physics.BlockAccess)
	Entity() *entity.Entity
}

// BaseVehicle provides common vehicle state.
type BaseVehicle struct {
	entity     *entity.Entity
	passengers []int32
	maxSeats   int
}

// NewBaseVehicle creates a base vehicle wrapping an entity.
func NewBaseVehicle(e *entity.Entity, maxSeats int) *BaseVehicle {
	return &BaseVehicle{
		entity:     e,
		passengers: make([]int32, 0, maxSeats),
		maxSeats:   maxSeats,
	}
}

// EntityID returns the vehicle's entity ID.
func (v *BaseVehicle) EntityID() int32 { return v.entity.ID() }

// Entity returns the underlying entity.
func (v *BaseVehicle) Entity() *entity.Entity { return v.entity }

// Passengers returns the current passenger entity IDs.
func (v *BaseVehicle) Passengers() []int32 {
	out := make([]int32, len(v.passengers))
	copy(out, v.passengers)
	return out
}

// AddPassenger adds a passenger to the vehicle. Returns false if full.
func (v *BaseVehicle) AddPassenger(id int32) bool {
	if len(v.passengers) >= v.maxSeats {
		return false
	}
	for _, p := range v.passengers {
		if p == id {
			return false
		}
	}
	v.passengers = append(v.passengers, id)
	return true
}

// RemovePassenger removes a passenger from the vehicle.
func (v *BaseVehicle) RemovePassenger(id int32) bool {
	for i, p := range v.passengers {
		if p == id {
			v.passengers[i] = v.passengers[len(v.passengers)-1]
			v.passengers = v.passengers[:len(v.passengers)-1]
			return true
		}
	}
	return false
}

// MaxPassengers returns the maximum number of passengers.
func (v *BaseVehicle) MaxPassengers() int { return v.maxSeats }

// HasPassengers returns true if the vehicle has any passengers.
func (v *BaseVehicle) HasPassengers() bool { return len(v.passengers) > 0 }
