import com.google.gson.Gson;
import com.mojang.serialization.Lifecycle;
import java.lang.reflect.Proxy;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Holder;
import net.minecraft.core.MappedRegistry;
import net.minecraft.core.Registry;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.core.registries.Registries;
import net.minecraft.server.Bootstrap;
import net.minecraft.tags.BlockTags;
import net.minecraft.tags.TagLoader;
import net.minecraft.util.RandomSource;
import net.minecraft.util.valueproviders.UniformInt;
import net.minecraft.world.level.ChunkPos;
import net.minecraft.world.level.LevelHeightAccessor;
import net.minecraft.world.level.WorldGenLevel;
import net.minecraft.world.level.biome.Biome;
import net.minecraft.world.level.biome.BiomeGenerationSettings;
import net.minecraft.world.level.biome.BiomeSpecialEffects;
import net.minecraft.world.level.biome.Biomes;
import net.minecraft.world.level.biome.MobSpawnSettings;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.block.state.properties.Property;
import net.minecraft.world.level.chunk.ProtoChunk;
import net.minecraft.world.level.chunk.UpgradeData;
import net.minecraft.world.level.levelgen.GeodeBlockSettings;
import net.minecraft.world.level.levelgen.GeodeCrackSettings;
import net.minecraft.world.level.levelgen.GeodeLayerSettings;
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.GeodeFeature;
import net.minecraft.world.level.levelgen.feature.configurations.GeodeConfiguration;
import net.minecraft.world.level.levelgen.feature.stateproviders.BlockStateProvider;

/** Exports deterministic geode geometry and random tails from controlled terrain. */
public final class DumpGeodeFixtures {
    /** Runs solid, wet, invalid, and protected cases across two world seeds. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        BuiltInRegistries.BLOCK.prepareTagReload(new TagLoader.LoadResult<>(Registries.BLOCK,
            Map.of(BlockTags.FEATURES_CANNOT_REPLACE, List.<Holder<Block>>of(Blocks.BEDROCK.builtInRegistryHolder()),
                BlockTags.GEODE_INVALID_BLOCKS, List.<Holder<Block>>of(
                    Blocks.BEDROCK.builtInRegistryHolder(), Blocks.WATER.builtInRegistryHolder())))).apply();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        Registry.register(biomes, Biomes.PLAINS, biome);
        biomes.freeze();
        var chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY,
            LevelHeightAccessor.create(-64, 128), biomes, null);
        var blocks = new GeodeBlockSettings(
            BlockStateProvider.simple(Blocks.AIR), BlockStateProvider.simple(Blocks.AMETHYST_BLOCK),
            BlockStateProvider.simple(Blocks.BUDDING_AMETHYST), BlockStateProvider.simple(Blocks.CALCITE),
            BlockStateProvider.simple(Blocks.SMOOTH_BASALT),
            List.of(Blocks.SMALL_AMETHYST_BUD.defaultBlockState(), Blocks.MEDIUM_AMETHYST_BUD.defaultBlockState(),
                Blocks.LARGE_AMETHYST_BUD.defaultBlockState(), Blocks.AMETHYST_CLUSTER.defaultBlockState()),
            BlockTags.FEATURES_CANNOT_REPLACE, BlockTags.GEODE_INVALID_BLOCKS);
        var config = new GeodeConfiguration(blocks, new GeodeLayerSettings(1.7, 2.2, 3.2, 4.2),
            new GeodeCrackSettings(.95, 2.0, 2), .35, .083, true,
            UniformInt.of(4, 6), UniformInt.of(3, 4), UniformInt.of(1, 2), -16, 16, .05, 1);
        var cases = new ArrayList<Map<String, Object>>();
        for (int shape = 0; shape < 4; shape++) {
            for (long worldSeed : new long[] {0, 17}) {
                for (long randomSeed : new long[] {0, 1}) {
                    cases.add(run(chunk, config, new BlockPos(15, -20, -1), shape, worldSeed, randomSeed));
                }
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(cases));
    }

    /** Returns a simple world state before geode placement. */
    private static BlockState initial(BlockPos origin, int shape, BlockPos position) {
        int x = position.getX() - origin.getX();
        int y = position.getY() - origin.getY();
        int z = position.getZ() - origin.getZ();
        if (shape == 2) return Blocks.AIR.defaultBlockState();
        if (shape == 1 && x >= -3 && x <= 3 && y >= -3 && y <= 3 && z >= -3 && z <= 3) {
            return y == 0 ? Blocks.WATER.defaultBlockState() : Blocks.AIR.defaultBlockState();
        }
        if (shape == 3 && x >= 4 && x <= 6 && y >= 4 && y <= 6 && z >= 4 && z <= 6) {
            return Blocks.BEDROCK.defaultBlockState();
        }
        return Blocks.STONE.defaultBlockState();
    }

    /** Places a geode and records final changed states and the next random value. */
    private static Map<String, Object> run(ProtoChunk chunk, GeodeConfiguration config, BlockPos origin,
                                            int shape, long worldSeed, long randomSeed) {
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
                    case "scheduleTick": return null;
                    case "getSeed": return worldSeed;
                    case "getChunk": return chunk;
                    case "getMinY": return -64;
                    case "getMaxY": return 63;
                    case "ensureCanWrite": return true;
                    default: throw new UnsupportedOperationException(method.toString());
                }
            });
        RandomSource random = RandomSource.create(randomSeed);
        boolean placed = new GeodeFeature(GeodeConfiguration.CODEC)
            .place(new FeaturePlaceContext<>(java.util.Optional.empty(), world, null, random, origin, config));
        var changed = new HashMap<BlockPos, BlockState>();
        writes.forEach((position, state) -> {
            if (state != initial(origin, shape, position)) changed.put(position, state);
        });
        var result = new LinkedHashMap<String, Object>();
        result.put("shape", shape);
        result.put("worldSeed", worldSeed);
        result.put("seed", randomSeed);
        result.put("origin", new int[] {origin.getX(), origin.getY(), origin.getZ()});
        result.put("placed", placed);
        result.put("tail", random.nextLong());
        result.put("changes", states(changed));
        return result;
    }

    /** Serializes states in stable x, z, y order. */
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
                row.put("name", BuiltInRegistries.BLOCK.getKey(item.getValue().getBlock()).toString());
                row.put("properties", properties);
                result.add(row);
            });
        return result;
    }

    /** Converts a property value to its canonical name. */
    @SuppressWarnings({"rawtypes", "unchecked"})
    private static String serialized(Property property, Comparable value) { return property.getName(value); }
}
