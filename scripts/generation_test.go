// Package scripts exercises the lifecycle of developer tooling.
package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// transactionCommand builds a shell harness using the updater's actual rollback helpers.
func transactionCommand(t *testing.T, root, ending string) *exec.Cmd {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Bash lifecycle is tested on Unix; Windows uses PowerShell lifecycle tests")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash unavailable")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	script := `set -Eeuo pipefail
project_root="$1"
source "$2"
install_generation_traps
snapshot_generation_outputs generated new.json fixtures/reference.json
generation_publishing=1
printf changed >"$project_root/generated/dirty.go"
printf created >"$project_root/generated/created.go"
printf created >"$project_root/new.json"
printf changed >"$project_root/fixtures/reference.json"
` + ending
	return exec.Command(bash, "-c", script, "--", filepath.ToSlash(root), filepath.ToSlash(filepath.Join(dir, "linux", "generation_transaction.sh")))
}

// transactionFixture records uncommitted generated content and an unrelated file.
func transactionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"generated", "fixtures"} {
		if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"generated/dirty.go", "fixtures/reference.json", "keep"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("uncommitted original"), 0640); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// assertTransactionRestored checks bytes, missing outputs and unrelated content after rollback.
func assertTransactionRestored(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"generated/dirty.go", "fixtures/reference.json", "keep"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != "uncommitted original" {
			t.Fatalf("%s = %q, error = %v", name, data, err)
		}
	}
	for _, name := range []string{"generated/created.go", "new.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected output %s: %v", name, err)
		}
	}
}

// TestGenerationRollbackRestoresAllOutputs verifies failures preserve dirty and missing files.
func TestGenerationRollbackRestoresAllOutputs(t *testing.T) {
	root := transactionFixture(t)
	output, err := transactionCommand(t, root, "run_generation_command bash -c 'exit 42'").CombinedOutput()
	if failure, ok := err.(*exec.ExitError); !ok || failure.ExitCode() != 42 {
		t.Fatalf("exit = %v, output = %s", err, output)
	}
	assertTransactionRestored(t, root)
}

// TestGenerationStopForwardsSignalAndRestores verifies the active command stops before rollback.
func TestGenerationStopForwardsSignalAndRestores(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group signals")
	}
	root := transactionFixture(t)
	ending := `run_generation_command bash -c 'trap '\''printf stopped >"$1/stopped"; exit 0'\'' TERM; printf ready >"$1/ready"; sleep 60 & wait' -- "$project_root"`
	command := transactionCommand(t, root, ending)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if failure, ok := err.(*exec.ExitError); !ok || failure.ExitCode() != 143 {
			t.Fatalf("exit = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("updater did not stop")
	}
	if _, err := os.Stat(filepath.Join(root, "stopped")); err != nil {
		t.Fatalf("command was not signalled: %v", err)
	}
	assertTransactionRestored(t, root)
}
