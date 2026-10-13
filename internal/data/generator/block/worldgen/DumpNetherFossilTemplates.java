import com.google.gson.Gson;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.nbt.NbtAccounter;
import net.minecraft.nbt.NbtIo;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.level.block.Blocks;
import net.minecraft.world.level.block.Rotation;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.block.state.properties.BlockStateProperties;
import net.minecraft.world.level.levelgen.structure.BoundingBox;
import net.minecraft.world.level.levelgen.structure.templatesystem.StructurePlaceSettings;
import net.minecraft.world.level.levelgen.structure.templatesystem.StructureTemplate;

/** Exports transformed geometry directly from all versioned Nether fossil templates. */
public final class DumpNetherFossilTemplates {
    /** Writes every fossil skeleton rotation and its enclosing template bounds. */
    public static void main(String[] args) throws Exception {
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        List<Map<String, Object>> cases = new ArrayList<>();
        BlockPos origin = new BlockPos(-17, 50, 22);
        for (int number = 1; number <= 14; number++) {
            Path source = Path.of(args[1], "fossil_" + number + ".nbt");
            StructureTemplate template = new StructureTemplate();
            template.load(BuiltInRegistries.BLOCK, NbtIo.readCompressed(source, NbtAccounter.unlimitedHeap()));
            for (Rotation rotation : Rotation.values()) {
                StructurePlaceSettings settings = new StructurePlaceSettings().setRotation(rotation);
                BoundingBox bounds = template.getBoundingBox(settings, origin);
                List<List<Object>> bones = new ArrayList<>();
                for (StructureTemplate.StructureBlockInfo item : template.filterBlocks(origin, settings, Blocks.BONE_BLOCK)) {
                    BlockPos p = item.pos();
                    BlockState state = item.state();
                    bones.add(List.of(p.getX(), p.getY(), p.getZ(), state.getValue(BlockStateProperties.AXIS).getSerializedName()));
                }
                bones.sort(Comparator.comparingInt((List<Object> p) -> (int) p.get(0))
                    .thenComparingInt(p -> (int) p.get(1)).thenComparingInt(p -> (int) p.get(2)));
                Map<String, Object> row = new LinkedHashMap<>();
                row.put("template", number);
                row.put("rotation", rotation.name());
                row.put("box", new int[] {bounds.minX(), bounds.minY(), bounds.minZ(), bounds.maxX(), bounds.maxY(), bounds.maxZ()});
                row.put("bones", bones);
                cases.add(row);
            }
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(cases));
    }
}
