package play

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestLoginPlayRoundtrip(t *testing.T) {
	original := &LoginPlay{
		EntityID:            42,
		IsHardcore:          false,
		DimensionNames:      []string{"minecraft:overworld", "minecraft:the_nether"},
		MaxPlayers:          20,
		ViewDistance:         10,
		SimulationDistance:   10,
		ReducedDebugInfo:    false,
		EnableRespawnScreen: true,
		DoLimitedCrafting:   false,
		DimensionType:       0,
		DimensionName:       "minecraft:overworld",
		HashedSeed:          12345,
		GameMode:            1,
		PreviousGameMode:    -1,
		IsDebug:             false,
		IsFlat:              false,
		HasDeathLocation:    false,
		PortalCooldown:      0,
		SeaLevel:            63,
		EnforcesSecureChat:  false,
	}

	buf := protocol.NewBuffer(512)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &LoginPlay{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.EntityID != original.EntityID {
		t.Errorf("EntityID: got %d, want %d", decoded.EntityID, original.EntityID)
	}
	if decoded.GameMode != original.GameMode {
		t.Errorf("GameMode: got %d, want %d", decoded.GameMode, original.GameMode)
	}
	if decoded.DimensionName != original.DimensionName {
		t.Errorf("DimensionName: got %q, want %q", decoded.DimensionName, original.DimensionName)
	}
	if len(decoded.DimensionNames) != len(original.DimensionNames) {
		t.Errorf("DimensionNames length: got %d, want %d", len(decoded.DimensionNames), len(original.DimensionNames))
	}
	if decoded.ViewDistance != original.ViewDistance {
		t.Errorf("ViewDistance: got %d, want %d", decoded.ViewDistance, original.ViewDistance)
	}
	if decoded.SeaLevel != original.SeaLevel {
		t.Errorf("SeaLevel: got %d, want %d", decoded.SeaLevel, original.SeaLevel)
	}
}

func TestSyncPlayerPositionRoundtrip(t *testing.T) {
	original := &SyncPlayerPosition{
		TeleportID: 7,
		X:          1.5,
		Y:          65.0,
		Z:          -3.5,
		VelocityX:  0.1,
		VelocityY:  0.0,
		VelocityZ:  -0.1,
		Yaw:        90.0,
		Pitch:      -45.0,
		Flags:      0,
	}

	buf := protocol.NewBuffer(128)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &SyncPlayerPosition{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.TeleportID != original.TeleportID {
		t.Errorf("TeleportID: got %d, want %d", decoded.TeleportID, original.TeleportID)
	}
	if decoded.X != original.X {
		t.Errorf("X: got %f, want %f", decoded.X, original.X)
	}
	if decoded.Y != original.Y {
		t.Errorf("Y: got %f, want %f", decoded.Y, original.Y)
	}
	if decoded.Yaw != original.Yaw {
		t.Errorf("Yaw: got %f, want %f", decoded.Yaw, original.Yaw)
	}
}

func TestSetDefaultSpawnPositionRoundtrip(t *testing.T) {
	original := &SetDefaultSpawnPosition{
		X:     100,
		Y:     64,
		Z:     -200,
		Angle: 45.0,
	}

	buf := protocol.NewBuffer(32)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &SetDefaultSpawnPosition{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.X != original.X {
		t.Errorf("X: got %d, want %d", decoded.X, original.X)
	}
	if decoded.Y != original.Y {
		t.Errorf("Y: got %d, want %d", decoded.Y, original.Y)
	}
	if decoded.Z != original.Z {
		t.Errorf("Z: got %d, want %d", decoded.Z, original.Z)
	}
	if decoded.Angle != original.Angle {
		t.Errorf("Angle: got %f, want %f", decoded.Angle, original.Angle)
	}
}

func TestSetCenterChunkRoundtrip(t *testing.T) {
	original := &SetCenterChunk{ChunkX: 5, ChunkZ: -3}

	buf := protocol.NewBuffer(16)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &SetCenterChunk{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.ChunkX != original.ChunkX {
		t.Errorf("ChunkX: got %d, want %d", decoded.ChunkX, original.ChunkX)
	}
	if decoded.ChunkZ != original.ChunkZ {
		t.Errorf("ChunkZ: got %d, want %d", decoded.ChunkZ, original.ChunkZ)
	}
}

func TestGameEventRoundtrip(t *testing.T) {
	original := &GameEvent{Event: 13, Value: 0.0}

	buf := protocol.NewBuffer(16)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &GameEvent{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Event != original.Event {
		t.Errorf("Event: got %d, want %d", decoded.Event, original.Event)
	}
	if decoded.Value != original.Value {
		t.Errorf("Value: got %f, want %f", decoded.Value, original.Value)
	}
}

func TestConfirmTeleportationRoundtrip(t *testing.T) {
	original := &ConfirmTeleportation{TeleportID: 42}

	buf := protocol.NewBuffer(8)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &ConfirmTeleportation{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.TeleportID != original.TeleportID {
		t.Errorf("TeleportID: got %d, want %d", decoded.TeleportID, original.TeleportID)
	}
}
