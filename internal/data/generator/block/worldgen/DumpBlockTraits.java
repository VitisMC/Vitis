import com.google.gson.Gson;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import net.minecraft.SharedConstants;
import net.minecraft.core.BlockPos;
import net.minecraft.core.Direction;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.level.EmptyBlockGetter;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.state.BlockState;
import net.minecraft.world.level.block.state.properties.Property;
import net.minecraft.world.level.material.Fluids;
import net.minecraft.world.phys.shapes.VoxelShape;

/** Exports generation traits from every registered vanilla state using empty collision context. */
public final class DumpBlockTraits {
    /** Initializes registries and writes a complete state inventory to the supplied path. */
    public static void main(String[] args) throws Exception {
        if (args.length != 1) throw new IllegalArgumentException("Expected output JSON path");
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        List<Map<String, Object>> states = new ArrayList<>();
        for (BlockState state : Block.BLOCK_STATE_REGISTRY) {
            VoxelShape support = state.getBlockSupportShape(EmptyBlockGetter.INSTANCE, BlockPos.ZERO);
            VoxelShape collision = state.getCollisionShape(EmptyBlockGetter.INSTANCE, BlockPos.ZERO);
            int traits = 0;
            for (Direction face : Direction.values()) {
                if (Block.isFaceFull(support, face)) traits |= 1 << face.ordinal();
                if (Block.isFaceFull(collision, face)) traits |= 1 << (face.ordinal() + 6);
            }
            if (state.canBeReplaced()) traits |= 1 << 12;
            if (state.getFluidState().is(Fluids.WATER) || state.getFluidState().is(Fluids.FLOWING_WATER)) traits |= 1 << 13;
            if (state.getFluidState().isSourceOfType(Fluids.WATER)) traits |= 1 << 14;
            if (!state.getFluidState().isEmpty()) traits |= 1 << 15;
            if (state.isSolid()) traits |= 1 << 16;
            traits |= state.getLightEmission() << 17;
            if (state.liquid()) traits |= 1 << 21;
            Map<String, String> properties = new LinkedHashMap<>();
            state.getValues().forEach((property, value) -> properties.put(property.getName(), serialized(property, value)));
            Map<String, Object> entry = new LinkedHashMap<>();
            entry.put("name", BuiltInRegistries.BLOCK.getKey(state.getBlock()).toString());
            entry.put("properties", properties);
            entry.put("traits", traits);
            states.add(entry);
        }
        Files.writeString(Path.of(args[0]), new Gson().toJson(states));
    }

    /** Uses registry serialization instead of enum display names. */
    @SuppressWarnings({"rawtypes", "unchecked"})
    private static String serialized(Property property, Comparable value) {
        return property.getName(value);
    }
}
