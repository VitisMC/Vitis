package entity

const (
	relativeMoveFactor = 4096.0
	relativeMoveCap    = 32767
	angleFactor        = 256.0 / 360.0
)

// PositionDelta computes protocol-level relative movement deltas (fixed-point * 4096).
// Returns the deltas and whether they fit in int16 range (±8 blocks).
// If fits is false, a teleport packet must be used instead.
func PositionDelta(prev, cur Vec3) (dx, dy, dz int16, fits bool) {
	rawX := (cur.X - prev.X) * relativeMoveFactor
	rawY := (cur.Y - prev.Y) * relativeMoveFactor
	rawZ := (cur.Z - prev.Z) * relativeMoveFactor

	ix := int64(rawX)
	iy := int64(rawY)
	iz := int64(rawZ)

	if ix < -relativeMoveCap || ix > relativeMoveCap ||
		iy < -relativeMoveCap || iy > relativeMoveCap ||
		iz < -relativeMoveCap || iz > relativeMoveCap {
		return 0, 0, 0, false
	}

	return int16(ix), int16(iy), int16(iz), true
}

// PositionChanged returns true if the position differs between prev and cur.
func PositionChanged(prev, cur Vec3) bool {
	return prev.X != cur.X || prev.Y != cur.Y || prev.Z != cur.Z
}

// RotationChanged returns true if the rotation differs between prev and cur.
func RotationChanged(prev, cur Vec2) bool {
	return prev.X != cur.X || prev.Y != cur.Y
}

// AngleToByte converts a float32 angle in degrees to the protocol byte representation (256/360).
func AngleToByte(degrees float32) byte {
	v := int32(degrees * angleFactor)
	return byte(v & 0xFF)
}

// ByteToAngle converts a protocol angle byte back to degrees.
func ByteToAngle(b byte) float32 {
	return float32(b) / float32(angleFactor)
}
