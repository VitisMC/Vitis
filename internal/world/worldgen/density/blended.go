package density

import "github.com/vitismc/vitis/internal/world/worldgen/noise"

// blendedFunction prepares a reusable octave column during bounded chunk evaluation.
type blendedFunction struct {
	id      int
	sampler *noise.Blended
}

// blendedColumnValue stores independently mutable state for one evaluation.
type blendedColumnValue struct {
	x, z   int
	set    bool
	column noise.BlendedColumn
	group  int
	values [4]float64
	batch  bool
}

// Sample reuses horizontal octave state while preserving scalar sampling outside chunk fills.
func (f *blendedFunction) Sample(p Position) float64 {
	e := p.Evaluation
	if e == nil || !e.bounded {
		return f.sampler.Sample(p.X, p.Y, p.Z)
	}
	for len(e.blended) <= f.id {
		e.blended = append(e.blended, nil)
	}
	value := e.blended[f.id]
	if value == nil {
		value = &blendedColumnValue{}
		e.blended[f.id] = value
	}
	if !value.set || value.x != p.X || value.z != p.Z {
		f.sampler.PrepareColumn(p.X, p.Z, &value.column)
		value.x, value.z, value.set = p.X, p.Z, true
		value.batch = false
	}
	if work := &e.interpolation; work.preparing {
		group := floorToMultiple(p.Y-e.minY, work.step*4) + e.minY
		if !value.batch || value.group != group {
			value.values = value.column.Sample4([4]int{group, group + work.step, group + 2*work.step, group + 3*work.step})
			value.group, value.batch = group, true
		}
		return value.values[(p.Y-group)/work.step]
	}
	return value.column.Sample(p.Y)
}
