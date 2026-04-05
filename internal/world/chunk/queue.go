package chunk

const (
	defaultLoadRequestQueueCapacity = 8192
	defaultLoadResultQueueCapacity  = 8192
	defaultUnloadQueueCapacity      = 8192
)

// LoadRequest describes one asynchronous chunk load request.
type LoadRequest struct {
	X int32
	Z int32
}

// LoadResult describes one asynchronous chunk load completion.
type LoadResult struct {
	X     int32
	Z     int32
	Chunk *Chunk
	Err   error
}

// LoadRequestQueue is a bounded multi-producer request queue.
type LoadRequestQueue struct {
	ch chan LoadRequest
}

// NewLoadRequestQueue constructs a bounded request queue.
func NewLoadRequestQueue(capacity int) *LoadRequestQueue {
	if capacity <= 0 {
		capacity = defaultLoadRequestQueueCapacity
	}
	return &LoadRequestQueue{ch: make(chan LoadRequest, capacity)}
}

// Enqueue inserts a request without blocking.
func (q *LoadRequestQueue) Enqueue(request LoadRequest) bool {
	if q == nil {
		return false
	}
	select {
	case q.ch <- request:
		return true
	default:
		return false
	}
}

// Drain removes up to max requests into dst and returns the resulting slice.
func (q *LoadRequestQueue) Drain(dst []LoadRequest, max int) []LoadRequest {
	if q == nil || max <= 0 {
		return dst
	}
	for i := 0; i < max; i++ {
		select {
		case request := <-q.ch:
			dst = append(dst, request)
		default:
			return dst
		}
	}
	return dst
}

// Len returns currently queued request count.
func (q *LoadRequestQueue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.ch)
}

// LoadResultQueue is a bounded completion queue.
type LoadResultQueue struct {
	ch chan LoadResult
}

// NewLoadResultQueue constructs a bounded result queue.
func NewLoadResultQueue(capacity int) *LoadResultQueue {
	if capacity <= 0 {
		capacity = defaultLoadResultQueueCapacity
	}
	return &LoadResultQueue{ch: make(chan LoadResult, capacity)}
}

// Enqueue inserts a completion and blocks when queue is full.
func (q *LoadResultQueue) Enqueue(result LoadResult) {
	if q == nil {
		return
	}
	q.ch <- result
}

// TryEnqueue inserts a completion without blocking.
func (q *LoadResultQueue) TryEnqueue(result LoadResult) bool {
	if q == nil {
		return false
	}
	select {
	case q.ch <- result:
		return true
	default:
		return false
	}
}

// Drain removes up to max results into dst and returns the resulting slice.
func (q *LoadResultQueue) Drain(dst []LoadResult, max int) []LoadResult {
	if q == nil || max <= 0 {
		return dst
	}
	for i := 0; i < max; i++ {
		select {
		case result := <-q.ch:
			dst = append(dst, result)
		default:
			return dst
		}
	}
	return dst
}

// Len returns currently queued completion count.
func (q *LoadResultQueue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.ch)
}

// UnloadQueue stores pending unload keys in FIFO order.
type UnloadQueue struct {
	keys []int64
	head int
}

// NewUnloadQueue creates a bounded unload queue.
func NewUnloadQueue(capacity int) *UnloadQueue {
	if capacity <= 0 {
		capacity = defaultUnloadQueueCapacity
	}
	return &UnloadQueue{keys: make([]int64, 0, capacity)}
}

// Enqueue appends an unload key and returns false when queue is saturated.
func (q *UnloadQueue) Enqueue(key int64) bool {
	if q == nil {
		return false
	}
	if len(q.keys) == cap(q.keys) && q.head > 0 {
		copy(q.keys, q.keys[q.head:])
		q.keys = q.keys[:len(q.keys)-q.head]
		q.head = 0
	}
	if len(q.keys)-q.head >= cap(q.keys) {
		return false
	}
	q.keys = append(q.keys, key)
	return true
}

// Drain removes up to max keys and appends them into dst.
func (q *UnloadQueue) Drain(dst []int64, max int) []int64 {
	if q == nil || max <= 0 {
		return dst
	}
	available := len(q.keys) - q.head
	if available <= 0 {
		q.keys = q.keys[:0]
		q.head = 0
		return dst
	}
	if max > available {
		max = available
	}
	end := q.head + max
	dst = append(dst, q.keys[q.head:end]...)
	q.head = end
	if q.head >= len(q.keys) {
		q.keys = q.keys[:0]
		q.head = 0
	}
	return dst
}

// Len returns number of queued unload keys.
func (q *UnloadQueue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.keys) - q.head
}
