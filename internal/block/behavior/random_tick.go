package behavior

// RandomTickContext supplies tick-owned block access, light, and bounded randomness.
type RandomTickContext struct {
	X, Y, Z     int
	StateID     int32
	Light       uint8
	GetBlock    func(int, int, int) (int32, bool)
	RandomBound func(int32) int32
}

// RandomTickBehavior extends block behavior without changing scheduled tick contracts.
type RandomTickBehavior interface {
	CanRandomTick(int32) bool
	OnRandomTick(*RandomTickContext) *TickResult
}
