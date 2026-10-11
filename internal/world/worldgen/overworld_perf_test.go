package worldgen

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
)

func TestOverworldGenerationSmoke(t *testing.T) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		t.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 123456789, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Generate(0, 0); err != nil {
		t.Fatal(err)
	}
}

func TestBenchmarkChunkCoordinatesAreUnique(t *testing.T) {
	seen := make(map[[2]int32]struct{})
	for index := int32(0); index < 4096; index++ {
		x, z := benchmarkChunkCoordinates(index)
		key := [2]int32{x, z}
		if _, exists := seen[key]; exists {
			t.Fatalf("coordinate %v repeats at index %d", key, index)
		}
		seen[key] = struct{}{}
	}
}

func BenchmarkOverworldChunkGeneration(b *testing.B) {
	benchmarkOverworldChunkGeneration(b, false, 123456789)
}

// BenchmarkOverworldTerrainThroughCarvers isolates biome, noise, surface, and carver stages.
func BenchmarkOverworldTerrainThroughCarvers(b *testing.B) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 0, 0)
	if err != nil {
		b.Fatal(err)
	}
	snapshot, err := generator.snapshot()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var last *section.Chunk
	for range b.N {
		last = generateTerrainThroughCarvers(generator, snapshot, 0, 0)
	}
	benchmarkTerrainResult = last
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "chunks/s")
}

var benchmarkTerrainResult *section.Chunk

// BenchmarkOverworldTerrainThroughCarversBatch16 measures 16 adjacent chunks with eight workers.
func BenchmarkOverworldTerrainThroughCarversBatch16(b *testing.B) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 2_965_282_071_327_931_563, 0)
	if err != nil {
		b.Fatal(err)
	}
	snapshot, err := generator.snapshot()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		var workers sync.WaitGroup
		var results [16]*section.Chunk
		workers.Add(8)
		for worker := 0; worker < 8; worker++ {
			go func(worker int) {
				defer workers.Done()
				for offset := 0; offset < 2; offset++ {
					index := worker*2 + offset
					results[index] = generateTerrainThroughCarvers(generator, snapshot, int32(index%4-1), int32(index/4-1))
				}
			}(worker)
		}
		workers.Wait()
		benchmarkTerrainResult = results[0]
	}
	b.ReportMetric(float64(b.N*16)/b.Elapsed().Seconds(), "chunks/s")
}

// generateTerrainThroughCarvers executes only the terrain stages measured by the stage benchmark.
func generateTerrainThroughCarvers(generator *TerrainGenerator, snapshot *terrainSnapshot, x, z int32) *section.Chunk {
	result := section.NewChunkWithBounds(x, z, snapshot.minY, snapshot.height, 0)
	biomes := newCachedBiomeSource(snapshot.biomes)
	baseX, baseZ := int(x)*16, int(z)*16
	generator.generateTerrainRange(snapshot, result, baseX, baseZ, snapshot.minY, snapshot.minY+snapshot.height)
	populateBiomes(result, biomes, x, z)
	snapshot.surface.apply(result, x, z, biomes)
	snapshot.carvers.apply(result, x, z)
	return result
}

func BenchmarkOverworldChunkGenerationParallel(b *testing.B) {
	benchmarkOverworldChunkGeneration(b, true, 123456789)
}

// BenchmarkOverworldRawGenerationParallel measures source chunks before border assembly.
func BenchmarkOverworldRawGenerationParallel(b *testing.B) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 123456789, 0)
	if err != nil {
		b.Fatal(err)
	}
	snapshot, err := generator.snapshot()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var coordinate atomic.Int32
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			index := coordinate.Add(1) - 1
			x, z := benchmarkChunkCoordinates(index)
			if _, err := generator.generateRaw(snapshot, x, z); err != nil {
				b.Error(err)
			}
		}
	})
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "chunks/s")
}

// BenchmarkOverworldChunkGenerationReferenceSeedParallel isolates seed effects.
func BenchmarkOverworldChunkGenerationReferenceSeedParallel(b *testing.B) {
	benchmarkOverworldChunkGeneration(b, true, 2_965_282_071_327_931_563)
}

// BenchmarkOverworldChunkBatch16 measures a contiguous 4x4 region with eight workers.
func BenchmarkOverworldChunkBatch16(b *testing.B) {
	benchmarkOverworldChunkBatch16(b, false, -1, -1)
}

// BenchmarkOverworldChunkBatch16Complete includes lighting and chunk setup.
func BenchmarkOverworldChunkBatch16Complete(b *testing.B) {
	benchmarkOverworldChunkBatch16(b, true, -1, -1)
}

// BenchmarkOverworldChunkBatch16FarComplete measures a separate 4x4 region.
func BenchmarkOverworldChunkBatch16FarComplete(b *testing.B) {
	benchmarkOverworldChunkBatch16(b, true, 256, -256)
}

