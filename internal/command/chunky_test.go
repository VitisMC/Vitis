package command

import (
	"testing"

	"github.com/vitismc/vitis/internal/world"
)

type fakeChunkyControl struct {
	shape        string
	x, z, radius int
	state        string
}

func (f *fakeChunkyControl) DefaultChunkyWorld() string { return "world" }
func (f *fakeChunkyControl) StartChunky(_ string, shape string, x, z, radius int) error {
	f.shape, f.x, f.z, f.radius, f.state = shape, x, z, radius, "running"
	return nil
}
func (f *fakeChunkyControl) PauseChunky(string) error    { f.state = "paused"; return nil }
func (f *fakeChunkyControl) ContinueChunky(string) error { f.state = "running"; return nil }
func (f *fakeChunkyControl) CancelChunky(string) error   { f.state = "cancelled"; return nil }
func (f *fakeChunkyControl) ChunkyStatus(name string) (world.PregenerationStatus, error) {
	return world.PregenerationStatus{World: name, State: f.state, Total: 9, Completed: 3, Average: 12, Maximum: 20}, nil
}

// TestChunkySelectionAndProgress verifies command input reaches the world control.
func TestChunkySelectionAndProgress(t *testing.T) {
	control := &fakeChunkyControl{}
	command := cmdChunky(control)
	sender := &mockSender{name: "operator", level: 4}
	for _, args := range [][]string{{"center", "-17", "32"}, {"radius", "64"}, {"shape", "circle"}, {"start"}, {"progress"}, {"pause"}, {"continue"}} {
		if err := command.Execute(&Context{Sender: sender, Args: args}); err != nil {
			t.Fatal(err)
		}
	}
	if control.shape != "circle" || control.x != -17 || control.z != 32 || control.radius != 64 || control.state != "running" {
		t.Fatalf("unexpected command selection: %+v", control)
	}
	if len(sender.messages) != 7 {
		t.Fatalf("missing command replies: %v", sender.messages)
	}
}
