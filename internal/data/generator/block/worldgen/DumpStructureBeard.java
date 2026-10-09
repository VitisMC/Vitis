import com.google.gson.Gson;
import it.unimi.dsi.fastutil.objects.ObjectArrayList;
import java.lang.reflect.Field;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import net.minecraft.SharedConstants;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.level.levelgen.Beardifier;
import net.minecraft.world.level.levelgen.DensityFunction;
import net.minecraft.world.level.levelgen.structure.BoundingBox;
import net.minecraft.world.level.levelgen.structure.TerrainAdjustment;
import net.minecraft.world.level.levelgen.structure.pools.JigsawJunction;
import net.minecraft.world.level.levelgen.structure.pools.StructureTemplatePool;

/** Exports the versioned structure terrain kernel and independent density examples. */
public final class DumpStructureBeard {
    /** Writes raw float bits and reference samples from the Minecraft implementation. */
    public static void main(String[] args) throws Exception {
        if (args.length != 2) throw new IllegalArgumentException("Expected kernel and cases paths");
        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        Field field = Beardifier.class.getDeclaredField("BEARD_KERNEL");
        field.setAccessible(true);
        float[] kernel = (float[]) field.get(null);
        if (kernel.length != 13824) throw new IllegalStateException("Unexpected kernel size");
        ByteBuffer bytes = ByteBuffer.allocate(kernel.length * 4).order(ByteOrder.LITTLE_ENDIAN);
        for (float value : kernel) bytes.putInt(Float.floatToRawIntBits(value));
        Files.write(Path.of(args[0]), bytes.array());

        List<Map<String, Object>> cases = new ArrayList<>();
        BoundingBox box = new BoundingBox(10, -30, -20, 23, -18, -7);
        int[][] points = {
            {10, -30, -20}, {10, -29, -20}, {16, -24, -13}, {23, -18, -7},
            {-2, -30, -20}, {-3, -30, -20}, {35, -30, -20}, {36, -30, -20},
            {10, -42, -20}, {10, -43, -20}, {10, -6, -20}, {10, -5, -20},
            {24, -30, -20}, {10, -31, -21}, {0, -10, 0}, {40, -45, -32}
        };
        for (TerrainAdjustment mode : TerrainAdjustment.values()) {
            for (int ground : new int[] {0, 3}) {
                for (int[] point : points) {
                    cases.add(example(mode, ground, point, false));
                }
            }
        }
        for (int[] point : points) cases.add(example(TerrainAdjustment.BEARD_THIN, 3, point, true));
        Files.writeString(Path.of(args[1]), new Gson().toJson(cases));
    }

    /** Computes a case using Minecraft's actual iteration and floating-point order. */
    private static Map<String, Object> example(TerrainAdjustment mode, int ground, int[] point, boolean extra) {
        var pieces = new ObjectArrayList<Beardifier.Rigid>();
        pieces.add(new Beardifier.Rigid(new BoundingBox(10, -30, -20, 23, -18, -7), mode, ground));
        var junctions = new ObjectArrayList<JigsawJunction>();
        if (extra) {
            pieces.add(new Beardifier.Rigid(new BoundingBox(-1, -31, -11, 5, -25, -2), TerrainAdjustment.BURY, 0));
            junctions.add(new JigsawJunction(7, -27, -15, 0, StructureTemplatePool.Projection.RIGID));
            junctions.add(new JigsawJunction(10, -30, -20, 0, StructureTemplatePool.Projection.RIGID));
        }
        Beardifier beard = new Beardifier(pieces.iterator(), junctions.iterator());
        double value = beard.compute(new DensityFunction.SinglePointContext(point[0], point[1], point[2]));
        // Verify that iterator rewind yields identical output on the same object.
        if (Double.doubleToRawLongBits(value) != Double.doubleToRawLongBits(beard.compute(
            new DensityFunction.SinglePointContext(point[0], point[1], point[2])))) {
            throw new IllegalStateException("Beardifier iterator was not rewound");
        }
        Map<String, Object> result = new LinkedHashMap<>();
        result.put("mode", mode.name());
        result.put("ground", ground);
        result.put("point", point);
        result.put("extra", extra);
        result.put("bits", Long.toUnsignedString(Double.doubleToRawLongBits(value)));
        return result;
    }
}
