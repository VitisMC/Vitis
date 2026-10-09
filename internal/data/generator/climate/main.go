//go:build ignore

// Command climate generates exact multi-noise biome preset parameters.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const helperSource = `
import java.util.Map;
import com.mojang.datafixers.util.Pair;
import net.minecraft.SharedConstants;
import net.minecraft.server.Bootstrap;
import net.minecraft.resources.ResourceKey;
import net.minecraft.world.level.biome.Biome;
import net.minecraft.world.level.biome.Climate;
import net.minecraft.world.level.biome.MultiNoiseBiomeSourceParameterList;

public final class VitisClimateDump {
  public static void main(String[] args) {
    SharedConstants.tryDetectVersion();
    Bootstrap.bootStrap();
    for (Map.Entry<MultiNoiseBiomeSourceParameterList.Preset, Climate.ParameterList<ResourceKey<Biome>>> preset : MultiNoiseBiomeSourceParameterList.knownPresets().entrySet()) {
      System.out.println("VITIS_PRESET\t" + preset.getKey().id());
      for (Pair<Climate.ParameterPoint, ResourceKey<Biome>> pair : preset.getValue().values()) {
        Climate.ParameterPoint p = pair.getFirst();
        System.out.printf("VITIS_POINT\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s%n",
          p.temperature().min(), p.temperature().max(), p.humidity().min(), p.humidity().max(),
          p.continentalness().min(), p.continentalness().max(), p.erosion().min(), p.erosion().max(),
          p.depth().min(), p.depth().max(), p.weirdness().min(), p.weirdness().max(), p.offset(), pair.getSecond().location());
      }
    }
  }
}
`

type point struct {
	values [14]string
}

func main() {
	version := flag.String("version", "1.21.4", "Minecraft version")
	flag.Parse()
	root := projectRoot()
	serverJAR := filepath.Join(root, ".mc-decompiled", "downloads", *version, "server.jar")
	remappedJAR := filepath.Join(root, ".mc-decompiled", *version+"-remapped.jar")
	if _, err := os.Stat(remappedJAR); err != nil {
		fatal("missing remapped server JAR %s: %v", remappedJAR, err)
	}
	temporary, err := os.MkdirTemp("", "vitis-climate-")
	if err != nil {
		fatal("create temporary directory: %v", err)
	}
	defer os.RemoveAll(temporary)
	libraries, err := extractLibraries(serverJAR, temporary)
	if err != nil {
		fatal("extract server libraries: %v", err)
	}
	source := filepath.Join(temporary, "VitisClimateDump.java")
	if err := os.WriteFile(source, []byte(helperSource), 0o600); err != nil {
		fatal("write helper: %v", err)
	}
	classpath := strings.Join(append([]string{remappedJAR}, libraries...), string(os.PathListSeparator))
	compile := exec.Command("javac", "-proc:none", "-cp", classpath, "-d", temporary, source)
	if output, err := compile.CombinedOutput(); err != nil {
		fatal("compile climate helper: %v\n%s", err, output)
	}
	run := exec.Command("java", "-cp", strings.Join([]string{temporary, classpath}, string(os.PathListSeparator)), "VitisClimateDump")
	output, err := run.CombinedOutput()
	if err != nil {
		fatal("run climate helper: %v\n%s", err, output)
	}
	presets, err := parseOutput(output)
	if err != nil {
		fatal("parse climate output: %v", err)
	}
	generated, err := render(presets)
	if err != nil {
		fatal("render generated source: %v", err)
	}
	destination := filepath.Join(root, "internal", "data", "generated", "climate", "presets.go")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		fatal("create output directory: %v", err)
	}
	if err := os.WriteFile(destination, generated, 0o644); err != nil {
		fatal("write output: %v", err)
	}
	fmt.Printf("Generated %s (%d overworld, %d nether climate points)\n", destination, len(presets["minecraft:overworld"]), len(presets["minecraft:nether"]))
}

func extractLibraries(serverJAR, destination string) ([]string, error) {
	archive, err := zip.OpenReader(serverJAR)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	if err := os.MkdirAll(filepath.Join(destination, "libraries"), 0o700); err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || !strings.HasPrefix(entry.Name, "META-INF/libraries/") || !strings.HasSuffix(entry.Name, ".jar") {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, err
		}
		target := filepath.Join(destination, "libraries", filepath.Base(entry.Name))
		file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			reader.Close()
			return nil, err
		}
		_, copyErr := io.Copy(file, io.LimitReader(reader, 128<<20))
		closeErr := file.Close()
		readerErr := reader.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if readerErr != nil {
			return nil, readerErr
		}
		paths = append(paths, target)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("server bundle has no libraries")
	}
	return paths, nil
}

func parseOutput(output []byte) (map[string][]point, error) {
	presets := make(map[string][]point)
	current := ""
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if index := strings.Index(line, "VITIS_PRESET\t"); index >= 0 {
			current = strings.TrimSpace(line[index+len("VITIS_PRESET\t"):])
			continue
		}
		index := strings.Index(line, "VITIS_POINT\t")
		if index < 0 {
			continue
		}
		if current == "" {
			return nil, fmt.Errorf("point before preset")
		}
		fields := strings.Split(line[index+len("VITIS_POINT\t"):], "\t")
		if len(fields) != 14 {
			return nil, fmt.Errorf("preset %s: got %d point fields", current, len(fields))
		}
		var value point
		copy(value.values[:], fields)
		for _, raw := range value.values[:13] {
			if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
				return nil, fmt.Errorf("invalid coordinate %q: %w", raw, err)
			}
		}
		presets[current] = append(presets[current], value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(presets["minecraft:overworld"]) == 0 || len(presets["minecraft:nether"]) == 0 {
		return nil, fmt.Errorf("required presets are missing")
	}
	return presets, nil
}

func render(presets map[string][]point) ([]byte, error) {
	var output strings.Builder
	output.WriteString("// Code generated by internal/data/generator/climate; DO NOT EDIT.\n\npackage climate\n\n")
	output.WriteString("// Parameter is one quantized climate interval.\ntype Parameter struct { Min, Max int64 }\n\n")
	output.WriteString("// Point maps seven climate intervals to a biome registry name.\ntype Point struct { Parameters [7]Parameter; Biome string }\n\n")
	for _, preset := range []struct{ id, name string }{{"minecraft:overworld", "Overworld"}, {"minecraft:nether", "Nether"}} {
		fmt.Fprintf(&output, "// %s contains the ordered %s climate preset.\nvar %s = [...]Point{\n", preset.name, preset.id, preset.name)
		for _, point := range presets[preset.id] {
			fmt.Fprintf(&output, "{Parameters: [7]Parameter{{%s,%s},{%s,%s},{%s,%s},{%s,%s},{%s,%s},{%s,%s},{%s,%s}}, Biome: %q},\n",
				point.values[0], point.values[1], point.values[2], point.values[3], point.values[4], point.values[5], point.values[6], point.values[7],
				point.values[8], point.values[9], point.values[10], point.values[11], point.values[12], point.values[12], point.values[13])
		}
		output.WriteString("}\n\n")
	}
	return format.Source([]byte(output.String()))
}

func projectRoot() string {
	directory, err := os.Getwd()
	if err != nil {
		fatal("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			fatal("project root not found")
		}
		directory = parent
	}
}

func fatal(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
