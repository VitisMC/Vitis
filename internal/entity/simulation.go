package entity

import "math"

// IsTickingAt applies a world's simulation policy while retaining standalone physics compatibility.
func IsTickingAt(world any, position Vec3) bool {
	if gate, ok := world.(interface{ IsEntityTicking(int32, int32) bool }); ok {
		return gate.IsEntityTicking(int32(math.Floor(position.X/16)), int32(math.Floor(position.Z/16)))
	}
	return true
}
