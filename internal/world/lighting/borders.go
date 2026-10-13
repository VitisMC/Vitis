package lighting

// BlockLight reads one in-range block light value.
func (e *Engine) BlockLight(x, y, z int) uint8 {
	if e == nil || x < 0 || x > 15 || z < 0 || z > 15 || y < e.minY || y > e.maxY {
		return 0
	}
	return e.getBlockLight(x, y, z)
}

// SkyLight reads one in-range sky light value.
func (e *Engine) SkyLight(x, y, z int) uint8 {
	if e == nil || x < 0 || x > 15 || z < 0 || z > 15 || y < e.minY || y > e.maxY {
		return 0
	}
	return e.getSkyLight(x, y, z)
}

// ImportBorders propagates incoming neighbor light through local blocks.
// Neighbor coordinates are local to this engine and lie just outside its four faces.
func (e *Engine) ImportBorders(access BlockAccess, sky bool, neighbor func(int, int, int) uint8) bool {
	if e == nil || access == nil || neighbor == nil {
		return false
	}
	queue := lightQueuePool.Get().([]lightNode)
	defer func() { releaseLightQueue(queue) }()
	for y := e.minY; y <= e.maxY; y++ {
		for offset := 0; offset < 16; offset++ {
			for _, face := range [4][4]int{{0, offset, -1, offset}, {15, offset, 16, offset}, {offset, 0, offset, -1}, {offset, 15, offset, 16}} {
				x, z := face[0], face[1]
				level := int(neighbor(face[2], y, face[3])) - max(1, int(blockOpacity(access.GetBlock(x, y, z))))
				if level <= 0 {
					continue
				}
				current := e.getBlockLight(x, y, z)
				if sky {
					current = e.getSkyLight(x, y, z)
				}
				if uint8(level) <= current {
					continue
				}
				if sky {
					e.setSkyLight(x, y, z, uint8(level))
				} else {
					e.setBlockLight(x, y, z, uint8(level))
				}
				queue = append(queue, lightNode{x: x, y: y, z: z, level: uint8(level)})
			}
		}
	}
	changed := len(queue) != 0
	queue = e.propagate(queue, access, sky)
	return changed
}
