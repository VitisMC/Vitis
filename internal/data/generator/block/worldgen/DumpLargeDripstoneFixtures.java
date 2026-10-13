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
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.LargeDripstoneFeature;
import net.minecraft.world.level.levelgen.feature.configurations.LargeDripstoneConfiguration;

/** Exports large dripstone geometry and RNG tails from controlled cave layers. */
public final class DumpLargeDripstoneFixtures {
    /** Runs both deep and shallow caves with rejected origins and lava boundaries. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        List<Holder<Block>> stone = List.of(Blocks.STONE.builtInRegistryHolder());
        BuiltInRegistries.BLOCK.prepareTagReload(new TagLoader.LoadResult<>(Registries.BLOCK,
            Map.of(BlockTags.DRIPSTONE_REPLACEABLE, stone, BlockTags.BASE_STONE_OVERWORLD, stone))).apply();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        Registry.register(biomes, Biomes.PLAINS, biome);
        biomes.freeze();
        var chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY,
            LevelHeightAccessor.create(-64, 128), biomes, null);
        var config = new LargeDripstoneConfiguration(30, UniformInt.of(3, 19), UniformFloat.of(.4F, 2.0F),
            .33F, UniformFloat.of(.3F, .9F), UniformFloat.of(.4F, 1.0F), UniformFloat.of(0F, .3F), 4, .6F);
        var cases = new ArrayList<Map<String, Object>>();
        for (int shape = 0; shape < 4; shape++) {
            for (long seed = 0; seed < 4; seed++) {
                cases.add(run(chunk, config, new BlockPos(15, -20, -1), shape, seed));
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(cases));
    }

    /** Returns the pre-placement cave block for one controlled scenario. */
    private static BlockState initial(BlockPos origin, int shape, BlockPos pos) {
        if (shape == 2 && pos.equals(origin)) return Blocks.STONE.defaultBlockState();
        if (Math.abs(pos.getX() - origin.getX()) > 20 || Math.abs(pos.getZ() - origin.getZ()) > 20) {
            return Blocks.AIR.defaultBlockState();
        }
        int y = pos.getY() - origin.getY();
        int threshold = shape == 1 ? 5 : 12;
        if (shape == 3 && y == threshold) return Blocks.LAVA.defaultBlockState();
        if (y >= threshold && y <= threshold + 4) return Blocks.STONE.defaultBlockState();
        if (y <= -threshold && y >= -threshold - 4) return Blocks.STONE.defaultBlockState();
        return Blocks.AIR.defaultBlockState();
    }

    /** Places one feature and captures final changed blocks and the next random long. */
    private static Map<String, Object> run(ProtoChunk chunk, LargeDripstoneConfiguration config, BlockPos origin, int shape, long seed) {
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
                    case "isWaterAt": return false;
                    case "setBlock":
                        BlockPos pos = ((BlockPos) args[0]).immutable();
                        terrain.put(pos, (BlockState) args[1]);
                        writes.put(pos, (BlockState) args[1]);
                        return true;
                    case "getHeight":
                        if (args == null || args.length == 0) return 128;
                        int x = (Integer) args[1];
                        int z = (Integer) args[2];
                        for (int y = 63; y >= -64; y--) {
                            BlockPos p = new BlockPos(x, y, z);
                            if (!terrain.getOrDefault(p, initial(origin, shape, p)).isAir()) return y + 1;
                        }
                        return -64;
                    case "getChunk": return chunk;
                    case "getMinY": return -64;
                    case "getMaxY": return 63;
                    case "getSeed": return 0L;
                    case "ensureCanWrite": return true;
                    default: throw new UnsupportedOperationException(method.toString());
                }
            });
        RandomSource random = RandomSource.create(seed);
        boolean placed = new LargeDripstoneFeature(LargeDripstoneConfiguration.CODEC)
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

    /** Serializes changed states in stable x/z/y order. */
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

    /** Converts each block property to its canonical string. */
    @SuppressWarnings({"rawtypes", "unchecked"})
    private static String serialized(Property property, Comparable value) { return property.getName(value); }
}
