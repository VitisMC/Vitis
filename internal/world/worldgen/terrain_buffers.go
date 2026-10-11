package worldgen

// terrainBufferPool keeps a small number of large worker buffers across GC cycles.
type terrainBufferPool[T any] struct {
	buffers chan []T
}

// newTerrainBufferPool bounds retained buffers independently of chunk-cache sizes.
func newTerrainBufferPool[T any](capacity int) terrainBufferPool[T] {
	return terrainBufferPool[T]{buffers: make(chan []T, capacity)}
}

// Get takes an exclusively owned buffer, or returns nil when none is available.
func (p *terrainBufferPool[T]) Get() []T {
	select {
	case buffer := <-p.buffers:
		return buffer
	default:
		return nil
	}
}

// Put retains an exclusively owned buffer when capacity remains.
func (p *terrainBufferPool[T]) Put(buffer []T) {
	if buffer == nil {
		return
	}
	select {
	case p.buffers <- buffer:
	default:
	}
}
