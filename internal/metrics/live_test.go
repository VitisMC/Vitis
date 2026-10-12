package metrics

import (
	"math"
	"testing"
	"time"
)

// TestLiveSamplerReportsIntervalRates verifies independent generation and send rates.
func TestLiveSamplerReportsIntervalRates(t *testing.T) {
	activity := &ChunkActivity{}
	sampler := NewLiveSampler(activity)
	sampler.Sample()
	activity.RecordGeneration(5 * time.Millisecond)
	activity.RecordGeneration(15 * time.Millisecond)
	activity.RecordSent()
	activity.RecordFullPath(20 * time.Millisecond)
	activity.RecordSent()
	activity.RecordSent()
	sampler.lastTime = sampler.lastTime.Add(-time.Second)
	snapshot := sampler.Sample()
	if math.Abs(snapshot.GeneratedPerSec-2) > 0.1 || math.Abs(snapshot.SentPerSec-3) > 0.1 {
		t.Fatalf("unexpected rates: generation=%f send=%f", snapshot.GeneratedPerSec, snapshot.SentPerSec)
	}
	if snapshot.AvgChunkMS != 10 || snapshot.LastChunkMS != 15 {
		t.Fatalf("unexpected generation times: average=%f last=%f", snapshot.AvgChunkMS, snapshot.LastChunkMS)
	}
	if math.Abs(snapshot.FullPathPerSec-1) > 0.1 || snapshot.AvgFullPathMS != 20 || snapshot.LastFullPathMS != 20 {
		t.Fatalf("unexpected full path metrics: %+v", snapshot)
	}
	if snapshot.RAMBytes == 0 {
		t.Fatal("expected nonzero process memory")
	}
}
