import com.google.gson.Gson;
import com.mojang.serialization.Lifecycle;
import java.lang.reflect.Proxy;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Direction;
import net.minecraft.core.HolderSet;
import net.minecraft.core.MappedRegistry;
import net.minecraft.core.Registry;
import net.minecraft.core.Holder;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.core.registries.Registries;
import net.minecraft.server.Bootstrap;
import net.minecraft.resources.ResourceLocation;
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
import net.minecraft.world.level.block.MultifaceSpreadeableBlock;
import net.minecraft.world.level.block.MultifaceBlock;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.block.state.properties.BlockStateProperties;
import net.minecraft.world.level.block.state.properties.Property;
import net.minecraft.world.level.chunk.ProtoChunk;
import net.minecraft.world.level.chunk.UpgradeData;
import net.minecraft.world.level.levelgen.feature.FeaturePlaceContext;
import net.minecraft.world.level.levelgen.feature.MultifaceGrowthFeature;
import net.minecraft.world.level.levelgen.feature.configurations.MultifaceGrowthConfiguration;

/** Exports block writes and RNG tails from the actual versioned multiface feature. */
public final class DumpMultifaceFixtures {
    /** Runs isolated deterministic feature cases without constructing a running server. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        List<Holder<Block>> fire = new ArrayList<>();
        for (String id : new Gson().fromJson(Files.readString(Path.of(args[1])), String[].class)) {
            fire.add(BuiltInRegistries.BLOCK.get(ResourceLocation.parse(id)).orElseThrow());
        }
        BuiltInRegistries.BLOCK.prepareTagReload(new TagLoader.LoadResult<>(Registries.BLOCK, Map.of(BlockTags.FIRE, fire))).apply();
        var biomes = new MappedRegistry<Biome>(Registries.BIOME, Lifecycle.stable());
        Biome biome = new Biome.BiomeBuilder().hasPrecipitation(false).temperature(0.5F).downfall(0.5F)
            .specialEffects(new BiomeSpecialEffects.Builder().fogColor(0).waterColor(0).waterFogColor(0).skyColor(0).build())
            .mobSpawnSettings(MobSpawnSettings.EMPTY).generationSettings(BiomeGenerationSettings.EMPTY).build();
        Registry.register(biomes, Biomes.PLAINS, biome);
        biomes.freeze();
        ProtoChunk chunk = new ProtoChunk(new ChunkPos(0, 0), UpgradeData.EMPTY, LevelHeightAccessor.create(-64, 128), biomes, null);
        List<Map<String, Object>> cases = new ArrayList<>();
        BlockPos origin = new BlockPos(15, -20, -1);
        for (Block block : List.of(Blocks.GLOW_LICHEN, Blocks.SCULK_VEIN)) {
            for (int shape = 0; shape < 7; shape++) {
                for (long seed = 0; seed < 16; seed++) {
                    Map<BlockPos, BlockState> input = new LinkedHashMap<>();
                    if (shape == 0) {
                        for (Direction face : Direction.values()) input.put(origin.relative(face), Blocks.STONE.defaultBlockState());
                    } else if (shape == 1 || shape == 2) {
                        input.put(origin.east(), Blocks.STONE.defaultBlockState());
                        if (shape == 2) {
                            input.put(origin.east().north(), Blocks.STONE.defaultBlockState());
                            input.put(origin.east().south(), Blocks.STONE.defaultBlockState());
                        }
                    } else if (shape == 3) {
                        input.put(origin.above().east(), Blocks.STONE.defaultBlockState());
                    } else if (shape == 4 || shape == 5) {
                        input.put(origin, Blocks.WATER.defaultBlockState().setValue(BlockStateProperties.LEVEL, shape == 4 ? 0 : 3));
                        input.put(origin.east(), Blocks.STONE.defaultBlockState());
                    } else {
                        input.put(origin.east(), Blocks.STONE.defaultBlockState());
                        for (Direction face : List.of(Direction.NORTH, Direction.SOUTH, Direction.UP, Direction.DOWN)) {
                            input.put(origin.east().relative(face), Blocks.SHORT_GRASS.defaultBlockState());
                        }
                    }
                    cases.add(run("shape-" + shape, block, origin, seed, 20, block == Blocks.SCULK_VEIN, true, true, 1.0F,
                        List.of(Blocks.STONE), input, chunk));
                }
            }
            for (int range : List.of(1, 64)) {
                cases.add(run("empty-range-" + range, block, origin, 123, range, true, true, true, 0.0F,
                    List.of(Blocks.STONE), new LinkedHashMap<>(), chunk));
            }
            for (Block support : List.of(Blocks.OAK_LEAVES, Blocks.MUD, Blocks.SOUL_SAND, Blocks.SNOW, Blocks.OAK_SLAB, Blocks.OAK_STAIRS)) {
                for (Direction face : Direction.values()) {
                    Map<BlockPos, BlockState> input = new LinkedHashMap<>();
                    input.put(origin.relative(face), support.defaultBlockState());
                    cases.add(run("support-" + BuiltInRegistries.BLOCK.getKey(support) + "-" + face.getSerializedName(), block,
                        origin, 5, 10, true, true, true, 0.0F, List.of(support), input, chunk));
                }
            }
            List<BlockState> replacements = List.of(Blocks.AIR.defaultBlockState(), Blocks.WATER.defaultBlockState(),
                Blocks.WATER.defaultBlockState().setValue(BlockStateProperties.LEVEL, 3), Blocks.SEAGRASS.defaultBlockState(),
                Blocks.LAVA.defaultBlockState(), Blocks.FIRE.defaultBlockState(), Blocks.SHORT_GRASS.defaultBlockState(),
                Blocks.STONE.defaultBlockState(), block.defaultBlockState().setValue(MultifaceBlock.getFaceProperty(Direction.DOWN), true));
            for (int shape = 0; shape < 3; shape++) {
                for (int replacement = 0; replacement < replacements.size(); replacement++) {
                    Map<BlockPos, BlockState> input = new LinkedHashMap<>();
                    input.put(origin, block.defaultBlockState().setValue(MultifaceBlock.getFaceProperty(Direction.EAST), true));
                    input.put(origin.east(), Blocks.STONE.defaultBlockState());
                    if (shape == 0) {
                        input.put(origin.north(), Blocks.STONE.defaultBlockState());
                    } else if (shape == 1) {
                        input.put(origin.north(), replacements.get(replacement));
                        input.put(origin.north().east(), Blocks.STONE.defaultBlockState());
                    } else {
                        input.put(origin.north().east(), replacements.get(replacement));
                    }
                    cases.add(run("spread-" + shape + "-" + replacement, block, origin, 7, 10, true, true, true, 1.0F,
                        List.of(Blocks.STONE), input, chunk, Direction.EAST, Direction.NORTH));
                }
            }
            for (Block support : List.of(Blocks.SCULK, Blocks.SCULK_CATALYST, Blocks.MOVING_PISTON)) {
                Map<BlockPos, BlockState> input = new LinkedHashMap<>();
                input.put(origin, block.defaultBlockState().setValue(MultifaceBlock.getFaceProperty(Direction.EAST), true));
                input.put(origin.east(), Blocks.STONE.defaultBlockState());
                input.put(origin.north(), support.defaultBlockState());
                input.put(origin.north().east(), Blocks.STONE.defaultBlockState());
                cases.add(run("spread-support-" + BuiltInRegistries.BLOCK.getKey(support), block, origin, 7, 10, true, true, true, 1.0F,
                    List.of(Blocks.STONE), input, chunk, Direction.EAST, Direction.NORTH));
            }
            if (block == Blocks.SCULK_VEIN) {
                for (boolean blocked : List.of(false, true)) {
                    Map<BlockPos, BlockState> input = new LinkedHashMap<>();
                    input.put(origin, Blocks.SCULK.defaultBlockState());
                    input.put(origin.east(), Blocks.STONE.defaultBlockState());
                    if (blocked) input.put(origin.north(), Blocks.STONE.defaultBlockState());
                    cases.add(run("spread-other-source-" + blocked, block, origin, 7, 10, true, true, true, 1.0F,
                        List.of(Blocks.STONE), input, chunk, Direction.EAST, Direction.NORTH));
                }
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(cases));
    }

    /** Invokes Minecraft's feature against a sparse fixed terrain view and records every final write. */
    private static Map<String, Object> run(String name, Block block, BlockPos origin, long seed, int range,
        boolean floor, boolean ceiling, boolean wall, float chance, List<Block> targets, Map<BlockPos, BlockState> input, ProtoChunk chunk) {
        return run(name, block, origin, seed, range, floor, ceiling, wall, chance, targets, input, chunk, null, null);
    }

