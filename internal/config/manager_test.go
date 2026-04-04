package config

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestManagerReload(t *testing.T) {
	path := writeConfigFile(t, `
server:
  host: "0.0.0.0"
  port: 25565
  max_players: 120
`)

	manager := NewManager(path)
	if _, err := manager.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	var hookCalls atomic.Int32
	var oldPort atomic.Int32
	var newPort atomic.Int32

	err := manager.RegisterReloadHook(func(oldConfig *Config, next *Config) error {
		hookCalls.Add(1)
		if oldConfig == nil || next == nil {
			return fmt.Errorf("nil hook config")
		}
		oldPort.Store(int32(oldConfig.Server.Port))
		newPort.Store(int32(next.Server.Port))
		return nil
	})
	if err != nil {
		t.Fatalf("register hook failed: %v", err)
	}

	mustWrite(t, path, `
server:
  host: "0.0.0.0"
  port: 25566
  max_players: 120
`)

	cfg, err := manager.Reload()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if cfg.Server.Port != 25566 {
		t.Fatalf("expected reloaded port 25566, got %d", cfg.Server.Port)
	}
	if manager.Get().Server.Port != 25566 {
		t.Fatalf("manager did not publish new config, got %d", manager.Get().Server.Port)
	}
	if manager.GetNetwork().CompressionThreshold != 256 {
		t.Fatalf("expected default network compression threshold, got %d", manager.GetNetwork().CompressionThreshold)
	}
	if hookCalls.Load() != 1 {
		t.Fatalf("expected hook calls 1, got %d", hookCalls.Load())
	}
	if oldPort.Load() != 25565 || newPort.Load() != 25566 {
		t.Fatalf("unexpected hook ports old=%d new=%d", oldPort.Load(), newPort.Load())
	}
}
