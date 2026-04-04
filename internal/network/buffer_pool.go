package network

import (
	"sort"
	"sync"
)

var defaultBufferClasses = []int{256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}

type classPool struct {
	size int
	pool sync.Pool
}

// BufferPool reuses byte slices by size class to reduce GC pressure in hot paths.
type BufferPool struct {
	classes []classPool
}

// NewBufferPool builds a pool with the provided class sizes.
func NewBufferPool(classSizes []int) *BufferPool {
	if len(classSizes) == 0 {
		classSizes = defaultBufferClasses
	} else {
		classSizes = append([]int(nil), classSizes...)
		sort.Ints(classSizes)
	}

	classes := make([]classPool, 0, len(classSizes))
	for _, size := range classSizes {
		if size <= 0 {
			continue
		}

		classSize := size
		classes = append(classes, classPool{
			size: classSize,
			pool: sync.Pool{
				New: func() any {
					buf := make([]byte, classSize)
					return &buf
				},
			},
		})
	}

	if len(classes) == 0 {
		classSize := 4096
		classes = append(classes, classPool{
			size: classSize,
			pool: sync.Pool{
				New: func() any {
					buf := make([]byte, classSize)
					return &buf
				},
			},
		})
	}

	return &BufferPool{classes: classes}
}

// Get returns a slice with length size and a reusable backing array when possible.
func (p *BufferPool) Get(size int) []byte {
	if size <= 0 {
		return nil
	}

	idx := p.classIndex(size)
	if idx < 0 {
		return make([]byte, size)
	}

	bufPtr := p.classes[idx].pool.Get().(*[]byte)
	buf := *bufPtr
	return buf[:size]
}

// Put returns a slice to the pool when its capacity exactly matches a class size.
func (p *BufferPool) Put(buf []byte) {
	if cap(buf) == 0 {
		return
	}

	idx := p.classIndex(cap(buf))
	if idx < 0 || p.classes[idx].size != cap(buf) {
		return
	}

	buf = buf[:cap(buf)]
	p.classes[idx].pool.Put(&buf)
}

func (p *BufferPool) classIndex(size int) int {
	low := 0
	high := len(p.classes) - 1
	for low <= high {
		mid := (low + high) >> 1
		if p.classes[mid].size < size {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if low >= len(p.classes) {
		return -1
	}

	return low
}
