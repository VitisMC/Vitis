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
import net.minecraft.tags.TagLoader;
import net.minecraft.util.RandomSource;
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
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.PointedDripstoneFeature;
import net.minecraft.world.level.levelgen.feature.configurations.PointedDripstoneConfiguration;

/** Exports small dripstone placement and random tails from the versioned feature. */
public final class DumpPointedDripstoneFixtures {
    /** Executes supported and rejected sites with mixed direction and water cases. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        List<Holder<Block>> replaceable = List.of(Blocks.STONE.builtInRegistryHolder());
        BuiltInRegistries.BLOCK.prepareTagReload(new TagLoader.LoadResult<>(Registries.BLOCK,
            Map.of(BlockTags.DRIPSTONE_REPLACEABLE, replaceable))).apply();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        Registry.register(biomes, Biomes.PLAINS, biome);
        biomes.freeze();
        var chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY,
            LevelHeightAccessor.create(-64, 128), biomes, null);
        var fixtures = new ArrayList<Map<String, Object>>();
        BlockPos origin = new BlockPos(15, -20, -1);
        for (int shape = 0; shape < 6; shape++) {
            for (long seed = 0; seed < 8; seed++) {
                fixtures.add(run(chunk, origin, shape, seed));
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(fixtures));
    }

    /** Builds a sparse cave with a ceiling, a floor, or both. */
    private static Map<BlockPos, BlockState> input(BlockPos origin, int shape) {
        var input = new HashMap<BlockPos, BlockState>();
        if (shape == 5) {
            input.put(origin.above(), Blocks.DRIPSTONE_BLOCK.defaultBlockState());
        }
        if (shape == 0 || shape == 2 || shape == 4) {
            for (int x = -3; x <= 3; x++) {
                for (int z = -3; z <= 3; z++) {
                    input.put(origin.offset(x, 1, z), Blocks.STONE.defaultBlockState());
                }
            }
        }
        if (shape == 1 || shape == 2) {
            for (int x = -3; x <= 3; x++) {
                for (int z = -3; z <= 3; z++) {
                    input.put(origin.offset(x, -1, z), Blocks.STONE.defaultBlockState());
                }
            }
        }
        if (shape == 4) {
            input.put(origin, Blocks.WATER.defaultBlockState());
            input.put(origin.below(), Blocks.WATER.defaultBlockState());
        }
        return input;
    }

    /** Records final changed states and one subsequent RNG value. */
    private static Map<String, Object> run(ProtoChunk chunk, BlockPos origin, int shape, long seed) {
        var initial = input(origin, shape);
        var terrain = new HashMap<>(initial);
        var writes = new HashMap<BlockPos, BlockState>();
        WorldGenLevel world = (WorldGenLevel) Proxy.newProxyInstance(WorldGenLevel.class.getClassLoader(),
            new Class<?>[] {WorldGenLevel.class}, (proxy, method, args) -> {
                switch (method.getName()) {
                    case "getBlockState": return terrain.getOrDefault((BlockPos) args[0], Blocks.AIR.defaultBlockState());
                    case "isStateAtPosition":
                        return ((Predicate<BlockState>) args[1]).test(terrain.getOrDefault((BlockPos) args[0], Blocks.AIR.defaultBlockState()));
                    case "isWaterAt": return terrain.getOrDefault((BlockPos) args[0], Blocks.AIR.defaultBlockState()).getBlock() == Blocks.WATER;
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
        var config = new PointedDripstoneConfiguration(.2F, .7F, .5F, .5F);
        boolean placed = new PointedDripstoneFeature(PointedDripstoneConfiguration.CODEC)
            .place(new FeaturePlaceContext<>(Optional.empty(), world, null, random, origin, config));
        var changed = new HashMap<BlockPos, BlockState>();
        writes.forEach((pos, state) -> {
            if (state != initial.getOrDefault(pos, Blocks.AIR.defaultBlockState())) changed.put(pos, state);
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
