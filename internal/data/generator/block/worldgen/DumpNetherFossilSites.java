import com.google.gson.Gson;
import com.mojang.serialization.MapCodec;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.HolderSet;
import net.minecraft.server.Bootstrap;
import net.minecraft.server.level.WorldGenRegion;
import net.minecraft.world.level.ChunkPos;
import net.minecraft.world.level.LevelHeightAccessor;
import net.minecraft.world.level.NoiseColumn;
import net.minecraft.world.level.StructureManager;
import net.minecraft.world.level.biome.BiomeManager;
import net.minecraft.world.level.biome.FixedBiomeSource;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.Rotation;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.block.state.properties.BlockStateProperties;
import net.minecraft.world.level.chunk.ChunkAccess;
import net.minecraft.world.level.chunk.ChunkGenerator;
import net.minecraft.world.level.levelgen.GenerationStep;
import net.minecraft.world.level.levelgen.Heightmap;
import net.minecraft.world.level.levelgen.RandomState;
import net.minecraft.world.level.levelgen.VerticalAnchor;
import net.minecraft.world.level.levelgen.blending.Blender;
import net.minecraft.world.level.levelgen.heightproviders.UniformHeight;
import net.minecraft.world.level.levelgen.structure.Structure;
import net.minecraft.world.level.levelgen.structure.TerrainAdjustment;
import net.minecraft.world.level.levelgen.structure.structures.NetherFossilStructure;

/** Runs the versioned Nether fossil site selection against controlled raw columns. */
public final class DumpNetherFossilSites {
    /** Writes deterministic site, rotation and template choice reference cases. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        NetherFossilStructure structure = new NetherFossilStructure(
            new Structure.StructureSettings(HolderSet.direct(), Map.of(),
                GenerationStep.Decoration.UNDERGROUND_DECORATION, TerrainAdjustment.BEARD_THIN),
            UniformHeight.of(VerticalAnchor.absolute(32), VerticalAnchor.belowTop(2)));
        LevelHeightAccessor world = LevelHeightAccessor.create(0, 256);
        List<Map<String, Object>> cases = new ArrayList<>();
        int[][] chunks = {{0, 0}, {-1, -2}, {3, 5}, {-17, 21}};
        for (int shape = 0; shape < 9; shape++) {
            for (int[] chunk : chunks) {
                for (long seed = 0; seed < 16; seed++) {
                    Structure.GenerationContext context = new Structure.GenerationContext(null,
                        new ColumnGenerator(shape), null, null, null, seed,
                        new ChunkPos(chunk[0], chunk[1]), world, biome -> true);
                    var found = structure.findGenerationPoint(context);
                    Map<String, Object> row = new LinkedHashMap<>();
                    row.put("seed", seed);
                    row.put("chunk", chunk);
                    row.put("shape", shape);
                    if (found.isPresent()) {
                        BlockPos point = found.get().position();
                        row.put("point", new int[] {point.getX(), point.getY(), point.getZ()});
                        row.put("rotation", Rotation.getRandom(context.random()).name());
                        row.put("template", context.random().nextInt(14) + 1);
                    }
                    row.put("tail", context.random().nextLong());
                    cases.add(row);
                }
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(cases));
    }

    /** Supplies raw terrain only; every variation isolates site scanning and support checks. */
    private static final class ColumnGenerator extends ChunkGenerator {
        private final int shape;

        ColumnGenerator(int shape) { super(new FixedBiomeSource(null)); this.shape = shape; }

        @Override protected MapCodec<? extends ChunkGenerator> codec() { return null; }
        @Override public void applyCarvers(WorldGenRegion region, long seed, RandomState randomState,
            BiomeManager biomes, StructureManager structures, ChunkAccess chunk) {}
        @Override public void buildSurface(WorldGenRegion region, StructureManager structures,
            RandomState randomState, ChunkAccess chunk) {}
        @Override public void spawnOriginalMobs(WorldGenRegion region) {}
        @Override public int getGenDepth() { return 128; }
        @Override public int getSeaLevel() { return 32; }
        @Override public int getMinY() { return 0; }
        @Override public int getBaseHeight(int x, int z, Heightmap.Types type,
            LevelHeightAccessor world, RandomState randomState) { return 0; }
        @Override public NoiseColumn getBaseColumn(int x, int z, LevelHeightAccessor world, RandomState randomState) {
            BlockState[] blocks = new BlockState[128];
            for (int y = 0; y < blocks.length; y++) blocks[y] = block(y);
            return new NoiseColumn(0, blocks);
        }
        private BlockState block(int y) {
            int floor = switch (shape) {
                case 7 -> 32;
                case 8 -> 105;
                default -> 50;
            };
            if (shape == 1) return Blocks.NETHERRACK.defaultBlockState();
            if (shape == 2) return Blocks.AIR.defaultBlockState();
            if (y > floor) return Blocks.AIR.defaultBlockState();
            if (y < floor) return Blocks.NETHERRACK.defaultBlockState();
            return switch (shape) {
                case 3 -> Blocks.SOUL_SAND.defaultBlockState();
                case 4 -> Blocks.OAK_SLAB.defaultBlockState();
                case 5 -> Blocks.OAK_SLAB.defaultBlockState().setValue(BlockStateProperties.SLAB_TYPE,
                    net.minecraft.world.level.block.state.properties.SlabType.TOP);
                case 6 -> Blocks.LAVA.defaultBlockState();
                default -> Blocks.NETHERRACK.defaultBlockState();
            };
        }
        @Override public CompletableFuture<ChunkAccess> fillFromNoise(Blender blender, RandomState randomState,
            StructureManager structures, ChunkAccess chunk) { return CompletableFuture.completedFuture(chunk); }
        @Override public void addDebugScreenInfo(List<String> lines, RandomState state, BlockPos position) {}
    }
}
