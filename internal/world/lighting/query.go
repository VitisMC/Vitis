package lighting

// Levels returns separate block and sky light at an available local position.
func (e *Engine) Levels(x, y, z int) (uint8, uint8) {
	if e == nil || x < 0 || x > 15 || z < 0 || z > 15 || y < e.minY || y > e.maxY {
		return 0, 0
	}
	return e.getBlockLight(x, y, z), e.getSkyLight(x, y, z)
}

// RawBrightness reads local light, optionally excluding a dimension's sky light.
func (e *Engine) RawBrightness(x, y, z int, hasSkyLight bool) uint8 {
	if e == nil || x < 0 || x > 15 || z < 0 || z > 15 || y < e.minY || y > e.maxY {
		return 0
	}
	value := e.getBlockLight(x, y, z)
	if hasSkyLight {
		value = max(value, e.getSkyLight(x, y, z))
	}
	return value
}
