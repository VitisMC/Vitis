package behavior

// NeighborContext supplies resident block access for shape and support updates.
type NeighborContext struct {
	X, Y, Z                   int
	SourceX, SourceY, SourceZ int
	StateID                   int32
	Light                     uint8
	Scheduled                 bool
	GetBlock                  func(int, int, int) (int32, bool)
}

// NeighborResult describes a shape change or a delayed support check.
type NeighborResult struct {
	StateID int32
	Changed bool
	Drop    bool
	Delay   int
}

// NeighborBehavior extends existing behavior without changing its public contract.
type NeighborBehavior interface {
	OnNeighborCheck(*NeighborContext) *NeighborResult
}
