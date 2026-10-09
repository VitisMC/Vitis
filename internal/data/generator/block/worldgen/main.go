//go:build ignore

// Generates exact static generation traits from the local versioned Minecraft server.
package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	genblock "github.com/vitismc/vitis/internal/data/generated/block"
	"github.com/vitismc/vitis/internal/datapack"
)

//go:embed DumpBlockTraits.java
var exporter []byte

//go:embed DumpMultifaceFixtures.java
var fixtureExporter []byte

//go:embed DumpStructureBeard.java
var beardExporter []byte

//go:embed DumpNetherFossilSites.java
var fossilSiteExporter []byte

//go:embed DumpNetherFossilTemplates.java
var fossilTemplateExporter []byte

//go:embed DumpLakeFixtures.java
var lakeExporter []byte

//go:embed DumpPointedDripstoneFixtures.java
var pointedDripstoneExporter []byte

//go:embed DumpDripstoneClusterFixtures.java
var dripstoneClusterExporter []byte

//go:embed DumpLargeDripstoneFixtures.java
var largeDripstoneExporter []byte

//go:embed DumpGeodeFixtures.java
var geodeExporter []byte

//go:embed DumpIcebergFixtures.java
var icebergExporter []byte

// stateTraits binds exported metadata to validated Vitis state properties.
type stateTraits struct {
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties"`
	Traits     uint32            `json:"traits"`
}

// main reports generation failures.
func main() {
	version := flag.String("version", "1.21.4", "Minecraft version")
	flag.Parse()
	if err := generate(*version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// projectRoot locates the repository without depending on the invocation directory.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found")
		}
		dir = parent
	}
}

// extractLibraries extracts only jar dependencies into a temporary flat classpath directory.
func extractLibraries(server, directory string) error {
	archive, err := zip.OpenReader(server)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, "META-INF/libraries/") || !strings.HasSuffix(entry.Name, ".jar") {
			continue
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.Create(filepath.Join(directory, filepath.Base(entry.Name)))
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		inputErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if inputErr != nil {
			return inputErr
		}
	}
	return nil
}

