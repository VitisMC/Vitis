package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorldgenVersionResources exercises CLI resource selection before any output is written.
func TestWorldgenVersionResources(t *testing.T) {
	if os.Getenv("VITIS_WORLDGEN_CLI_TEST") == "1" {
		flag.CommandLine = flag.NewFlagSet("worldgen", flag.ExitOnError)
		os.Args = append([]string{"worldgen"}, os.Args[3:]...)
		main()
		return
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "default", want: filepath.Join(".mcdata", "1.21.4", "vanilla", "data", "minecraft", "worldgen")},
		{name: "version", args: []string{"-version", "25w01a"}, want: filepath.Join(".mcdata", "25w01a", "vanilla", "data", "minecraft", "worldgen")},
		{name: "explicit resources", args: []string{"-version", "25w01a", "-resources", "custom-resources"}, want: "custom-resources"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], append([]string{"-test.run=^TestWorldgenVersionResources$", "--"}, test.args...)...)
			command.Dir = t.TempDir()
			command.Env = append(os.Environ(), "VITIS_WORLDGEN_CLI_TEST=1")
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), filepath.Join(test.want, "noise_settings", "overworld.json")) {
				t.Fatalf("unexpected input lookup: %v\n%s", err, output)
			}
		})
	}
}

// TestSurfaceTerminalRuleStopsSequence rejects dead branches after the first unconditional match.
func TestSurfaceTerminalRuleStopsSequence(t *testing.T) {
	rule := json.RawMessage(`{"type":"minecraft:sequence","sequence":[{"type":"minecraft:block","result_state":{"Name":"minecraft:stone"}},{"type":"unsupported"}]}`)
	emitter := &surfaceEmitter{}
	if !emitter.rule(rule) || len(emitter.states) != 1 {
		t.Fatal("terminal block did not stop the sequence")
	}
	conditional := json.RawMessage(`{"type":"minecraft:condition","if_true":{"type":"minecraft:above_preliminary_surface"},"then_run":{"type":"minecraft:block","result_state":{"Name":"minecraft:stone"}}}`)
	emitter = &surfaceEmitter{}
	if emitter.rule(conditional) {
		t.Fatal("conditional branch incorrectly removed the fallback")
	}
	emitter = &surfaceEmitter{deep: true}
	if emitter.rule(conditional) || emitter.body.Len() != 0 {
		t.Fatal("deep surface emitted a proven-false branch")
	}
}

// TestDensityProductBoundsRetainCalculation checks the emitter does not overwrite computed bounds.
func TestDensityProductBoundsRetainCalculation(t *testing.T) {
	emitter := &densityEmitter{nodes: []*densityNode{{kind: "mul", children: map[string]int{"argument1": 1, "argument2": 2}}}}
	var bind, functions bytes.Buffer
	emitter.emitNode(0, &bind, &functions)
	if !strings.Contains(bind.String(), "g.lower[0],g.upper[0]=low,high") {
		t.Fatal("product bounds were omitted")
	}
	if strings.Contains(bind.String(), "g.lower[0],g.upper[0]=g.lower[0],g.upper[0]") {
		t.Fatal("product bounds emitted self-assignment")
	}
}
