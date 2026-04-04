package session

import (
	"fmt"
	"os"

	"github.com/vitismc/vitis/internal/logger"

	"github.com/vitismc/vitis/internal/command"
)

// consoleSender implements command.Sender for the server console.
type consoleSender struct{}

// NewConsoleSender creates a command.Sender for the server console.
func NewConsoleSender() command.Sender {
	return &consoleSender{}
}

func (c *consoleSender) Name() string {
	return "Server"
}

func (c *consoleSender) SendMessage(text string) {
	clean := stripSectionCodes(text)
	fmt.Println(clean)
}

func (c *consoleSender) HasPermission(level int) bool {
	_ = level
	return true
}

func (c *consoleSender) IsPlayer() bool {
	return false
}

func stripSectionCodes(s string) string {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		if runes[i] == '§' && i+1 < len(runes) {
			i++
			continue
		}
		out = append(out, runes[i])
	}
	return string(out)
}

// StartConsoleReader starts a goroutine that reads lines from stdin and dispatches them as commands.
func StartConsoleReader(registry *command.Registry, stopFunc func()) {
	go func() {
		var buf [4096]byte
		for {
			n, err := readLine(buf[:])
			if err != nil {
				return
			}
			line := string(buf[:n])
			if line == "" {
				continue
			}

			if line == "stop" {
				logger.Info("console: stopping server...")
				stopFunc()
				return
			}

			sender := &consoleSender{}
			if err := registry.Dispatch(sender, line); err != nil {
				logger.Error("console command error", "error", err)
			}
		}
	}()
}

func readLine(buf []byte) (int, error) {
	n := 0
	one := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(one)
		if err != nil {
			return 0, err
		}
		if one[0] == '\n' {
			return n, nil
		}
		if one[0] == '\r' {
			continue
		}
		if n < len(buf) {
			buf[n] = one[0]
			n++
		}
	}
}