    /** Captures either initial placement or a directed spreading operation against the same terrain. */
    private static Map<String, Object> run(String name, Block block, BlockPos origin, long seed, int range,
        boolean floor, boolean ceiling, boolean wall, float chance, List<Block> targets, Map<BlockPos, BlockState> input, ProtoChunk chunk,
        Direction from, Direction toward) {
        Map<BlockPos, BlockState> terrain = new LinkedHashMap<>(input);
        Map<BlockPos, BlockState> writes = new LinkedHashMap<>();
        WorldGenLevel world = (WorldGenLevel) Proxy.newProxyInstance(WorldGenLevel.class.getClassLoader(), new Class<?>[]{WorldGenLevel.class},
            (proxy, method, args) -> {
                switch (method.getName()) {
                    case "getBlockState": return terrain.getOrDefault((BlockPos) args[0], Blocks.AIR.defaultBlockState());
                    case "getFluidState": return terrain.getOrDefault((BlockPos) args[0], Blocks.AIR.defaultBlockState()).getFluidState();
                    case "getBlockEntity": return null;
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
        var config = new MultifaceGrowthConfiguration((MultifaceSpreadeableBlock) block, range, floor, ceiling, wall, chance,
            HolderSet.direct(targets.stream().map(Block::builtInRegistryHolder).toList()));
        RandomSource random = RandomSource.create(seed);
        boolean placed = from == null ? new MultifaceGrowthFeature(MultifaceGrowthConfiguration.CODEC)
            .place(new FeaturePlaceContext<>(Optional.empty(), world, null, random, origin, config))
            : ((MultifaceSpreadeableBlock) block).getSpreader()
                .spreadFromFaceTowardDirection(terrain.get(origin), world, origin, from, toward, true).isPresent();
        Map<String, Object> options = new LinkedHashMap<>();
        options.put("block", BuiltInRegistries.BLOCK.getKey(block).toString());
        options.put("search_range", range);
        options.put("can_place_on_floor", floor);
        options.put("can_place_on_ceiling", ceiling);
        options.put("can_place_on_wall", wall);
        options.put("chance_of_spreading", chance);
        options.put("can_be_placed_on", targets.stream().map(target -> BuiltInRegistries.BLOCK.getKey(target).toString()).toList());
        Map<String, Object> result = new LinkedHashMap<>();
        result.put("name", name);
        result.put("seed", seed);
        result.put("origin", List.of(origin.getX(), origin.getY(), origin.getZ()));
        if (from != null) {
            result.put("from", from.getSerializedName());
            result.put("toward", toward.getSerializedName());
        }
        Map<String, Object> feature = new LinkedHashMap<>();
        feature.put("type", "minecraft:multiface_growth");
        feature.put("config", options);
        result.put("feature", feature);
        result.put("input", states(input));
        result.put("placed", placed);
        result.put("writes", states(writes));
        result.put("tail", random.nextLong());
        return result;
    }

    /** Serializes state properties using the registry's canonical names. */
    private static List<Map<String, Object>> states(Map<BlockPos, BlockState> states) {
        List<Map<String, Object>> result = new ArrayList<>();
        states.forEach((pos, state) -> {
            Map<String, String> properties = new LinkedHashMap<>();
            state.getValues().forEach((property, value) -> properties.put(property.getName(), serialized(property, value)));
            Map<String, Object> entry = new LinkedHashMap<>();
            entry.put("position", List.of(pos.getX(), pos.getY(), pos.getZ()));
            entry.put("name", BuiltInRegistries.BLOCK.getKey(state.getBlock()).toString());
            entry.put("properties", properties);
            result.add(entry);
        });
        return result;
    }

    /** Converts enum values to stable serialized property values. */
    @SuppressWarnings({"rawtypes", "unchecked"})
    private static String serialized(Property property, Comparable value) { return property.getName(value); }
}
