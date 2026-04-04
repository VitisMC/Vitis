package config

import (
	"strings"
	"testing"
)

func TestValidateAcceptsDefaultWorldSubsystemConfig(t *testing.T) {
	cfg := Default()
	if err := Validate(cfg); err != nil {
		t.Fatalf("default config should validate, got: %v", err)
	}
}

func TestValidateRejectsInvalidWorldDefaultName(t *testing.T) {
	cfg := Default()
	cfg.World.DefaultWorldName = ""

	err := Validate(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "world.default_world_name") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRejectsInvalidChunkLoadWorkers(t *testing.T) {
	cfg := Default()
	cfg.World.ChunkLoadWorkers = 0

	err := Validate(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "world.chunk_load_workers") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
