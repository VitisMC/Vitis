package climate

import "math"

// depthGroup indexes points with identical vertical intervals independently of depth.
type depthGroup struct {
	interval Parameter
	nodes    []depthNode
}

// depthNode retains original leaf ranks so grouped lookup preserves global tie order.
type depthNode struct {
	parameters [parameterCount]Parameter
	children   [6]int32
	count      uint8
	leaf       int32
}

// depthCandidate is the exact horizontal winner for one vertical parameter interval.
type depthCandidate struct {
	leaf     int32
	distance int64
}

// buildDepthGroups specializes small sets of vertical intervals without approximating climate.
func (i *Index) buildDepthGroups() {
	var intervals []Parameter
	var leaves [][]*node
	var visit func(*node)
	visit = func(n *node) {
		if !n.leaf {
			for _, child := range n.children {
				visit(child)
			}
			return
		}
		interval := n.parameters[4]
		for group, existing := range intervals {
			if existing == interval {
				leaves[group] = append(leaves[group], n)
				return
			}
		}
		intervals = append(intervals, interval)
		leaves = append(leaves, []*node{n})
	}
	visit(i.root)
	if len(intervals) > 8 {
		return
	}
	i.groups = make([]depthGroup, len(intervals))
	for index, interval := range intervals {
		g := &i.groups[index]
		g.interval = interval
		g.flatten(build(leaves[index]))
	}
}

// flatten stores one group's immutable search nodes contiguously.
func (g *depthGroup) flatten(n *node) int32 {
	index := int32(len(g.nodes))
	g.nodes = append(g.nodes, depthNode{parameters: n.parameters, leaf: n.rank})
	for offset, child := range n.children {
		childIndex := g.flatten(child)
		g.nodes[index].children[offset] = childIndex
	}
	g.nodes[index].count = uint8(len(n.children))
	return index
}

// distance computes horizontal climate cost with the global evaluator's early bound.
func (n *depthNode) distance(target *[5]int64, limit int64) int64 {
	var total int64
	for offset, dimension := range [...]uint8{0, 1, 2, 3, 5} {
		parameter, value := n.parameters[dimension], target[offset]
		var delta int64
		if value > parameter.Max {
			delta = value - parameter.Max
		} else if value < parameter.Min {
			delta = parameter.Min - value
		}
		total += delta * delta
		if total > limit {
			return total
		}
	}
	delta := n.parameters[6].distance(0)
	return total + delta*delta
}

// find returns the closest horizontal leaf, breaking ties by original tree rank.
func (g *depthGroup) find(target *[5]int64) depthCandidate {
	best := depthCandidate{leaf: math.MaxInt32, distance: math.MaxInt64}
	var stack [64]int32
	length := 1
	for length > 0 {
		length--
		n := &g.nodes[stack[length]]
		distance := n.distance(target, best.distance)
		if distance > best.distance {
			continue
		}
		if n.count == 0 {
			if distance < best.distance || n.leaf < best.leaf {
				best = depthCandidate{leaf: n.leaf, distance: distance}
			}
			continue
		}
		for offset := int(n.count) - 1; offset >= 0; offset-- {
			stack[length] = n.children[offset]
			length++
		}
	}
	return best
}

// findDepthGroups searches horizontal climate once and combines exact vertical costs per query.
func (i *Index) findDepthGroups(target Target, cursor *Cursor) int32 {
	horizontal := [5]int64{target[0], target[1], target[2], target[3], target[5]}
	if !cursor.groupsSet || cursor.index != i || cursor.groupTarget != horizontal {
		for index := range i.groups {
			cursor.groupCandidates[index] = i.groups[index].find(&horizontal)
		}
		cursor.groupTarget, cursor.groupsSet = horizontal, true
	}
	best := depthCandidate{leaf: math.MaxInt32, distance: math.MaxInt64}
	for index, group := range i.groups {
		candidate := cursor.groupCandidates[index]
		delta := group.interval.distance(target[4])
		distance := candidate.distance + delta*delta
		if distance < best.distance || distance == best.distance && candidate.leaf < best.leaf {
			best = depthCandidate{leaf: candidate.leaf, distance: distance}
		}
	}
	cursor.index, cursor.flat, cursor.set = i, best.leaf, true
	return best.leaf
}