// generate validates every vanilla state before writing a deterministic checked-in Go table.
func generate(version string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "vitis-block-traits-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	libraries := filepath.Join(temporary, "libraries")
	if err := os.Mkdir(libraries, 0755); err != nil {
		return err
	}
	if err := extractLibraries(filepath.Join(root, ".mc-decompiled", "downloads", version, "server.jar"), libraries); err != nil {
		return err
	}
	source := filepath.Join(temporary, "DumpBlockTraits.java")
	if err := os.WriteFile(source, exporter, 0644); err != nil {
		return err
	}
	fixtureSource := filepath.Join(temporary, "DumpMultifaceFixtures.java")
	if err := os.WriteFile(fixtureSource, fixtureExporter, 0644); err != nil {
		return err
	}
	beardSource := filepath.Join(temporary, "DumpStructureBeard.java")
	if err := os.WriteFile(beardSource, beardExporter, 0644); err != nil {
		return err
	}
	fossilSource := filepath.Join(temporary, "DumpNetherFossilSites.java")
	if err := os.WriteFile(fossilSource, fossilSiteExporter, 0644); err != nil {
		return err
	}
	fossilTemplateSource := filepath.Join(temporary, "DumpNetherFossilTemplates.java")
	if err := os.WriteFile(fossilTemplateSource, fossilTemplateExporter, 0644); err != nil {
		return err
	}
	lakeSource := filepath.Join(temporary, "DumpLakeFixtures.java")
	if err := os.WriteFile(lakeSource, lakeExporter, 0644); err != nil {
		return err
	}
	pointedSource := filepath.Join(temporary, "DumpPointedDripstoneFixtures.java")
	if err := os.WriteFile(pointedSource, pointedDripstoneExporter, 0644); err != nil {
		return err
	}
	clusterSource := filepath.Join(temporary, "DumpDripstoneClusterFixtures.java")
	if err := os.WriteFile(clusterSource, dripstoneClusterExporter, 0644); err != nil {
		return err
	}
	largeSource := filepath.Join(temporary, "DumpLargeDripstoneFixtures.java")
	if err := os.WriteFile(largeSource, largeDripstoneExporter, 0644); err != nil {
		return err
	}
	geodeSource := filepath.Join(temporary, "DumpGeodeFixtures.java")
	if err := os.WriteFile(geodeSource, geodeExporter, 0644); err != nil {
		return err
	}
	icebergSource := filepath.Join(temporary, "DumpIcebergFixtures.java")
	if err := os.WriteFile(icebergSource, icebergExporter, 0644); err != nil {
		return err
	}
	classpath := filepath.Join(root, ".mc-decompiled", version+"-remapped.jar") + string(os.PathListSeparator) + filepath.Join(libraries, "*")
	compile := exec.Command("javac", "-cp", classpath, "-d", temporary, source, fixtureSource, beardSource, fossilSource, fossilTemplateSource, lakeSource, pointedSource, clusterSource, largeSource, geodeSource, icebergSource)
	compile.Dir = temporary
	if output, err := compile.CombinedOutput(); err != nil {
		return fmt.Errorf("compile block exporter: %w\n%s", err, output)
	}
	jsonPath := filepath.Join(temporary, "traits.json")
	run := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpBlockTraits", jsonPath)
	run.Dir = temporary
	if output, err := run.CombinedOutput(); err != nil {
		return fmt.Errorf("run block exporter: %w\n%s", err, output)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	var states []stateTraits
	if err := json.Unmarshal(data, &states); err != nil {
		return err
	}
	if len(states) != genblock.TotalStates {
		return fmt.Errorf("Minecraft has %d states, Vitis has %d", len(states), genblock.TotalStates)
	}
	traits := make([]uint32, genblock.TotalStates)
	seen := make([]bool, len(traits))
	for _, entry := range states {
		state, err := block.BlockStateFromProperties(entry.Name, entry.Properties)
		if err != nil {
			return err
		}
		if seen[state.ID()] {
			return fmt.Errorf("duplicate state %s %v", entry.Name, entry.Properties)
		}
		seen[state.ID()] = true
		traits[state.ID()] = entry.Traits
	}
	var output bytes.Buffer
	fmt.Fprintln(&output, "// Code generated by internal/data/generator/block/worldgen; DO NOT EDIT.")
	fmt.Fprintln(&output, "\npackage block\n\n// GenerationEmissionShift locates the four-bit light emission field in GenerationTraits.")
	fmt.Fprintln(&output, "const GenerationEmissionShift = 17")
	fmt.Fprintln(&output, "\n// GenerationTraits stores support/collision face masks, replacement/fluid/solid flags and emission.")
	fmt.Fprintln(&output, "// Face bits follow down, up, north, south, west, east. Flags start at bit 12.")
	fmt.Fprintln(&output, "var GenerationTraits = [...]uint32{")
	for index, value := range traits {
		if index%16 == 0 {
			output.WriteByte('\t')
		}
		fmt.Fprintf(&output, "0x%x,", value)
		if index%16 == 15 || index == len(traits)-1 {
			output.WriteByte('\n')
		}
	}
	fmt.Fprintln(&output, "}")
	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return err
	}
	path := filepath.Join(root, "internal", "data", "generated", "block", "generation_traits.go")
	fixturePath := filepath.Join(temporary, "multiface.json")
	manager := &datapack.Manager{}
	if err := manager.Reload([]string{filepath.Join(root, ".mcdata", version, "vanilla")}); err != nil {
		return err
	}
	fire, err := manager.Current().ResolveTag("block", "minecraft:fire")
	if err != nil {
		return err
	}
	fireJSON, err := json.Marshal(fire)
	if err != nil {
		return err
	}
	firePath := filepath.Join(temporary, "fire.json")
	if err := os.WriteFile(firePath, fireJSON, 0644); err != nil {
		return err
	}
	fixtureRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpMultifaceFixtures", fixturePath, firePath)
	fixtureRun.Dir = temporary
	if output, err := fixtureRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run multiface fixture exporter: %w\n%s", err, output)
	}
	fixtures, err := os.ReadFile(fixturePath)
	if err != nil {
		return err
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, fixtures, "", "  "); err != nil {
		return err
	}
	kernelPath := filepath.Join(temporary, "beard-kernel.bin")
	beardFixturePath := filepath.Join(temporary, "beard-cases.json")
	beardRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpStructureBeard", kernelPath, beardFixturePath)
	beardRun.Dir = temporary
	if output, err := beardRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run structure beard exporter: %w\n%s", err, output)
	}
	kernel, err := os.ReadFile(kernelPath)
	if err != nil {
		return err
	}
	if len(kernel) != 13824*4 {
		return fmt.Errorf("unexpected beard kernel size %d", len(kernel))
	}
	var beardGo bytes.Buffer
	fmt.Fprintln(&beardGo, "// Code generated by internal/data/generator/block/worldgen; DO NOT EDIT.\n\npackage worldgen\n\n// structureBeardKernel retains Minecraft 1.21.4 float coefficients as raw bits.\nvar structureBeardKernel = [...]uint32{")
	for index := 0; index < len(kernel); index += 4 {
		if index%64 == 0 {
			beardGo.WriteByte('\t')
		}
		fmt.Fprintf(&beardGo, "0x%x,", binary.LittleEndian.Uint32(kernel[index:index+4]))
		if index%64 == 60 {
			beardGo.WriteByte('\n')
		}
	}
	fmt.Fprintln(&beardGo, "}")
	beardFormatted, err := format.Source(beardGo.Bytes())
	if err != nil {
		return err
	}
	beardCases, err := os.ReadFile(beardFixturePath)
	if err != nil {
		return err
	}
	var beardIndented bytes.Buffer
	if err := json.Indent(&beardIndented, beardCases, "", "  "); err != nil {
		return err
	}
	fossilPath := filepath.Join(temporary, "fossil-sites.json")
	fossilRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpNetherFossilSites", fossilPath)
	fossilRun.Dir = temporary
	if output, err := fossilRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run Nether fossil exporter: %w\n%s", err, output)
	}
	fossilCases, err := os.ReadFile(fossilPath)
	if err != nil {
		return err
	}
	var fossilIndented bytes.Buffer
	if err := json.Indent(&fossilIndented, fossilCases, "", "  "); err != nil {
		return err
	}
	templatePath := filepath.Join(temporary, "fossil-templates.json")
	templateRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpNetherFossilTemplates", templatePath, filepath.Join(root, ".mcdata", version, "vanilla", "data", "minecraft", "structure", "nether_fossils"))
	templateRun.Dir = temporary
	if output, err := templateRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run Nether fossil template exporter: %w\n%s", err, output)
	}
	templateCases, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	var templateIndented bytes.Buffer
	if err := json.Indent(&templateIndented, templateCases, "", "  "); err != nil {
		return err
	}
	lakePath := filepath.Join(temporary, "lake-cases.json")
	lakeRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpLakeFixtures", lakePath)
	lakeRun.Dir = temporary
	if output, err := lakeRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run lake fixture exporter: %w\n%s", err, output)
	}
	lakeCases, err := os.ReadFile(lakePath)
	if err != nil {
		return err
	}
	var lakeIndented bytes.Buffer
	if err := json.Indent(&lakeIndented, lakeCases, "", "  "); err != nil {
		return err
	}
	pointedPath := filepath.Join(temporary, "pointed-dripstone-cases.json")
	pointedRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpPointedDripstoneFixtures", pointedPath)
	pointedRun.Dir = temporary
	if output, err := pointedRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run pointed dripstone fixture exporter: %w\n%s", err, output)
	}
	pointedCases, err := os.ReadFile(pointedPath)
	if err != nil {
		return err
	}
	var pointedIndented bytes.Buffer
	if err := json.Indent(&pointedIndented, pointedCases, "", "  "); err != nil {
		return err
	}
	clusterPath := filepath.Join(temporary, "dripstone-cluster-cases.json")
	clusterRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpDripstoneClusterFixtures", clusterPath)
	clusterRun.Dir = temporary
	if output, err := clusterRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run dripstone cluster fixture exporter: %w\n%s", err, output)
	}
	clusterCases, err := os.ReadFile(clusterPath)
	if err != nil {
		return err
	}
	var clusterIndented bytes.Buffer
	if err := json.Indent(&clusterIndented, clusterCases, "", "  "); err != nil {
		return err
	}
	largePath := filepath.Join(temporary, "large-dripstone-cases.json")
	largeRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpLargeDripstoneFixtures", largePath)
	largeRun.Dir = temporary
	if output, err := largeRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run large dripstone fixture exporter: %w\n%s", err, output)
	}
	largeCases, err := os.ReadFile(largePath)
	if err != nil {
		return err
	}
	var largeIndented bytes.Buffer
	if err := json.Indent(&largeIndented, largeCases, "", "  "); err != nil {
		return err
	}
	geodePath := filepath.Join(temporary, "geode-cases.json")
	geodeRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpGeodeFixtures", geodePath)
	geodeRun.Dir = temporary
	if output, err := geodeRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run geode fixture exporter: %w\n%s", err, output)
	}
	geodeCases, err := os.ReadFile(geodePath)
	if err != nil {
		return err
	}
	var geodeIndented bytes.Buffer
	if err := json.Indent(&geodeIndented, geodeCases, "", "  "); err != nil {
		return err
	}
	icebergPath := filepath.Join(temporary, "iceberg-cases.json")
	icebergRun := exec.Command("java", "-cp", temporary+string(os.PathListSeparator)+classpath, "DumpIcebergFixtures", icebergPath)
	icebergRun.Dir = temporary
	if output, err := icebergRun.CombinedOutput(); err != nil {
		return fmt.Errorf("run iceberg fixture exporter: %w\n%s", err, output)
	}
	icebergCases, err := os.ReadFile(icebergPath)
	if err != nil {
		return err
	}
	var icebergIndented bytes.Buffer
	if err := json.Indent(&icebergIndented, icebergCases, "", "  "); err != nil {
		return err
	}
	fixtureOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "multiface_1_21_4.json")
	beardOutput := filepath.Join(root, "internal", "world", "worldgen", "structure_beard_generated.go")
	beardCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "structure_beard_1_21_4.json")
	fossilCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "nether_fossil_sites_1_21_4.json")
	fossilTemplateOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "nether_fossil_templates_1_21_4.json")
	lakeCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "lake_1_21_4.json")
	pointedCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "pointed_dripstone_1_21_4.json")
	clusterCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "dripstone_cluster_1_21_4.json")
	largeCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "large_dripstone_1_21_4.json")
	geodeCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "geode_1_21_4.json")
	icebergCasesOutput := filepath.Join(root, "internal", "world", "worldgen", "testdata", "iceberg_1_21_4.json")
	if err := os.MkdirAll(filepath.Dir(fixtureOutput), 0755); err != nil {
		return err
	}
	// Finish both exporters before publishing either validated artifact.
	if err := writeGeneratedFile(path, formatted); err != nil {
		return err
	}
	if err := writeGeneratedFile(fixtureOutput, append(indented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(beardOutput, beardFormatted); err != nil {
		return err
	}
	if err := writeGeneratedFile(beardCasesOutput, append(beardIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(fossilCasesOutput, append(fossilIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(fossilTemplateOutput, append(templateIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(lakeCasesOutput, append(lakeIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(pointedCasesOutput, append(pointedIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(clusterCasesOutput, append(clusterIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(largeCasesOutput, append(largeIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(geodeCasesOutput, append(geodeIndented.Bytes(), '\n')); err != nil {
		return err
	}
	if err := writeGeneratedFile(icebergCasesOutput, append(icebergIndented.Bytes(), '\n')); err != nil {
		return err
	}
	fmt.Printf("Generated %s (%d validated states)\n", path, len(traits))
	return nil
}

// writeGeneratedFile atomically replaces a generated artifact in its destination directory.
func writeGeneratedFile(path string, contents []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".worldgen-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
