package command

import "testing"

// spawnControl records the complete spawn mutation requested by a command.
type spawnControl struct {
	*mockServer
	x, y, z int
	angle   float32
	calls   int
}

// SetWorldSpawnAngle captures validated coordinates and yaw.
func (s *spawnControl) SetWorldSpawnAngle(x, y, z int, angle float32) error {
	s.x, s.y, s.z, s.angle = x, y, z, angle
	s.calls++
	return nil
}

// consoleSpawnSender supplies the command sender contract without a player position.
type consoleSpawnSender struct{ messages []string }

// Name returns the console label.
func (s *consoleSpawnSender) Name() string { return "console" }

// SendMessage records command feedback.
func (s *consoleSpawnSender) SendMessage(message string) { s.messages = append(s.messages, message) }

// HasPermission permits administrator commands in this fixture.
func (s *consoleSpawnSender) HasPermission(int) bool { return true }

// IsPlayer identifies the console source.
func (s *consoleSpawnSender) IsPlayer() bool { return false }

// TestWorldSpawnConsoleAndNegativeAngle verifies explicit console coordinates and yaw wrapping.
func TestWorldSpawnConsoleAndNegativeAngle(t *testing.T) {
	control := &spawnControl{mockServer: newMockServer()}
	sender := &consoleSpawnSender{}
	command := cmdSetworldspawn(control)
	if err := command.Execute(&Context{Sender: sender, Args: []string{"-1.2", "70", "5", "-541"}}); err != nil {
		t.Fatal(err)
	}
	if control.calls != 1 || control.x != -2 || control.y != 70 || control.z != 5 || control.angle != 179 {
		t.Fatalf("spawn=%+v", control)
	}
	for _, args := range [][]string{nil, {"~", "70", "5"}, {"NaN", "70", "5"}, {"1", "70", "5", "NaN"}} {
		if err := command.Execute(&Context{Sender: sender, Args: args}); err != nil {
			t.Fatal(err)
		}
		if control.calls != 1 {
			t.Fatalf("invalid console coordinates accepted: %v", args)
		}
	}
}
