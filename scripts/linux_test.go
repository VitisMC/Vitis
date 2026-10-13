package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLinuxScripts checks repository discovery, argument forwarding and build failures.
func TestLinuxScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses native PowerShell scripts")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash unavailable")
	}
	binary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		script string
		args   []string
		code   string
		calls  int
	}{
		{script: "build.sh", args: []string{"custom output"}, calls: 1},
		{script: "run.sh", args: []string{"-config", "path with spaces.yaml"}, calls: 2},
		{script: "build.sh", code: "42", calls: 1},
		{script: "run.sh", code: "42", calls: 1},
	} {
		t.Run(test.script+test.code, func(t *testing.T) {
			root := t.TempDir()
			scripts := filepath.Join(root, "scripts", "linux")
			bin := filepath.Join(root, "bin")
			for _, dir := range []string{scripts, bin} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(filepath.Join("linux", test.script))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(scripts, test.script)
			if err := os.WriteFile(path, data, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "go"), binary, 0755); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(root, "calls.jsonl")
			command := exec.Command(bash, append([]string{path}, test.args...)...)
			command.Dir = t.TempDir()
			command.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "VITIS_PS_STUB=1", "VITIS_PS_LOG="+logPath, "VITIS_PS_EXIT="+test.code)
			output, runErr := command.CombinedOutput()
			if (runErr != nil) != (test.code != "") {
				t.Fatalf("unexpected exit: %v\n%s", runErr, output)
			}
			if test.code != "" && command.ProcessState.ExitCode() != 42 {
				t.Fatalf("exit code = %d, want 42", command.ProcessState.ExitCode())
			}
			data, err = os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != test.calls {
				t.Fatalf("calls = %s, output = %s", data, output)
			}
			if !strings.Contains(lines[0], `"./cmd/vitis"`) {
				t.Fatalf("build must use the complete server package: %s", lines[0])
			}
			if test.code != "" {
				return
			}
			if test.script == "build.sh" {
				if _, err := os.Stat(filepath.Join(root, "custom output")); err != nil || !strings.Contains(lines[0], `"-s -w"`) {
					t.Fatalf("output path or linker arguments lost: %v, %s", err, lines[0])
				}
			} else {
				var arguments []string
				if err := json.Unmarshal([]byte(lines[1]), &arguments); err != nil || len(arguments) != 2 || arguments[0] != "-config" || arguments[1] != "path with spaces.yaml" {
					t.Fatalf("server arguments = %s, error = %v", lines[1], err)
				}
			}
		})
	}
}
