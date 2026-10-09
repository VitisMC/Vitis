import com.google.gson.Gson;
import com.mojang.serialization.Lifecycle;
import com.mojang.serialization.MapCodec;
import java.lang.reflect.Proxy;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.MappedRegistry;
import net.minecraft.core.Registry;
import net.minecraft.core.registries.Registries;
import net.minecraft.server.Bootstrap;
import net.minecraft.server.level.WorldGenRegion;
import net.minecraft.util.RandomSource;
import net.minecraft.world.level.ChunkPos;
import net.minecraft.world.level.LevelHeightAccessor;
import net.minecraft.world.level.NoiseColumn;
import net.minecraft.world.level.StructureManager;
import net.minecraft.world.level.WorldGenLevel;
import net.minecraft.world.level.biome.Biome;
import net.minecraft.world.level.biome.BiomeGenerationSettings;
import net.minecraft.world.level.biome.BiomeManager;
import net.minecraft.world.level.biome.BiomeSpecialEffects;
import net.minecraft.world.level.biome.FixedBiomeSource;
import net.minecraft.world.level.biome.MobSpawnSettings;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.block.state.properties.Property;
import net.minecraft.world.level.chunk.ChunkAccess;
import net.minecraft.world.level.chunk.ChunkGenerator;
import net.minecraft.world.level.chunk.ProtoChunk;
import net.minecraft.world.level.chunk.UpgradeData;
import net.minecraft.world.level.levelgen.Heightmap;
import net.minecraft.world.level.levelgen.RandomState;
import net.minecraft.world.level.levelgen.blending.Blender;
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.IcebergFeature;
import net.minecraft.world.level.levelgen.feature.configurations.BlockStateConfiguration;

/** Exports deterministic icebergs on controlled ocean and ice-covered terrain. */
public final class DumpIcebergFixtures {
    /** Runs both configured materials across boundary positions and random seeds. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        var holder = Registry.registerForHolder(biomes, net.minecraft.world.level.biome.Biomes.PLAINS, biome);
        biomes.freeze();
        var chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY,
            LevelHeightAccessor.create(-64, 192), biomes, null);
        var generator = new SeaLevelGenerator(holder);
        var cases = new ArrayList<Map<String, Object>>();
        for (BlockState material : List.of(Blocks.BLUE_ICE.defaultBlockState(), Blocks.PACKED_ICE.defaultBlockState())) {
            for (int shape = 0; shape < 2; shape++) {
                for (long seed : new long[] {0, 2, 119, 512}) {
                    cases.add(run(chunk, generator, material, new BlockPos(15, 40, -1), shape, seed));
                }
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(cases));
    }

    /** Supplies the original ocean before any feature writes. */
    private static BlockState initial(BlockPos origin, int shape, BlockPos position) {
        if (position.getY() <= 59) return Blocks.STONE.defaultBlockState();
        if (position.getY() <= 63) return Blocks.WATER.defaultBlockState();
        if (shape == 1 && position.getY() == 64 &&
            Math.abs(position.getX() - origin.getX()) <= 3 &&
            Math.abs(position.getZ() - origin.getZ()) <= 3) return Blocks.ICE.defaultBlockState();
        return Blocks.AIR.defaultBlockState();
    }

