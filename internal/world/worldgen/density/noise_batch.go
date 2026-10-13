package density

import "github.com/vitismc/vitis/internal/world/worldgen/noise"

// normalColumnBatch caches the last requested four-height group for a direct noise node.
type normalColumnBatch struct {
	x, z, group int
	values      [4]float64
	set         bool
	column      noise.NormalColumn
}

// sampleStandardNoise batches corner noise without changing scalar external sampling.
func sampleStandardNoise(id int, p Position, n *noise.Normal, xzScale, yScale float64) float64 {
	e := p.Evaluation
	if e == nil || !e.interpolation.preparing {
		return n.Sample(float64(p.X)*xzScale, float64(p.Y)*yScale, float64(p.Z)*xzScale)
	}
	for len(e.normalColumns) <= id {
		e.normalColumns = append(e.normalColumns, normalColumnBatch{})
	}
	value := &e.normalColumns[id]
	step := e.interpolation.step
	group := floorToMultiple(p.Y-e.minY, step*4) + e.minY
	if !value.set || value.x != p.X || value.z != p.Z || value.group != group {
		if !value.set || value.x != p.X || value.z != p.Z {
			n.PrepareColumn(float64(p.X)*xzScale, float64(p.Z)*xzScale, &value.column)
		}
		ys := [4]float64{float64(group) * yScale, float64(group+step) * yScale, float64(group+2*step) * yScale, float64(group+3*step) * yScale}
		value.values = value.column.SampleY4(ys)
		value.x, value.z, value.group, value.set = p.X, p.Z, group, true
	}
	return value.values[(p.Y-group)/step]
}
