package section

import (
	"testing"
)

func TestBitStorageGetSet(t *testing.T) {
	bs := NewBitStorage(4, 16, nil)
	bs.Set(0, 5)
	bs.Set(15, 10)
	if got := bs.Get(0); got != 5 {
		t.Fatalf("Get(0) = %d, want 5", got)
	}
	if got := bs.Get(15); got != 10 {
		t.Fatalf("Get(15) = %d, want 10", got)
	}
	if got := bs.Get(1); got != 0 {
		t.Fatalf("Get(1) = %d, want 0", got)
	}
}

func TestBitStorageZeroBits(t *testing.T) {
	bs := NewBitStorage(0, 100, nil)
	if got := bs.Get(50); got != 0 {
		t.Fatalf("Get(50) with 0 bits = %d, want 0", got)
	}
	bs.Set(50, 0)
}

func TestBitStorageSize(t *testing.T) {
	tests := []struct {
		bits, length, want int
	}{
		{0, 100, 0},
		{4, 16, 1},
		{4, 4096, 256},
		{8, 4096, 512},
		{15, 4096, 1024},
		{1, 64, 1},
	}
	for _, tt := range tests {
		got := BitStorageSize(tt.bits, tt.length)
		if got != tt.want {
			t.Errorf("BitStorageSize(%d, %d) = %d, want %d", tt.bits, tt.length, got, tt.want)
		}
	}
}

func TestBitStorageLargeValues(t *testing.T) {
	bs := NewBitStorage(15, 4096, nil)
	bs.Set(0, 27865)
	bs.Set(4095, 12345)
	if got := bs.Get(0); got != 27865 {
		t.Fatalf("Get(0) = %d, want 27865", got)
	}
	if got := bs.Get(4095); got != 12345 {
		t.Fatalf("Get(4095) = %d, want 12345", got)
	}
}

func TestSinglePalette(t *testing.T) {
	pc := NewSinglePalette(BlocksPerSection, 42)
	for i := 0; i < BlocksPerSection; i++ {
		if got := pc.Get(i); got != 42 {
			t.Fatalf("Get(%d) = %d, want 42", i, got)
		}
	}
	if !pc.IsSingle() {
		t.Fatal("expected single mode")
	}
}

func TestPaletteGrowFromSingle(t *testing.T) {
	pc := NewSinglePalette(BlocksPerSection, 0)
	pc.Set(0, 1)
	if pc.IsSingle() {
		t.Fatal("should no longer be single after Set with new value")
	}
	if got := pc.Get(0); got != 1 {
		t.Fatalf("Get(0) = %d, want 1", got)
	}
	for i := 1; i < BlocksPerSection; i++ {
		if got := pc.Get(i); got != 0 {
			t.Fatalf("Get(%d) = %d, want 0", i, got)
		}
	}
}

func TestPaletteMultipleValues(t *testing.T) {
	pc := NewSinglePalette(BlocksPerSection, 0)
	for i := 0; i < 16; i++ {
		pc.Set(i, int32(i+1))
	}
	for i := 0; i < 16; i++ {
		if got := pc.Get(i); got != int32(i+1) {
			t.Fatalf("Get(%d) = %d, want %d", i, got, i+1)
		}
	}
	for i := 16; i < BlocksPerSection; i++ {
		if got := pc.Get(i); got != 0 {
			t.Fatalf("Get(%d) = %d, want 0", i, got)
		}
	}
}

func TestPalettePromoteToDirect(t *testing.T) {
	pc := NewSinglePalette(BlocksPerSection, 0)
	for i := 0; i < 300; i++ {
		pc.Set(i, int32(i+1))
	}
	for i := 0; i < 300; i++ {
		if got := pc.Get(i); got != int32(i+1) {
			t.Fatalf("Get(%d) = %d, want %d", i, got, i+1)
		}
	}
}

func TestSectionGetSetBlock(t *testing.T) {
	s := NewSection(0, 0, 0)
	s.SetBlock(0, 0, 0, 1)
	s.SetBlock(15, 15, 15, 42)
	if got := s.GetBlock(0, 0, 0); got != 1 {
		t.Fatalf("GetBlock(0,0,0) = %d, want 1", got)
	}
	if got := s.GetBlock(15, 15, 15); got != 42 {
		t.Fatalf("GetBlock(15,15,15) = %d, want 42", got)
	}
	if got := s.GetBlock(1, 0, 0); got != 0 {
		t.Fatalf("GetBlock(1,0,0) = %d, want 0", got)
	}
}

func TestSectionGetSetBiome(t *testing.T) {
	s := NewSection(0, 0, 5)
	if got := s.GetBiome(0, 0, 0); got != 5 {
		t.Fatalf("GetBiome(0,0,0) = %d, want 5", got)
	}
	s.SetBiome(0, 0, 0, 10)
	if got := s.GetBiome(0, 0, 0); got != 10 {
		t.Fatalf("GetBiome(0,0,0) = %d, want 10", got)
	}
}

func TestSectionNonAirCount(t *testing.T) {
	s := NewSection(0, 0, 0)
	if got := s.NonAirCount(); got != 0 {
		t.Fatalf("NonAirCount = %d, want 0", got)
	}
	s.SetBlock(0, 0, 0, 1)
	s.SetBlock(1, 0, 0, 2)
	s.SetBlock(2, 0, 0, 3)
	if got := s.NonAirCount(); got != 3 {
		t.Fatalf("NonAirCount = %d, want 3", got)
	}
}

func TestIndirectPalette(t *testing.T) {
	palette := []int32{0, 1, 2, 3}
	data := make([]uint64, BitStorageSize(4, BlocksPerSection))
	pc := NewIndirectPalette(BlocksPerSection, 4, palette, data)

	pc.Set(0, 3)
	pc.Set(1, 1)
	if got := pc.Get(0); got != 3 {
		t.Fatalf("Get(0) = %d, want 3", got)
	}
	if got := pc.Get(1); got != 1 {
		t.Fatalf("Get(1) = %d, want 1", got)
	}
}

func TestDirectPalette(t *testing.T) {
	data := make([]uint64, BitStorageSize(DirectBitsBlock, BlocksPerSection))
	pc := NewDirectPalette(BlocksPerSection, DirectBitsBlock, data)

	pc.Set(0, 27865)
	pc.Set(4095, 100)
	if got := pc.Get(0); got != 27865 {
		t.Fatalf("Get(0) = %d, want 27865", got)
	}
	if got := pc.Get(4095); got != 100 {
		t.Fatalf("Get(4095) = %d, want 100", got)
	}
}

func TestBitsFor(t *testing.T) {
	tests := []struct {
		count int
		want  int
	}{
		{0, 0}, {1, 0}, {2, 1}, {3, 2}, {4, 2}, {5, 3}, {16, 4}, {17, 5}, {256, 8}, {257, 9},
	}
	for _, tt := range tests {
		got := bitsFor(tt.count)
		if got != tt.want {
			t.Errorf("bitsFor(%d) = %d, want %d", tt.count, got, tt.want)
		}
	}
}

func BenchmarkSectionSetBlock(b *testing.B) {
	s := NewSection(0, 0, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SetBlock(i%16, (i/16)%16, (i/256)%16, int32(i%100)+1)
	}
}

func BenchmarkSectionGetBlock(b *testing.B) {
	s := NewSection(0, 0, 0)
	for i := 0; i < 4096; i++ {
		s.SetBlock(i%16, (i/16)%16, (i/256)%16, int32(i%100)+1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetBlock(i%16, (i/16)%16, (i/256)%16)
	}
}
