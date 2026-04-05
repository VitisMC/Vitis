package ai

import (
	"container/heap"
	"math"
)

const (
	maxPathLength  = 256
	maxSearchNodes = 2000
)

// PathNode represents a single position in a path.
type PathNode struct {
	X, Y, Z int
}

// Path is an ordered sequence of block positions from start to goal.
type Path struct {
	Nodes []PathNode
	Index int
}

// Done returns true if the path has been fully traversed.
func (p *Path) Done() bool {
	return p == nil || p.Index >= len(p.Nodes)
}

// Current returns the current node in the path.
func (p *Path) Current() PathNode {
	if p.Done() {
		return PathNode{}
	}
	return p.Nodes[p.Index]
}

// Advance moves to the next node.
func (p *Path) Advance() {
	if !p.Done() {
		p.Index++
	}
}

// Len returns the number of remaining nodes.
func (p *Path) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Nodes) - p.Index
}

// FindPath computes an A* path from start to goal using block access for collision.
// Returns nil if no path is found within search limits.
// The mob dimensions (width, height) determine passability.
func FindPath(blocks BlockAccess, start, goal PathNode, width, height float64) *Path {
	if blocks == nil {
		return nil
	}

	dist := math.Abs(float64(start.X-goal.X)) + math.Abs(float64(start.Y-goal.Y)) + math.Abs(float64(start.Z-goal.Z))
	if dist > float64(maxPathLength) {
		return nil
	}

	open := &nodeHeap{}
	heap.Init(open)

	closed := make(map[PathNode]bool, 128)
	cameFrom := make(map[PathNode]PathNode, 128)
	gScore := make(map[PathNode]float64, 128)

	gScore[start] = 0
	startNode := &astarNode{
		pos:   start,
		g:     0,
		f:     heuristic(start, goal),
	}
	heap.Push(open, startNode)

	searched := 0
	halfW := int(math.Ceil(width/2.0 - 0.5))
	h := int(math.Ceil(height))

	for open.Len() > 0 && searched < maxSearchNodes {
		searched++
		current := heap.Pop(open).(*astarNode)

		if current.pos == goal {
			return reconstructPath(cameFrom, current.pos)
		}

		if closed[current.pos] {
			continue
		}
		closed[current.pos] = true

		for _, neighbor := range neighbors(current.pos) {
			if closed[neighbor] {
				continue
			}
			if !isWalkable(blocks, neighbor, halfW, h) {
				continue
			}

			tentativeG := current.g + moveCost(current.pos, neighbor)
			if g, ok := gScore[neighbor]; ok && tentativeG >= g {
				continue
			}

			gScore[neighbor] = tentativeG
			cameFrom[neighbor] = current.pos
			heap.Push(open, &astarNode{
				pos: neighbor,
				g:   tentativeG,
				f:   tentativeG + heuristic(neighbor, goal),
			})
		}
	}

	return nil
}

func heuristic(a, b PathNode) float64 {
	dx := math.Abs(float64(a.X - b.X))
	dy := math.Abs(float64(a.Y - b.Y))
	dz := math.Abs(float64(a.Z - b.Z))
	return dx + dy + dz
}

func moveCost(from, to PathNode) float64 {
	if from.Y != to.Y {
		return 1.5
	}
	return 1.0
}

func neighbors(pos PathNode) []PathNode {
	return []PathNode{
		{pos.X + 1, pos.Y, pos.Z},
		{pos.X - 1, pos.Y, pos.Z},
		{pos.X, pos.Y, pos.Z + 1},
		{pos.X, pos.Y, pos.Z - 1},
		{pos.X + 1, pos.Y + 1, pos.Z},
		{pos.X - 1, pos.Y + 1, pos.Z},
		{pos.X, pos.Y + 1, pos.Z + 1},
		{pos.X, pos.Y + 1, pos.Z - 1},
		{pos.X + 1, pos.Y - 1, pos.Z},
		{pos.X - 1, pos.Y - 1, pos.Z},
		{pos.X, pos.Y - 1, pos.Z + 1},
		{pos.X, pos.Y - 1, pos.Z - 1},
	}
}

func isWalkable(blocks BlockAccess, pos PathNode, halfW, h int) bool {
	for bx := pos.X - halfW; bx <= pos.X+halfW; bx++ {
		for bz := pos.Z - halfW; bz <= pos.Z+halfW; bz++ {
			for by := pos.Y; by < pos.Y+h; by++ {
				state := blocks.GetBlockStateAt(bx, by, bz)
				if isSolidBlock(state) {
					return false
				}
			}
			groundState := blocks.GetBlockStateAt(bx, pos.Y-1, bz)
			if !isSolidBlock(groundState) {
				return false
			}
		}
	}
	return true
}

func isSolidBlock(stateID int32) bool {
	return stateID > 0
}

func reconstructPath(cameFrom map[PathNode]PathNode, current PathNode) *Path {
	var nodes []PathNode
	nodes = append(nodes, current)
	for {
		prev, ok := cameFrom[current]
		if !ok {
			break
		}
		nodes = append(nodes, prev)
		current = prev
	}

	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}

	if len(nodes) > 1 {
		nodes = nodes[1:]
	}

	return &Path{Nodes: nodes}
}

type astarNode struct {
	pos   PathNode
	g     float64
	f     float64
	index int
}

type nodeHeap []*astarNode

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool  { return h[i].f < h[j].f }
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }

func (h *nodeHeap) Push(x interface{}) {
	n := x.(*astarNode)
	n.index = len(*h)
	*h = append(*h, n)
}

func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return node
}
