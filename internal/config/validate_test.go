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

func TestValidateRejectsInvalidNetworkConnectionLimit(t *testing.T) {
	cfg := Default()
	cfg.Network.MaxConnections = 0

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "network.max_connections") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRejectsInvalidNetworkAdmissionLimit(t *testing.T) {
	cfg := Default()
	cfg.Network.AdmissionRatePerSecond = 0

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "network.admission_rate_per_second") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRejectsWeakRCONPassword(t *testing.T) {
	cfg := Default()
	cfg.RCON.Enabled = true
	cfg.RCON.Password = "short"

	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted a weak RCON password")
	}
}

// TestRCONPasswordStrengthRejectsPredictableSecrets checks enabled RCON validation.
func TestRCONPasswordStrengthRejectsPredictableSecrets(t *testing.T) {
	for _, password := range []string{
		"", "short", "aaaaaaaaaaaaaaaa", "abcdefghijklmnop",
		"password123456789", "ChangeMe-123456789", "  StrongToken-7Kq4!  ",
	} {
		cfg := Default()
		cfg.RCON.Enabled = true
		cfg.RCON.Password = password
		if err := Validate(cfg); err == nil {
			t.Errorf("accepted weak RCON password %q", password)
		}
	}
	cfg := Default()
	cfg.RCON.Enabled = true
	cfg.RCON.Password = "Q7v#2mT!9xL@4pR$"
	if err := Validate(cfg); err != nil {
		t.Fatalf("rejected strong RCON password: %v", err)
	}
}

func TestDefaultUsesProductionSecuritySettings(t *testing.T) {
	cfg := Default()
	if !cfg.Server.OnlineMode {
		t.Fatal("online mode must be enabled by default")
	}
	if cfg.RCON.Address != "127.0.0.1" {
		t.Fatalf("RCON address = %q; want loopback", cfg.RCON.Address)
	}
}

func TestValidateRejectsDisabledApplicationRateLimits(t *testing.T) {
	cfg := Default()
	cfg.Security.ChatMessagesPer10Seconds = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted a disabled chat rate limit")
	}
}

func TestValidateRejectsUnknownDifficulty(t *testing.T) {
	cfg := Default()
	cfg.World.Difficulty = "nightmare"
	if err := Validate(cfg); err == nil {
		t.Fatal("unknown difficulty was accepted")
	}
}

// TestValidateRejectsInvalidAutosaveBounds covers bounded background persistence.
func TestValidateRejectsInvalidAutosaveBounds(t *testing.T) {
	for _, field := range []string{"interval", "batch", "queue"} {
		cfg := Default()
		switch field {
		case "interval":
			cfg.World.AutosaveIntervalTicks = 0
		case "batch":
			cfg.World.AutosaveBatchPerTick = 0
		case "queue":
			cfg.World.AutosaveQueueCapacity = 0
		}
		if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "world.autosave_") {
			t.Errorf("%s: expected autosave validation error, got %v", field, err)
		}
	}
}
