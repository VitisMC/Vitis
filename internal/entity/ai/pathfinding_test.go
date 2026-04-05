package ai

import "testing"

type mockBlocks struct {
	solid map[[3]int]bool
}

func (m *mockBlocks) GetBlockStateAt(x, y, z int) int32 {
	if m.solid[[3]int{x, y, z}] {
		return 1
	}
	return 0
}

func flatWorld(groundY int) *mockBlocks {
	m := &mockBlocks{solid: make(map[[3]int]bool)}
	for x := -20; x <= 20; x++ {
		for z := -20; z <= 20; z++ {
			m.solid[[3]int{x, groundY, z}] = true
		}
	}
	return m
}

func TestFindPathStraightLine(t *testing.T) {
	blocks := flatWorld(63)
	start := PathNode{0, 64, 0}
	goal := PathNode{5, 64, 0}

	path := FindPath(blocks, start, goal, 0.6, 1.8)
	if path == nil {
		t.Fatal("expected path, got nil")
	}
	if path.Done() {
		t.Fatal("expected non-empty path")
	}

	last := path.Nodes[len(path.Nodes)-1]
	if last != goal {
		t.Fatalf("expected path to end at goal %v, got %v", goal, last)
	}
}

func TestFindPathNoPath(t *testing.T) {
	blocks := flatWorld(63)
	for z := -20; z <= 20; z++ {
		blocks.solid[[3]int{3, 64, z}] = true
		blocks.solid[[3]int{3, 65, z}] = true
		blocks.solid[[3]int{3, 66, z}] = true
	}

	start := PathNode{0, 64, 0}
	goal := PathNode{5, 64, 0}

	path := FindPath(blocks, start, goal, 0.6, 1.8)
	if path != nil {
		t.Fatal("expected no path through wall")
	}
}

func TestFindPathAroundObstacle(t *testing.T) {
	blocks := flatWorld(63)
	for z := -2; z <= 2; z++ {
		blocks.solid[[3]int{3, 64, z}] = true
		blocks.solid[[3]int{3, 65, z}] = true
	}

	start := PathNode{0, 64, 0}
	goal := PathNode{5, 64, 0}

	path := FindPath(blocks, start, goal, 0.6, 1.8)
	if path == nil {
		t.Fatal("expected path around obstacle")
	}

	last := path.Nodes[len(path.Nodes)-1]
	if last != goal {
		t.Fatalf("expected path to reach goal %v, got %v", goal, last)
	}

	if len(path.Nodes) <= 5 {
		t.Fatal("expected path to be longer than straight line due to obstacle")
	}
}

func TestFindPathStepUp(t *testing.T) {
	blocks := flatWorld(63)
	blocks.solid[[3]int{2, 64, 0}] = true
	blocks.solid[[3]int{3, 64, 0}] = true

	start := PathNode{0, 64, 0}
	goal := PathNode{3, 65, 0}

	path := FindPath(blocks, start, goal, 0.6, 1.8)
	if path == nil {
		t.Fatal("expected path with step-up")
	}
}

func TestFindPathTooFar(t *testing.T) {
	blocks := flatWorld(63)
	start := PathNode{0, 64, 0}
	goal := PathNode{300, 64, 0}

	path := FindPath(blocks, start, goal, 0.6, 1.8)
	if path != nil {
		t.Fatal("expected nil for path beyond max distance")
	}
}

func TestPathTraversal(t *testing.T) {
	path := &Path{
		Nodes: []PathNode{{1, 64, 0}, {2, 64, 0}, {3, 64, 0}},
	}

	if path.Done() {
		t.Fatal("new path should not be done")
	}
	if path.Len() != 3 {
		t.Fatalf("expected len 3, got %d", path.Len())
	}

	c := path.Current()
	if c.X != 1 {
		t.Fatalf("expected first node X=1, got %d", c.X)
	}

	path.Advance()
	if path.Len() != 2 {
		t.Fatalf("expected len 2 after advance, got %d", path.Len())
	}

	path.Advance()
	path.Advance()
	if !path.Done() {
		t.Fatal("expected path to be done")
	}
}

func TestFindPathNilBlocks(t *testing.T) {
	path := FindPath(nil, PathNode{0, 64, 0}, PathNode{5, 64, 0}, 0.6, 1.8)
	if path != nil {
		t.Fatal("expected nil path with nil blocks")
	}
}
