package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestMain also provides an isolated native executable for platform script tests.
func TestMain(m *testing.M) {
	if os.Getenv("VITIS_PS_STUB") == "1" {
		arguments, err := json.Marshal(os.Args[1:])
		if err != nil {
			os.Exit(99)
		}
		log, err := os.OpenFile(os.Getenv("VITIS_PS_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(99)
		}
		_, writeErr := log.Write(append(arguments, '\n'))
		closeErr := log.Close()
		if writeErr != nil || closeErr != nil {
			os.Exit(99)
		}
		code, _ := strconv.Atoi(os.Getenv("VITIS_PS_EXIT"))
		if len(os.Args) > 1 && os.Args[1] == "build" && code == 0 {
			for i := 2; i+1 < len(os.Args); i++ {
				if os.Args[i] == "-o" {
					executable, err := os.Executable()
					if err != nil {
						os.Exit(99)
					}
					data, err := os.ReadFile(executable)
					if err != nil || os.WriteFile(os.Args[i+1], data, 0755) != nil {
						os.Exit(99)
					}
					break
				}
			}
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// powershellCommand selects PowerShell and gives its caches writable test directories.
func powershellCommand(t *testing.T, script string, args ...string) *exec.Cmd {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell 7 unavailable")
	}
	command := exec.Command(pwsh, append([]string{"-NoProfile", "-NonInteractive", "-File", script}, args...)...)
	command.Env = os.Environ()
	for _, name := range []string{"XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
		command.Env = append(command.Env, name+"="+t.TempDir())
	}
	return command
}

// TestPowerShellScripts checks native build/run arguments, failures and mandatory verification.
func TestPowerShellScripts(t *testing.T) {
	for _, test := range []struct {
		script string
		args   []string
		code   string
		calls  int
	}{
		{script: "build.ps1", args: []string{"custom output.exe"}, calls: 1},
		{script: "run.ps1", args: []string{"-config", "path with spaces.yaml"}, calls: 2},
		{script: "build.ps1", code: "42", calls: 1},
		{script: "run.ps1", code: "42", calls: 1},
		{script: "verify.ps1", calls: 7},
		{script: "verify.ps1", code: "42", calls: 1},
		{script: "measure_worldgen.ps1", args: []string{"metrics.json", "go", "argument with spaces"}, code: "42", calls: 1},
	} {
		t.Run(test.script+test.code, func(t *testing.T) {
			root := t.TempDir()
			scripts := filepath.Join(root, "scripts", "windows")
			if err := os.MkdirAll(scripts, 0755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{test.script, "generation_transaction.ps1"} {
				data, err := os.ReadFile(filepath.Join("windows", name))
				if err != nil || os.WriteFile(filepath.Join(scripts, name), data, 0644) != nil {
					t.Fatalf("copy %s: %v", name, err)
				}
			}
			binary, err := os.ReadFile(os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"go", "go.exe"} {
				if err := os.WriteFile(filepath.Join(bin, name), binary, 0755); err != nil {
					t.Fatal(err)
				}
			}
			command := powershellCommand(t, filepath.Join(scripts, test.script), test.args...)
			command.Dir = t.TempDir()
			if test.script == "measure_worldgen.ps1" {
				command.Dir = root
			}
			logPath := filepath.Join(root, "calls.jsonl")
			command.Env = append(command.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "VITIS_PS_STUB=1", "VITIS_PS_LOG="+logPath, "VITIS_PS_EXIT="+test.code)
			output, runErr := command.CombinedOutput()
			if (runErr != nil) != (test.code != "") {
				t.Fatalf("unexpected exit: %v\n%s", runErr, output)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != test.calls {
				t.Fatalf("calls = %s, output = %s", data, output)
			}
			if test.script == "build.ps1" && test.code == "" {
				if _, err := os.Stat(filepath.Join(root, "custom output.exe")); err != nil || !strings.Contains(lines[0], `"-s -w"`) {
					t.Fatalf("output path or linker arguments lost: %v, %s", err, lines[0])
				}
			}
			if test.script == "run.ps1" && test.code == "" {
				var arguments []string
				if err := json.Unmarshal([]byte(lines[1]), &arguments); err != nil || len(arguments) != 2 || arguments[1] != "path with spaces.yaml" {
					t.Fatalf("server arguments = %s, error = %v", lines[1], err)
				}
			}
			if test.script == "measure_worldgen.ps1" {
				metrics, err := os.ReadFile(filepath.Join(root, "metrics.json"))
				if err != nil || !strings.Contains(string(metrics), `"success": false`) {
					t.Fatalf("failed workload metrics = %s, error = %v", metrics, err)
				}
			}
		})
	}
}

// TestPowerShellGenerationHelpers runs dependency-free download and rollback edge cases.
func TestPowerShellGenerationHelpers(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("windows", "test_windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	command := powershellCommand(t, script, t.TempDir())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell helper tests: %v\n%s", err, output)
	}
}

// TestPowerShellVersionValidation rejects invalid input before dependency checks or writes.
func TestPowerShellVersionValidation(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("windows", "update_version.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"", "../outside", "1.21.4 extra", "1.21.4; Write-Output injected"} {
		command := powershellCommand(t, script, "-Version", version)
		if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "Usage:") {
			t.Fatalf("version %q: %v\n%s", version, err, output)
		}
	}
}
