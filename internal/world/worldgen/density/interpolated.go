package density

type interpolated struct {
	id                          int
	function                    Function
	width, height               int
	widthMask, heightMask       int
	widthInverse, heightInverse float64
	powerOfTwo                  bool
}

func (f *interpolated) Sample(position Position) float64 {
	evaluation := position.Evaluation
	if evaluation != nil {
		current := &evaluation.interpolation
		if current.active && current.x == position.X && current.y == position.Y && current.z == position.Z && f.id < len(current.pairs) && current.pairs[f.id] >= 0 {
			pair := current.row + current.pairs[f.id]
			low, high := current.partials[pair], current.partials[pair+1]
			return low + current.fraction*(high-low)
		}
	}
	baseX, baseY, baseZ := 0, 0, 0
	xDelta, yDelta, zDelta := 0.0, 0.0, 0.0
	if f.powerOfTwo {
		baseX, baseY, baseZ = position.X&^f.widthMask, position.Y&^f.heightMask, position.Z&^f.widthMask
		xDelta = float64(position.X&f.widthMask) * f.widthInverse
		yDelta = float64(position.Y&f.heightMask) * f.heightInverse
		zDelta = float64(position.Z&f.widthMask) * f.widthInverse
	} else {
		baseX, baseY, baseZ = floorToMultiple(position.X, f.width), floorToMultiple(position.Y, f.height), floorToMultiple(position.Z, f.width)
		xDelta = float64(position.X-baseX) / float64(f.width)
		yDelta = float64(position.Y-baseY) / float64(f.height)
		zDelta = float64(position.Z-baseZ) / float64(f.width)
	}
	if position.Evaluation == nil {
		values := f.sampleCell(position, baseX, baseY, baseZ)
		return lerp3(xDelta, yDelta, zDelta, &values)
	}
	deltaY := position.Y - evaluation.minY
	cacheable := evaluation.bounded && position.X >= evaluation.originX && position.X < evaluation.originX+16 &&
		position.Z >= evaluation.originZ && position.Z < evaluation.originZ+16 && deltaY >= 0 && deltaY < evaluation.height
	values := evaluation.interpolationCellValue(f.id, baseX, baseY, baseZ, f.width, f.height, f.function, position)
	if !cacheable {
		return lerp3(xDelta, yDelta, zDelta, values)
	}
	x00 := values[0] + xDelta*(values[1]-values[0])
	x10 := values[2] + xDelta*(values[3]-values[2])
	x01 := values[4] + xDelta*(values[5]-values[4])
	x11 := values[6] + xDelta*(values[7]-values[6])
	y0 := x00 + yDelta*(x10-x00)
	y1 := x01 + yDelta*(x11-x01)
	for len(evaluation.partials) <= f.id {
		evaluation.partials = append(evaluation.partials, interpolationPartials{})
	}
	partial := &evaluation.partials[f.id]
	if len(partial.values) != evaluation.height {
		partial.values = make([][2]float64, evaluation.height)
		partial.set = make([]uint64, (evaluation.height+63)/64)
	}
	if partial.x != position.X || partial.z != baseZ {
		clear(partial.set)
		partial.x, partial.z = position.X, baseZ
	}
	partial.values[deltaY] = [2]float64{y0, y1}
	partial.set[deltaY>>6] |= uint64(1) << uint(deltaY&63)
	return y0 + zDelta*(y1-y0)
}

func (f *interpolated) sampleCell(position Position, baseX, baseY, baseZ int) [8]float64 {
	return [8]float64{
		f.corner(position, baseX, baseY, baseZ),
		f.corner(position, baseX+f.width, baseY, baseZ),
		f.corner(position, baseX, baseY+f.height, baseZ),
		f.corner(position, baseX+f.width, baseY+f.height, baseZ),
		f.corner(position, baseX, baseY, baseZ+f.width),
		f.corner(position, baseX+f.width, baseY, baseZ+f.width),
		f.corner(position, baseX, baseY+f.height, baseZ+f.width),
		f.corner(position, baseX+f.width, baseY+f.height, baseZ+f.width),
	}
}

func (f *interpolated) corner(parent Position, x, y, z int) float64 {
	position := Position{X: x, Y: y, Z: z, Evaluation: parent.Evaluation}
	if parent.Evaluation == nil {
		return f.function.Sample(position)
	}
	return parent.Evaluation.interpolationSample(f.id, x, y, z, f.width, f.height, f.function)
}

func lerp3(x, y, z float64, values *[8]float64) float64 {
	x00 := values[0] + x*(values[1]-values[0])
	x10 := values[2] + x*(values[3]-values[2])
	x01 := values[4] + x*(values[5]-values[4])
	x11 := values[6] + x*(values[7]-values[6])
	y0 := x00 + y*(x10-x00)
	y1 := x01 + y*(x11-x01)
	return y0 + z*(y1-y0)
}
