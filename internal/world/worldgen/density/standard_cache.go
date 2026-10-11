package density

// lastStandardValue reads the last exact positional result of a generated cache marker.
func (e *Evaluation) lastStandardValue(id int, p Position) (float64, bool) {
	if id >= len(e.once) {
		return 0, false
	}
	value := e.once[id]
	return value.value, value.set && value.key == positionKey(p)
}

// storeStandardLast publishes a marker's last result without interface dispatch.
func (e *Evaluation) storeStandardLast(id int, p Position, value float64) {
	for len(e.once) <= id {
		e.once = append(e.once, cachedValue{})
	}
	e.once[id] = cachedValue{key: positionKey(p), value: value, set: true}
}

// standardHorizontalValue reads chunk-local or external horizontal marker storage.
func (e *Evaluation) standardHorizontalValue(id int, p Position) (float64, bool) {
	if value, ok := e.lastStandardValue(id, p); ok {
		return value, true
	}
	dx, dz := p.X-e.originX, p.Z-e.originZ
	if !e.bounded || dx < 0 || dx > 16 || dz < 0 || dz > 16 {
		value, ok := e.cache(id)[positionKey(p)]
		if ok {
			e.storeStandardLast(id, p, value)
		}
		return value, ok
	}
	for len(e.horizontal) <= id {
		e.horizontal = append(e.horizontal, nil)
	}
	grid := e.horizontal[id]
	if grid == nil {
		grid = &horizontalGrid{}
		e.horizontal[id] = grid
	}
	index := dx*17 + dz
	if grid.set[index>>6]&(uint64(1)<<uint(index&63)) == 0 {
		return 0, false
	}
	value := grid.values[index]
	e.storeStandardLast(id, p, value)
	return value, true
}

// storeStandardHorizontal stores an independently evaluated generated horizontal result.
func (e *Evaluation) storeStandardHorizontal(id int, p Position, value float64) {
	dx, dz := p.X-e.originX, p.Z-e.originZ
	if !e.bounded || dx < 0 || dx > 16 || dz < 0 || dz > 16 {
		e.cache(id)[positionKey(p)] = value
	} else {
		grid := e.horizontal[id]
		index := dx*17 + dz
		grid.values[index] = value
		grid.set[index>>6] |= uint64(1) << uint(index&63)
	}
	e.storeStandardLast(id, p, value)
}
