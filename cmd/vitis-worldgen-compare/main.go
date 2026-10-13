// Command vitis-worldgen-compare checks generated chunks against a saved Minecraft world.
package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/region"
	"github.com/vitismc/vitis/internal/world/worldgen"
)

type comparison struct {
	blocks, differentBlocks, differentAir, biomes, differentBiomes int
	differentInterior                                              int
}

// main validates arguments and reports comparison errors.
func main() {
	worldPath := flag.String("world", "", "saved world directory containing level.dat")
	version := flag.String("version", protocol.CurrentMinecraftVersion, "Minecraft version used to select the default resource pack")
	resourcesPath := flag.String("resources", "", "server resource pack (default: .mcdata/<version>/vanilla)")
	dimension := flag.String("dimension", "overworld", "overworld, nether, or end")
	x := flag.Int("x", 0, "chunk X coordinate")
	z := flag.Int("z", 0, "chunk Z coordinate")
	top := flag.Int("top", 0, "show this many most common state mismatches")
	flag.Parse()
	if *resourcesPath == "" {
		*resourcesPath = filepath.Join(".mcdata", *version, "vanilla")
	}
	if *worldPath == "" {
		fmt.Fprintln(os.Stderr, "-world is required")
		os.Exit(2)
	}
	if err := run(*worldPath, *resourcesPath, *dimension, int32(*x), int32(*z), *top); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run loads one saved chunk and compares its block states and biome IDs.
func run(worldPath, resourcesPath, dimension string, x, z int32, top int) error {
	seed, err := readSeed(filepath.Join(worldPath, "level.dat"))
	if err != nil {
		return err
	}
	var folder, settings string
	switch dimension {
	case "overworld":
		folder, settings = worldPath, "minecraft:overworld"
	case "nether":
		folder, settings = filepath.Join(worldPath, "DIM-1"), "minecraft:nether"
	case "end":
		folder, settings = filepath.Join(worldPath, "DIM1"), "minecraft:end"
	default:
		return fmt.Errorf("unknown dimension %q", dimension)
	}
	regionPath := filepath.Join(folder, "region", fmt.Sprintf("r.%d.%d.mca", x>>5, z>>5))
	file, err := region.Open(regionPath)
	if err != nil {
		return fmt.Errorf("open region: %w", err)
	}
	defer file.Close()
	raw, err := file.ReadChunkNBT(int(x&31), int(z&31))
	if err != nil {
		return fmt.Errorf("read chunk (%d,%d): %w", x, z, err)
	}
	_, root, err := nbt.NewDecoder(raw).ReadNamedRootCompound()
	if err != nil {
		return fmt.Errorf("decode chunk NBT: %w", err)
	}
	version, _ := root.GetInt("DataVersion")
	if version != protocol.CurrentDataVersion {
		return fmt.Errorf("chunk data version %d, expected %d", version, protocol.CurrentDataVersion)
	}
	status, _ := root.GetString("Status")
	if status != "minecraft:full" && status != "full" {
		return fmt.Errorf("chunk (%d,%d) is at %q, expected full", x, z, status)
	}
	actual, err := chunk.DecodeChunkNBT(raw, x, z, 0)
	if err != nil {
		return fmt.Errorf("decode saved chunk: %w", err)
	}
	manager := &datapack.Manager{}
	runtime, err := worldgen.NewRuntime(manager)
	if err != nil {
		return err
	}
	if err := manager.Reload([]string{resourcesPath}); err != nil {
		return fmt.Errorf("load resources: %w", err)
	}
	generator, err := runtime.NewTerrainGenerator(settings, seed, 0)
	if err != nil {
		return err
	}
	generated, err := generator.Generate(x, z)
	if err != nil {
		return err
	}
	result, err := compare(actual.Data(), generated)
	if err != nil {
		return err
	}
	fmt.Printf("seed=%d dimension=%s chunk=(%d,%d) blocks=%d different=%d interior=%d air=%d biomes=%d different=%d\n",
		seed, dimension, x, z, result.blocks, result.differentBlocks, result.differentInterior, result.differentAir, result.biomes, result.differentBiomes)
	if top > 0 {
		for _, mismatch := range commonMismatches(actual.Data(), generated, top) {
			fmt.Printf("%d %s -> %s\n", mismatch.count, block.NameFromState(mismatch.actual), block.NameFromState(mismatch.generated))
		}
		fmt.Println("interior (x,z 4..11):")
		for _, mismatch := range commonInteriorMismatches(actual.Data(), generated, top) {
			fmt.Printf("%d %s -> %s\n", mismatch.count, block.NameFromState(mismatch.actual), block.NameFromState(mismatch.generated))
		}
	}
	if result.differentBlocks != 0 || result.differentBiomes != 0 {
		return errors.New("generated chunk differs from saved chunk")
	}
	return nil
}

type stateMismatch struct {
	actual, generated int32
	count             int
}

// commonMismatches returns the most frequent state differences in stable order.
func commonMismatches(actual, generated *section.Chunk, limit int) []stateMismatch {
	return commonMismatchesInRange(actual, generated, limit, 0, 16)
}

// commonInteriorMismatches reports differences away from horizontal chunk edges.
func commonInteriorMismatches(actual, generated *section.Chunk, limit int) []stateMismatch {
	return commonMismatchesInRange(actual, generated, limit, 4, 12)
}

func commonMismatchesInRange(actual, generated *section.Chunk, limit, start, end int) []stateMismatch {
	counts := make(map[[2]int32]int)
	for y := actual.MinY(); y <= actual.MaxY(); y++ {
		for z := start; z < end; z++ {
			for x := start; x < end; x++ {
				left, right := actual.GetBlock(x, y, z), generated.GetBlock(x, y, z)
				if left != right {
					counts[[2]int32{left, right}]++
				}
			}
		}
	}
	result := make([]stateMismatch, 0, len(counts))
	for pair, count := range counts {
		result = append(result, stateMismatch{actual: pair[0], generated: pair[1], count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].count != result[j].count {
			return result[i].count > result[j].count
		}
		if result[i].actual != result[j].actual {
			return result[i].actual < result[j].actual
		}
		return result[i].generated < result[j].generated
	})
	return result[:min(limit, len(result))]
}

// readSeed reads the world seed from a compressed level.dat file.
func readSeed(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open level.dat: %w", err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return 0, fmt.Errorf("decompress level.dat: %w", err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, (16<<20)+1))
	if err != nil {
		return 0, fmt.Errorf("read level.dat: %w", err)
	}
	if len(raw) > 16<<20 {
		return 0, errors.New("level.dat exceeds size limit")
	}
	_, root, err := nbt.NewDecoder(raw).ReadNamedRootCompound()
	if err != nil {
		return 0, fmt.Errorf("decode level.dat: %w", err)
	}
	data, ok := root.GetCompound("Data")
	if !ok {
		return 0, errors.New("level.dat has no Data compound")
	}
	settings, ok := data.GetCompound("WorldGenSettings")
	if !ok {
		return 0, errors.New("level.dat has no WorldGenSettings")
	}
	seed, ok := settings.GetLong("seed")
	if !ok {
		return 0, errors.New("level.dat has no world seed")
	}
	return seed, nil
}

// compare counts state and biome differences between chunks with equal bounds.
func compare(actual, generated *section.Chunk) (comparison, error) {
	if actual == nil || generated == nil || actual.MinY() != generated.MinY() || actual.MaxY() != generated.MaxY() || len(actual.Sections) != len(generated.Sections) {
		return comparison{}, errors.New("chunk bounds differ")
	}
	var result comparison
	for y := actual.MinY(); y <= actual.MaxY(); y++ {
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				result.blocks++
				left, right := actual.GetBlock(x, y, z), generated.GetBlock(x, y, z)
				if left != right {
					result.differentBlocks++
					if x >= 4 && x < 12 && z >= 4 && z < 12 {
						result.differentInterior++
					}
					if block.IsAir(left) != block.IsAir(right) {
						result.differentAir++
					}
				}
			}
		}
	}
	for i, left := range actual.Sections {
		right := generated.Sections[i]
		for y := 0; y < 4; y++ {
			for z := 0; z < 4; z++ {
				for x := 0; x < 4; x++ {
					result.biomes++
					if left.GetBiome(x, y, z) != right.GetBiome(x, y, z) {
						result.differentBiomes++
					}
				}
			}
		}
	}
	return result, nil
}