// benchmarkOverworldChunkBatch16 times distinct adjacent chunks without packet delivery.
func benchmarkOverworldChunkBatch16(b *testing.B, complete bool, startX, startZ int32) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", 2_965_282_071_327_931_563, 0)
	if err != nil {
		b.Fatal(err)
	}
	var generate func(int32, int32) error
	if complete {
		fullGenerator, createErr := chunk.NewWorldgenGenerator(generator)
		if createErr != nil {
			b.Fatal(createErr)
		}
		generate = func(x, z int32) error {
			_, generateErr := fullGenerator.Generate(x, z)
			return generateErr
		}
	} else {
		generate = func(x, z int32) error {
			_, generateErr := generator.Generate(x, z)
			return generateErr
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for batch := range b.N {
		batchX := startX + int32(batch%8)*4
		batchZ := startZ + int32(batch/8)*4
		var workers sync.WaitGroup
		workers.Add(8)
		for worker := 0; worker < 8; worker++ {
			go func(worker int) {
				defer workers.Done()
				for offset := 0; offset < 2; offset++ {
					index := worker*2 + offset
					x := batchX + int32(index%4)
					z := batchZ + int32(index/4)
					if generateErr := generate(x, z); generateErr != nil {
						b.Error(generateErr)
					}
				}
			}(worker)
		}
		workers.Wait()
	}
	b.ReportMetric(float64(b.N*16)/b.Elapsed().Seconds(), "chunks/s")
}

// BenchmarkOverworldChunkGenerationCompleteParallel includes chunk lighting and lifecycle setup.
func BenchmarkOverworldChunkGenerationCompleteParallel(b *testing.B) {
	benchmarkOverworldChunkComplete(b, false, false)
}

// BenchmarkOverworldChunkGenerateAndFrameParallel includes packet encoding and compression.
func BenchmarkOverworldChunkGenerateAndFrameParallel(b *testing.B) {
	benchmarkOverworldChunkComplete(b, true, false)
}

// BenchmarkOverworldChunkFullPathParallel includes delivery over a local TCP stream.
func BenchmarkOverworldChunkFullPathParallel(b *testing.B) {
	benchmarkOverworldChunkComplete(b, true, true)
}

// benchmarkOverworldChunkComplete measures chunk generation and optional framing and delivery.
func benchmarkOverworldChunkComplete(b *testing.B, framePacket, sendPacket bool) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	terrain, err := runtime.NewTerrainGenerator("minecraft:overworld", 123456789, 0)
	if err != nil {
		b.Fatal(err)
	}
	generator, err := chunk.NewWorldgenGenerator(terrain)
	if err != nil {
		b.Fatal(err)
	}
	var listener net.Listener
	var acceptDone chan struct{}
	var readers sync.WaitGroup
	if sendPacket {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			b.Fatal(err)
		}
		acceptDone = make(chan struct{})
		go func() {
			defer close(acceptDone)
			for {
				connection, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				readers.Add(1)
				go func() {
					defer readers.Done()
					defer connection.Close()
					if _, copyErr := io.Copy(io.Discard, connection); copyErr != nil {
						b.Error(copyErr)
					}
				}()
			}
		}()
	}
	b.ReportAllocs()
	b.ResetTimer()
	var coordinate atomic.Int32
	b.RunParallel(func(pb *testing.PB) {
		var connection net.Conn
		if sendPacket {
			var dialErr error
			connection, dialErr = net.Dial("tcp", listener.Addr().String())
			if dialErr != nil {
				b.Error(dialErr)
				return
			}
			defer connection.Close()
		}
		packetBuffer := protocol.NewBuffer(16384)
		frameBuffer := protocol.NewBuffer(16384)
		for pb.Next() {
			index := coordinate.Add(1) - 1
			x, z := benchmarkChunkCoordinates(index)
			generated, err := generator.Generate(x, z)
			if err != nil {
				b.Error(err)
				continue
			}
			if !framePacket {
				continue
			}
			packetBuffer.Reset()
			packet := &play.ChunkDataAndUpdateLight{ChunkX: x, ChunkZ: z, Payload: generated.EncodePacketPayload()}
			if err := packet.Encode(packetBuffer); err != nil {
				b.Error(err)
				continue
			}
			frameBuffer.Reset()
			frameBuffer.WriteVarInt(packet.ID())
			frameBuffer.WriteBytes(packetBuffer.Bytes())
			frame, err := network.CompressFrame(frameBuffer.Bytes(), 256)
			if err != nil {
				b.Error(err)
				continue
			}
			if sendPacket {
				for len(frame) > 0 {
					written, writeErr := connection.Write(frame)
					if writeErr != nil {
						b.Error(writeErr)
						return
					}
					if written == 0 {
						b.Error(io.ErrNoProgress)
						return
					}
					frame = frame[written:]
				}
			}
		}
	})
	if sendPacket {
		listener.Close()
		<-acceptDone
		readers.Wait()
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "chunks/s")
}

func benchmarkOverworldChunkGeneration(b *testing.B, parallel bool, seed int64) {
	resources := &datapack.Manager{}
	runtime, err := NewRuntime(resources)
	if err != nil {
		b.Fatal(err)
	}
	if err := resources.Reload([]string{"../../../.mcdata/1.21.4/vanilla"}); err != nil {
		b.Fatal(err)
	}
	generator, err := runtime.NewTerrainGenerator("minecraft:overworld", seed, 0)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	if parallel {
		var coordinate atomic.Int32
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				index := coordinate.Add(1) - 1
				x, z := benchmarkChunkCoordinates(index)
				if _, err := generator.Generate(x, z); err != nil {
					b.Error(err)
				}
			}
		})
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "chunks/s")
		return
	}
	for i := 0; i < b.N; i++ {
		if _, err := generator.Generate(int32(i), 0); err != nil {
			b.Fatal(err)
		}
	}
}

// benchmarkChunkCoordinates assigns each iteration a unique nearby chunk.
func benchmarkChunkCoordinates(index int32) (int32, int32) {
	return (index&31 - 16) + (index>>10)*32, index>>5&31 - 16
}
