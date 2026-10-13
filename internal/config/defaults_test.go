package config

import "testing"

func TestDefaultChunkWorkersAvoidsSMTOversubscription(t *testing.T) {
	tests := map[int]int{1: 2, 2: 2, 4: 2, 6: 4, 8: 5, 12: 8, 16: 10}
	for parallelism, want := range tests {
		if got := defaultChunkWorkers(parallelism); got != want {
			t.Fatalf("defaultChunkWorkers(%d) = %d, want %d", parallelism, got, want)
		}
	}
}
