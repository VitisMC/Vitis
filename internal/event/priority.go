package event

// Priority determines the order in which handlers are called.
// Lower values run first.
type Priority int

const (
	PriorityLowest  Priority = -200
	PriorityLow     Priority = -100
	PriorityNormal  Priority = 0
	PriorityHigh    Priority = 100
	PriorityHighest Priority = 200
	PriorityMonitor Priority = 300
)