    /** Places one feature and records changed states and the subsequent random value. */
    private static Map<String, Object> run(ProtoChunk chunk, ChunkGenerator generator,
                                            BlockState material, BlockPos origin, int shape, long seed) {
        var terrain = new HashMap<BlockPos, BlockState>();
        var writes = new HashMap<BlockPos, BlockState>();
        WorldGenLevel world = (WorldGenLevel) Proxy.newProxyInstance(WorldGenLevel.class.getClassLoader(),
            new Class<?>[] {WorldGenLevel.class}, (proxy, method, args) -> {
                switch (method.getName()) {
                    case "getBlockState": {
                        BlockPos position = (BlockPos) args[0];
                        return terrain.getOrDefault(position, initial(origin, shape, position));
                    }
                    case "getFluidState": {
                        BlockPos position = (BlockPos) args[0];
                        return terrain.getOrDefault(position, initial(origin, shape, position)).getFluidState();
                    }
                    case "setBlock": {
                        BlockPos position = ((BlockPos) args[0]).immutable();
                        BlockState state = (BlockState) args[1];
                        terrain.put(position, state);
                        writes.put(position, state);
                        return true;
                    }
                    case "getMinY": return -64;
                    case "getMaxY": return 127;
                    case "getChunk": return chunk;
                    case "ensureCanWrite": return true;
                    default: throw new UnsupportedOperationException(method.toString());
                }
            });
        RandomSource random = RandomSource.create(seed);
        boolean placed = new IcebergFeature(BlockStateConfiguration.CODEC)
            .place(new FeaturePlaceContext<>(java.util.Optional.empty(), world, generator, random,
                origin, new BlockStateConfiguration(material)));
        var result = new LinkedHashMap<String, Object>();
        result.put("material", net.minecraft.core.registries.BuiltInRegistries.BLOCK.getKey(material.getBlock()).toString());
        result.put("shape", shape);
        result.put("seed", seed);
        result.put("origin", new int[] {origin.getX(), origin.getY(), origin.getZ()});
        result.put("placed", placed);
        result.put("tail", random.nextLong());
        var changes = new HashMap<BlockPos, BlockState>();
        writes.forEach((position, state) -> {
            if (state != initial(origin, shape, position)) changes.put(position, state);
        });
        result.put("changes", states(changes));
        return result;
    }

    /** Serializes states in stable coordinate order. */
    private static List<Map<String, Object>> states(Map<BlockPos, BlockState> blocks) {
        var result = new ArrayList<Map<String, Object>>();
        blocks.entrySet().stream()
            .sorted(Comparator.comparingInt((Map.Entry<BlockPos, BlockState> item) -> item.getKey().getX())
                .thenComparingInt(item -> item.getKey().getZ()).thenComparingInt(item -> item.getKey().getY()))
            .forEach(item -> {
                var properties = new LinkedHashMap<String, String>();
                item.getValue().getValues().forEach((property, value) -> properties.put(property.getName(), serialized(property, value)));
                var row = new LinkedHashMap<String, Object>();
                row.put("position", new int[] {item.getKey().getX(), item.getKey().getY(), item.getKey().getZ()});
                row.put("name", net.minecraft.core.registries.BuiltInRegistries.BLOCK.getKey(item.getValue().getBlock()).toString());
                row.put("properties", properties);
                result.add(row);
            });
        return result;
    }

    /** Converts one block property to its canonical name. */
    @SuppressWarnings({"rawtypes", "unchecked"})
    private static String serialized(Property property, Comparable value) { return property.getName(value); }

    /** Provides the sea level used by the feature without generating terrain. */
    private static final class SeaLevelGenerator extends ChunkGenerator {
        SeaLevelGenerator(net.minecraft.core.Holder<Biome> biome) { super(new FixedBiomeSource(biome)); }
        @Override protected MapCodec<? extends ChunkGenerator> codec() { return null; }
        @Override public void applyCarvers(WorldGenRegion region, long seed, RandomState randomState,
            BiomeManager biomes, StructureManager structures, ChunkAccess chunk) {}
        @Override public void buildSurface(WorldGenRegion region, StructureManager structures,
            RandomState randomState, ChunkAccess chunk) {}
        @Override public void spawnOriginalMobs(WorldGenRegion region) {}
        @Override public int getGenDepth() { return 192; }
        @Override public int getSeaLevel() { return 63; }
        @Override public int getMinY() { return -64; }
        @Override public int getBaseHeight(int x, int z, Heightmap.Types type,
            LevelHeightAccessor world, RandomState randomState) { return 63; }
        @Override public NoiseColumn getBaseColumn(int x, int z, LevelHeightAccessor world, RandomState randomState) {
            return new NoiseColumn(-64, new BlockState[192]);
        }
        @Override public CompletableFuture<ChunkAccess> fillFromNoise(Blender blender, RandomState randomState,
            StructureManager structures, ChunkAccess chunk) { return CompletableFuture.completedFuture(chunk); }
        @Override public void addDebugScreenInfo(List<String> lines, RandomState state, BlockPos position) {}
    }
}
