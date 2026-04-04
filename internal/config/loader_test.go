package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	path := writeConfigFile(t, `
server:
  host: "127.0.0.1"
  port: 25565
  max_players: 120
network:
  read_timeout_seconds: 20
  write_timeout_seconds: 20
  idle_timeout_seconds: 90
  compression_threshold: 256
  max_packet_size: 2097152
  max_inbound_queue_capacity: 3000
tick:
  target_tps: 20
  max_catch_up_ticks: 5
world:
  view_distance: 12
  simulation_distance: 10
logging:
  level: "info"
  format: "json"
  file_path: "logs/vitis.log"
  enable_color: false
performance:
  io_worker_pool_size: 4
  packet_worker_pool_size: 8
  read_buffer_size: 65536
  write_buffer_size: 65536
  outbound_queue_size: 1024
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Server.Port != 25565 {
		t.Fatalf("unexpected port: %d", cfg.Server.Port)
	}
	if cfg.World.ViewDistance != 12 {
		t.Fatalf("unexpected view distance: %d", cfg.World.ViewDistance)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("unexpected logging format: %q", cfg.Logging.Format)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeConfigFile(t, `
server:
  max_players: 64
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Server.Port != 25565 {
		t.Fatalf("expected default port 25565, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected default host, got %q", cfg.Server.Host)
	}
	if cfg.Network.CompressionThreshold != 256 {
		t.Fatalf("expected default compression threshold, got %d", cfg.Network.CompressionThreshold)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	path := writeConfigFile(t, `
server:
  host: "0.0.0.0"
  port: 70000
  max_players: 100
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected load error")
	}
	if !strings.Contains(err.Error(), "server.port") {
		t.Fatalf("expected port validation error, got %v", err)
	}
}

func TestLoaderReload(t *testing.T) {
	path := writeConfigFile(t, `
server:
  host: "0.0.0.0"
  port: 25565
  max_players: 100
`)
	loader := NewLoader(path)

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	if cfg.Server.Port != 25565 {
		t.Fatalf("unexpected initial port: %d", cfg.Server.Port)
	}

	mustWrite(t, path, `
server:
  host: "0.0.0.0"
  port: 25566
  max_players: 100
`)

	cfg, err = loader.Reload()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if cfg.Server.Port != 25566 {
		t.Fatalf("expected reloaded port 25566, got %d", cfg.Server.Port)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "vitis.yaml")
	mustWrite(t, path, content)
	return path
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write file %q failed: %v", path, err)
	}
}
