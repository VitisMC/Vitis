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
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.MappedRegistry;
import net.minecraft.core.Registry;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.core.registries.Registries;
import net.minecraft.server.Bootstrap;
import net.minecraft.util.RandomSource;
import net.minecraft.world.level.ChunkPos;
import net.minecraft.world.level.LevelHeightAccessor;
import net.minecraft.world.level.WorldGenLevel;
import net.minecraft.world.level.biome.Biome;
import net.minecraft.world.level.biome.BiomeGenerationSettings;
import net.minecraft.world.level.biome.BiomeSpecialEffects;
import net.minecraft.world.level.biome.Biomes;
import net.minecraft.world.level.biome.MobSpawnSettings;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.chunk.ProtoChunk;
import net.minecraft.world.level.chunk.UpgradeData;
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.LakeFeature;
import net.minecraft.world.level.levelgen.feature.stateproviders.BlockStateProvider;

/** Exports lava lake geometry and random tails from the versioned feature. */
public final class DumpLakeFixtures {
    /** Runs border, negative, ordinary, and minimum-height cases. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        Registry.register(biomes, Biomes.PLAINS, biome);
        biomes.freeze();
        var chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY,
            LevelHeightAccessor.create(-64, 128), biomes, null);
        var fixtures = new ArrayList<Map<String, Object>>();
        for (int[] origin : new int[][] {{15, -20, -1}, {-32, 40, 31}, {2, 16, 2}, {0, -60, 0}}) {
            for (long seed = 0; seed < 8; seed++) {
                fixtures.add(run(chunk, new BlockPos(origin[0], origin[1], origin[2]), seed));
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(fixtures));
    }

    /** Captures final changed blocks, placement success, and the next RNG value. */
    private static Map<String, Object> run(ProtoChunk chunk, BlockPos origin, long seed) {
        var terrain = new HashMap<BlockPos, BlockState>();
        WorldGenLevel world = (WorldGenLevel) Proxy.newProxyInstance(WorldGenLevel.class.getClassLoader(),
            new Class<?>[] {WorldGenLevel.class}, (proxy, method, args) -> {
                switch (method.getName()) {
                    case "getBlockState": return terrain.getOrDefault((BlockPos) args[0], Blocks.STONE.defaultBlockState());
                    case "getFluidState": return terrain.getOrDefault((BlockPos) args[0], Blocks.STONE.defaultBlockState()).getFluidState();
                    case "setBlock":
                        terrain.put(((BlockPos) args[0]).immutable(), (BlockState) args[1]);
                        return true;
                    case "scheduleTick": return null;
                    case "getChunk": return chunk;
                    case "getMinY": return -64;
                    case "getMaxY": return 63;
                    case "getHeight": return 128;
                    case "getSeed": return 0L;
                    case "ensureCanWrite": return true;
                    default: throw new UnsupportedOperationException(method.toString());
                }
            });
        var config = new LakeFeature.Configuration(BlockStateProvider.simple(Blocks.LAVA.defaultBlockState()),
            BlockStateProvider.simple(Blocks.STONE.defaultBlockState()));
        RandomSource random = RandomSource.create(seed);
        boolean placed = new LakeFeature(LakeFeature.Configuration.CODEC)
            .place(new FeaturePlaceContext<>(Optional.empty(), world, null, random, origin, config));
        var changes = new ArrayList<Map<String, Object>>();
        terrain.entrySet().stream()
            .filter(entry -> entry.getValue() != Blocks.STONE.defaultBlockState())
            .sorted(Comparator.comparingInt((Map.Entry<BlockPos, BlockState> entry) -> entry.getKey().getX())
                .thenComparingInt(entry -> entry.getKey().getZ()).thenComparingInt(entry -> entry.getKey().getY()))
            .forEach(entry -> {
                var row = new LinkedHashMap<String, Object>();
                var pos = entry.getKey();
                row.put("position", new int[] {pos.getX(), pos.getY(), pos.getZ()});
                row.put("block", BuiltInRegistries.BLOCK.getKey(entry.getValue().getBlock()).toString());
                changes.add(row);
            });
        var result = new LinkedHashMap<String, Object>();
        result.put("seed", seed);
        result.put("origin", new int[] {origin.getX(), origin.getY(), origin.getZ()});
        result.put("placed", placed);
        result.put("tail", random.nextLong());
        result.put("changes", changes);
        return result;
    }
}
