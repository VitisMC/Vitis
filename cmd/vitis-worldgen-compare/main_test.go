package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/chunk/section"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/region"
)

// comparisonWorld creates a valid saved chunk so the CLI reaches resource loading.
func comparisonWorld(t *testing.T, path string) {
	t.Helper()
	if err := level.New("comparison", 0, 64, 0).Save(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "region"), 0755); err != nil {
		t.Fatal(err)
	}
	file, err := region.Create(filepath.Join(path, "region", "r.0.0.mca"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	saved := chunk.New(0, 0)
	saved.SetData(section.NewChunk(0, 0, section.OverworldSections, 0))
	data, err := chunk.EncodeChunkNBT(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.WriteChunkNBT(0, 0, data); err != nil {
		t.Fatal(err)
	}
}

// TestComparisonVersionResources checks versioned defaults and explicit resource precedence.
func TestComparisonVersionResources(t *testing.T) {
	if os.Getenv("VITIS_COMPARISON_CLI_TEST") == "1" {
		flag.CommandLine = flag.NewFlagSet("vitis-worldgen-compare", flag.ExitOnError)
		os.Args = append([]string{"vitis-worldgen-compare"}, os.Args[3:]...)
		main()
		return
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "default", want: filepath.Join(".mcdata", protocol.CurrentMinecraftVersion, "vanilla")},
		{name: "version", args: []string{"-version", "25w01a"}, want: filepath.Join(".mcdata", "25w01a", "vanilla")},
		{name: "explicit resources", args: []string{"-version", "25w01a", "-resources", "custom-resources"}, want: "custom-resources"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			world := filepath.Join(root, "world")
			comparisonWorld(t, world)
			args := append([]string{"-test.run=^TestComparisonVersionResources$", "--", "-world", world}, test.args...)
			command := exec.Command(os.Args[0], args...)
			command.Dir = root
			command.Env = append(os.Environ(), "VITIS_COMPARISON_CLI_TEST=1")
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "load resources:") || !strings.Contains(string(output), test.want) {
				t.Fatalf("unexpected resource lookup: %v\n%s", err, output)
			}
		})
	}
}
