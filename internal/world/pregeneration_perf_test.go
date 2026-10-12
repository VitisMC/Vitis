package world

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/worldgen"
)

// BenchmarkPregenerationFreshWorld measures durable new chunks at the server tick rate.
// VITIS_BENCH_DIMENSION and VITIS_BENCH_SIDE select the dimension and odd square side.
// VITIS_BENCH_RADIUS overrides the square radius; VITIS_BENCH_SEED selects its seed.
// VITIS_BENCH_DEADLINE overrides the per-world 20-minute completion deadline.
// VITIS_BENCH_TRACE logs interval throughput, resident chunks, heap and tick duration.
// VITIS_BENCH_IDLE samples memory with the world alive after this positive duration.
// VITIS_BENCH_KEEP_CACHES retains generation caches for a reclamation comparison.
func BenchmarkPregenerationFreshWorld(b *testing.B) {
	dimension := os.Getenv("VITIS_BENCH_DIMENSION")
	if dimension == "" {
		dimension = "minecraft:overworld"
	}
	side := 101
	if value := os.Getenv("VITIS_BENCH_SIDE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 3 || parsed%2 == 0 {
			b.Fatal("VITIS_BENCH_SIDE must be an odd integer >= 3")
		}
		side = parsed
	}
	radius := (side / 2) * 16
	if value := os.Getenv("VITIS_BENCH_RADIUS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 30000 {
			b.Fatal("VITIS_BENCH_RADIUS must be an integer in [1, 30000]")
		}
		radius = parsed
	}
	minCoordinate, maxCoordinate := floorDiv16(-radius), floorDiv16(radius)
	width := int64(maxCoordinate - minCoordinate + 1)
	workers := max(2, runtime.GOMAXPROCS(0)*2/3)
	seed := int64(0)
	if value := os.Getenv("VITIS_BENCH_SEED"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			b.Fatal("VITIS_BENCH_SEED must be an int64")
		}
		seed = parsed
	}
	trace := os.Getenv("VITIS_BENCH_TRACE") == "1"
	var idleDuration time.Duration
	if value := os.Getenv("VITIS_BENCH_IDLE"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			b.Fatal("VITIS_BENCH_IDLE must be a positive duration")
		}
		idleDuration = parsed
	}
	completionDeadline := 20 * time.Minute
	if value := os.Getenv("VITIS_BENCH_DEADLINE"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			b.Fatal("VITIS_BENCH_DEADLINE must be a positive duration")
		}
		completionDeadline = parsed
	}
	var seconds float64
	var generated int64
	var diagnostics worldgen.GenerationDiagnostics
	var idleRSS, idleHeap uint64
	var savePeak, requestPeak, resultPeak int
	var diagnosticTicks, saveFullTicks int
	observe := os.Getenv("VITIS_BENCH_DIAGNOSTICS") == "1"
	b.ReportAllocs()
	b.StopTimer()
	for range b.N {
		resources := &datapack.Manager{}
		generation, err := worldgen.NewRuntime(resources)
		if err != nil {
			b.Fatal(err)
		}
		if err := resources.Reload([]string{"../../.mcdata/1.21.4/vanilla"}); err != nil {
			b.Fatal(err)
		}
		terrain, err := generation.NewTerrainGenerator(dimension, seed, 0)
		if err != nil {
			b.Fatal(err)
		}
		generator, err := chunk.NewWorldgenGenerator(terrain)
		if observe {
			terrain.EnableDiagnostics()
		}
		if err != nil {
			b.Fatal(err)
		}
		storeDir := b.TempDir()
		store := persistence.NewChunkStore(storeDir)
		var source chunk.Generator = generator
		if os.Getenv("VITIS_BENCH_KEEP_CACHES") == "1" {
			source = retainedCacheGenerator{generator}
		}
		w, err := New(Config{
			Name: "benchmark", Dimension: dimension, Generator: source, ChunkStore: store,
			ChunkLoadWorkers: workers, ChunkWorkerQueueSize: workers,
			ChunkRequestQueueSize: 8192, ChunkResultQueueSize: 8192,
		})
		if err != nil {
			b.Fatal(err)
		}
		ticker := time.NewTicker(time.Second / 20)
		b.StartTimer()
		if err := w.StartPregeneration("square", 0, 0, radius); err != nil {
			b.Fatal(err)
		}
		deadline := time.Now().Add(completionDeadline)
		traceSince := time.Now()
		var traceCompleted int64
		var tickTime time.Duration
		var tickCount int
		var status PregenerationStatus
		var activeMemory runtime.MemStats
		var activeRSS uint64
		for {
			<-ticker.C
			startedTick := time.Now()
			w.Tick()
			if trace {
				tickTime += time.Since(startedTick)
				tickCount++
			}
			if observe {
				diagnosticTicks++
				savePeak = max(savePeak, len(w.autosave.queue))
				if len(w.autosave.queue) == cap(w.autosave.queue) {
					saveFullTicks++
				}
				requestPeak = max(requestPeak, w.chunks.PendingLoadRequests())
				resultPeak = max(resultPeak, w.chunks.PendingLoadCompletions())
			}
			status = w.PregenerationStatus()
			if idleDuration > 0 && activeMemory.HeapAlloc == 0 && status.State == "running" && status.Total-status.Completed <= 256 {
				runtime.ReadMemStats(&activeMemory)
				activeRSS = benchmarkRSS(b)
			}
			if trace && time.Since(traceSince) >= 10*time.Second {
				now := time.Now()
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				stats := terrain.Diagnostics()
				b.Logf("interval completed=%d rate=%.1f loaded=%d heap_mib=%.1f gc=%d tick_ms=%.3f repeated_sources=%d sources=%d save_queue=%d", status.Completed, float64(status.Completed-traceCompleted)/now.Sub(traceSince).Seconds(), w.chunks.Len(), float64(memory.HeapAlloc)/(1<<20), memory.NumGC, float64(tickTime)/float64(tickCount)/float64(time.Millisecond), stats.RepeatedRawGenerations, stats.RawGenerations, len(w.autosave.queue))
				traceSince, traceCompleted, tickTime, tickCount = now, status.Completed, 0, 0
			}
			if status.State != "running" || time.Now().After(deadline) {
				break
			}
		}
		b.StopTimer()
		if idleDuration > 0 && status.State == "completed" {
			idleUntil := time.Now().Add(idleDuration)
			for time.Now().Before(idleUntil) {
				<-ticker.C
				w.Tick()
			}
			var memory runtime.MemStats
			runtime.ReadMemStats(&memory)
			rss := benchmarkRSS(b)
			idleRSS += rss
			idleHeap += memory.HeapAlloc
			b.Logf("idle retained_caches=%t active_rss_mib=%.1f active_heap_mib=%.1f idle_rss_mib=%.1f idle_heap_mib=%.1f idle_heap_inuse_mib=%.1f loaded=%d", os.Getenv("VITIS_BENCH_KEEP_CACHES") == "1", float64(activeRSS)/(1<<20), float64(activeMemory.HeapAlloc)/(1<<20), float64(rss)/(1<<20), float64(memory.HeapAlloc)/(1<<20), float64(memory.HeapInuse)/(1<<20), w.chunks.Len())
			runtime.KeepAlive(w)
		}
		ticker.Stop()
		if err := w.Close(context.Background()); err != nil {
			b.Fatal(err)
		}
		if status.State != "completed" || status.Generated != width*width || status.Completed != status.Generated {
			b.Fatalf("incomplete fresh-world generation: %+v", status)
		}
		reopened := persistence.NewChunkStore(storeDir)
		for _, coordinate := range [][2]int{{0, 0}, {minCoordinate, minCoordinate}, {maxCoordinate, maxCoordinate}} {
			encoded, err := reopened.ReadChunkNBT(coordinate[0], coordinate[1])
			if err != nil {
				b.Fatal(err)
			}
			if _, err := chunk.DecodeChunkNBT(encoded, int32(coordinate[0]), int32(coordinate[1]), 0); err != nil {
				b.Fatal(err)
			}
		}
		if err := reopened.Close(); err != nil {
			b.Fatal(err)
		}
		seconds += status.Elapsed.Seconds()
		generated += status.Generated
		stats := terrain.Diagnostics()
		diagnostics.RawGenerations += stats.RawGenerations
		diagnostics.RepeatedRawGenerations += stats.RepeatedRawGenerations
		diagnostics.BaseGenerations += stats.BaseGenerations
		diagnostics.RepeatedBaseGenerations += stats.RepeatedBaseGenerations
		diagnostics.BaseCacheHits += stats.BaseCacheHits
		diagnostics.ClimateSamples += stats.ClimateSamples
		diagnostics.ClimateColumns += stats.ClimateColumns
		diagnostics.ClimateColumnComputations += stats.ClimateColumnComputations
	}
	b.ReportMetric(float64(generated)/seconds, "new_chunks/s")
	b.ReportMetric(float64(workers), "workers")
	if idleDuration > 0 {
		b.ReportMetric(float64(idleRSS)/float64(b.N)/(1<<20), "idle_RSS_MiB")
		b.ReportMetric(float64(idleHeap)/float64(b.N)/(1<<20), "idle_heap_MiB")
	}
	if diagnostics.RawGenerations != 0 {
		b.ReportMetric(float64(savePeak), "save_queue_peak")
		b.ReportMetric(float64(requestPeak), "generation_queue_peak")
		b.ReportMetric(float64(resultPeak), "result_queue_peak")
		b.ReportMetric(float64(saveFullTicks)/float64(diagnosticTicks), "save_queue_full/tick")
		b.ReportMetric(float64(diagnostics.RawGenerations)/float64(generated), "source_chunks/chunk")
		b.ReportMetric(float64(diagnostics.RepeatedRawGenerations)/float64(generated), "repeated_sources/chunk")
		b.ReportMetric(float64(diagnostics.BaseGenerations)/float64(generated), "terrain_bases/chunk")
		b.ReportMetric(float64(diagnostics.RepeatedBaseGenerations)/float64(generated), "repeated_bases/chunk")
		b.ReportMetric(float64(diagnostics.BaseCacheHits)/float64(generated), "base_cache_hits/chunk")
		b.ReportMetric(float64(diagnostics.ClimateSamples)/float64(generated), "climate_samples/chunk")
		b.ReportMetric(float64(diagnostics.ClimateColumns)/float64(generated), "climate_columns/chunk")
		b.ReportMetric(float64(diagnostics.ClimateColumnComputations)/float64(generated), "horizontal_evaluations/chunk")
	}
}

// retainedCacheGenerator isolates live cache retention from heap scavenging.
type retainedCacheGenerator struct{ *chunk.WorldgenGenerator }

// TryReleaseGenerationCaches keeps the original cache lifetime for the baseline.
func (retainedCacheGenerator) TryReleaseGenerationCaches() bool { return true }

// benchmarkRSS reads current Linux resident memory; other platforms report zero.
func benchmarkRSS(b *testing.B) uint64 {
	b.Helper()
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		b.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "VmRSS:" {
			kib, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				b.Fatal(err)
			}
			return kib * 1024
		}
	}
	b.Fatal("process RSS is unavailable")
	return 0
}
