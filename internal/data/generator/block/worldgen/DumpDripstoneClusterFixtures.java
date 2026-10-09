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
import java.util.Optional;
import java.util.function.Predicate;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Holder;
import net.minecraft.core.MappedRegistry;
import net.minecraft.core.Registry;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.core.registries.Registries;
import net.minecraft.server.Bootstrap;
import net.minecraft.tags.BlockTags;
import net.minecraft.tags.FluidTags;
import net.minecraft.tags.TagLoader;
import net.minecraft.util.RandomSource;
import net.minecraft.util.valueproviders.ClampedNormalFloat;
import net.minecraft.util.valueproviders.UniformFloat;
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
import net.minecraft.world.level.material.Fluid;
import net.minecraft.world.level.material.Fluids;
import net.minecraft.world.level.levelgen.feature.DripstoneClusterFeature;
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.configurations.DripstoneClusterConfiguration;

/** Exports cave-column geometry and RNG tails for the versioned cluster feature. */
public final class DumpDripstoneClusterFixtures {
    /** Runs the shipped configuration against caves with one or both boundaries. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        List<Holder<Block>> stone = List.of(Blocks.STONE.builtInRegistryHolder());
        BuiltInRegistries.BLOCK.prepareTagReload(new TagLoader.LoadResult<>(Registries.BLOCK,
            Map.of(BlockTags.DRIPSTONE_REPLACEABLE, stone, BlockTags.BASE_STONE_OVERWORLD, stone))).apply();
        List<Holder<Fluid>> water = List.of(Fluids.WATER.builtInRegistryHolder(), Fluids.FLOWING_WATER.builtInRegistryHolder());
        BuiltInRegistries.FLUID.prepareTagReload(new TagLoader.LoadResult<>(Registries.FLUID,
            Map.of(FluidTags.WATER, water))).apply();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        Registry.register(biomes, Biomes.PLAINS, biome);
        biomes.freeze();
        var chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY,
            LevelHeightAccessor.create(-64, 128), biomes, null);
        var config = new DripstoneClusterConfiguration(12, UniformInt.of(3, 6), UniformInt.of(2, 8),
            1, 3, UniformInt.of(2, 4), UniformFloat.of(.3F, .7F), ClampedNormalFloat.of(.1F, .3F, .1F, .9F),
            .1F, 3, 8);
        var fixtures = new ArrayList<Map<String, Object>>();
        for (int shape = 0; shape < 4; shape++) {
            for (long seed = 0; seed < 4; seed++) {
                fixtures.add(run(chunk, config, new BlockPos(15, -20, -1), shape, seed));
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(fixtures));
    }

    /** Returns the controlled raw cave state before feature writes. */
    private static BlockState initial(BlockPos origin, int shape, BlockPos pos) {
        if (shape == 3 && pos.equals(origin)) return Blocks.STONE.defaultBlockState();
        if (Math.abs(pos.getX() - origin.getX()) > 10 || Math.abs(pos.getZ() - origin.getZ()) > 10) {
            return Blocks.AIR.defaultBlockState();
        }
        int y = pos.getY() - origin.getY();
        if ((shape == 0 || shape == 1 || shape == 3) && y >= 5 && y <= 9) return Blocks.STONE.defaultBlockState();
        if ((shape == 0 || shape == 2 || shape == 3) && y <= -5 && y >= -9) return Blocks.STONE.defaultBlockState();
        return Blocks.AIR.defaultBlockState();
    }

    /** Records changed blocks and the next RNG value after feature placement. */
    private static Map<String, Object> run(ProtoChunk chunk, DripstoneClusterConfiguration config, BlockPos origin, int shape, long seed) {
        var terrain = new HashMap<BlockPos, BlockState>();
        var writes = new HashMap<BlockPos, BlockState>();
        WorldGenLevel world = (WorldGenLevel) Proxy.newProxyInstance(WorldGenLevel.class.getClassLoader(),
            new Class<?>[] {WorldGenLevel.class}, (proxy, method, args) -> {
                switch (method.getName()) {
                    case "getBlockState":
                        BlockPos read = (BlockPos) args[0];
                        return terrain.getOrDefault(read, initial(origin, shape, read));
                    case "isStateAtPosition":
                        BlockPos tested = (BlockPos) args[0];
                        return ((Predicate<BlockState>) args[1]).test(terrain.getOrDefault(tested, initial(origin, shape, tested)));
                    case "isWaterAt":
                        BlockPos wet = (BlockPos) args[0];
                        return terrain.getOrDefault(wet, initial(origin, shape, wet)).getBlock() == Blocks.WATER;
                    case "setBlock":
                        BlockPos pos = ((BlockPos) args[0]).immutable();
                        terrain.put(pos, (BlockState) args[1]);
                        writes.put(pos, (BlockState) args[1]);
                        return true;
                    case "getChunk": return chunk;
                    case "getMinY": return -64;
                    case "getMaxY": return 63;
                    case "getHeight": return 128;
                    case "getSeed": return 0L;
                    case "ensureCanWrite": return true;
                    default: throw new UnsupportedOperationException(method.toString());
                }
            });
        RandomSource random = RandomSource.create(seed);
        boolean placed = new DripstoneClusterFeature(DripstoneClusterConfiguration.CODEC)
            .place(new FeaturePlaceContext<>(Optional.empty(), world, null, random, origin, config));
        var changed = new HashMap<BlockPos, BlockState>();
        writes.forEach((pos, state) -> {
            if (state != initial(origin, shape, pos)) changed.put(pos, state);
        });
        var result = new LinkedHashMap<String, Object>();
        result.put("shape", shape);
        result.put("seed", seed);
        result.put("origin", new int[] {origin.getX(), origin.getY(), origin.getZ()});
        result.put("placed", placed);
        result.put("tail", random.nextLong());
        result.put("changes", states(changed));
        return result;
    }

    /** Serializes states in stable coordinate order with canonical properties. */
    private static List<Map<String, Object>> states(Map<BlockPos, BlockState> states) {
        var result = new ArrayList<Map<String, Object>>();
        states.entrySet().stream()
            .sorted(Comparator.comparingInt((Map.Entry<BlockPos, BlockState> entry) -> entry.getKey().getX())
                .thenComparingInt(entry -> entry.getKey().getZ()).thenComparingInt(entry -> entry.getKey().getY()))
            .forEach(entry -> {
                var properties = new LinkedHashMap<String, String>();
                entry.getValue().getValues().forEach((property, value) -> properties.put(property.getName(), serialized(property, value)));
                var row = new LinkedHashMap<String, Object>();
                row.put("position", new int[] {entry.getKey().getX(), entry.getKey().getY(), entry.getKey().getZ()});
                row.put("name", BuiltInRegistries.BLOCK.getKey(entry.getValue().getBlock()).toString());
                row.put("properties", properties);
                result.add(row);
            });
        return result;
    }

    /** Converts registry property values to their serialized form. */
    @SuppressWarnings({"rawtypes", "unchecked"})
    private static String serialized(Property property, Comparable value) { return property.getName(value); }
}
