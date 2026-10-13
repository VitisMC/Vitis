package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeGenerationCommand runs the real Makefile with isolated input preparation.
func makeGenerationCommand(t *testing.T, updater string, args ...string) (*exec.Cmd, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Bash Make target is tested on Unix; Windows uses the native PowerShell updater")
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Skip("Make unavailable")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Bash unavailable")
	}
	root := t.TempDir()
	makefile, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), makefile, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "linux"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "linux", "update_version.sh"), []byte(updater), 0755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(makePath, append([]string{"--no-print-directory", "generate"}, args...)...)
	command.Dir = root
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if name != "MC_VERSION" && name != "MAKEFLAGS" && name != "MFLAGS" && name != "MAKELEVEL" && name != "MAKEOVERRIDES" {
			command.Env = append(command.Env, value)
		}
	}
	return command, root
}

// TestMakeGenerateVersion verifies default, environment and command-line version selection.
func TestMakeGenerateVersion(t *testing.T) {
	const updater = "#!/usr/bin/env bash\nset -eu\nprintf '%s\\n' \"$#\" \"$1\" > received\n"
	for _, test := range []struct {
		name, environment, argument, want string
	}{
		{name: "default", want: "1.21.4"},
		{name: "release", argument: "1.21.5", want: "1.21.5"},
		{name: "snapshot", argument: "25w01a", want: "25w01a"},
		{name: "environment", environment: "1.21.5", want: "1.21.5"},
		{name: "command line wins", environment: "1.21.5", argument: "1.21.4", want: "1.21.4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var args []string
			if test.argument != "" {
				args = append(args, "MC_VERSION="+test.argument)
			}
			command, root := makeGenerationCommand(t, updater, args...)
			if test.environment != "" {
				command.Env = append(command.Env, "MC_VERSION="+test.environment)
			}
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("make generate: %v\n%s", err, output)
			}
			received, err := os.ReadFile(filepath.Join(root, "received"))
			if err != nil || string(received) != "1\n"+test.want+"\n" {
				t.Fatalf("updater arguments = %q, error = %v", received, err)
			}
		})
	}
}

// TestMakeGenerateRejectsInvalidVersion checks validation precedes downloads and shell evaluation.
func TestMakeGenerateRejectsInvalidVersion(t *testing.T) {
	updater, err := os.ReadFile(filepath.Join("linux", "update_version.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"", "../1.21.4", "1.21.4 extra", "1.21.4; touch injected", "\"`touch injected`\""} {
		t.Run(version, func(t *testing.T) {
			command, root := makeGenerationCommand(t, string(updater), "MC_VERSION="+version)
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "usage:") {
				t.Fatalf("invalid version accepted or wrong error: %v\n%s", err, output)
			}
			for _, name := range []string{"injected", ".mcdata", ".mc-decompiled"} {
				if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
					t.Fatalf("invalid version created %s: %v", name, err)
				}
			}
		})
	}
}

// TestMakeGeneratePropagatesFailure prevents preparation errors being reported as success.
func TestMakeGeneratePropagatesFailure(t *testing.T) {
	command, _ := makeGenerationCommand(t, "#!/usr/bin/env bash\nexit 42\n", "MC_VERSION=1.21.5")
	if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "42") {
		t.Fatalf("updater failure lost: %v\n%s", err, output)
	}
}
