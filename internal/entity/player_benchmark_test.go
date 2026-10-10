package entity

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func BenchmarkPlayerVisibleChunkKeys(b *testing.B) {
	p := NewPlayer(1, protocol.UUID{}, "player", Vec3{}, Vec2{}, nil, 10)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = p.VisibleChunkKeys()
	}
}
