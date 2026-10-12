package main

import (
	"testing"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/world/level"
)

// TestMeleeEffectSnapshotSurvivesPlayerSave preserves the duration and flags used by species combat.
func TestMeleeEffectSnapshotSurvivesPlayerSave(t *testing.T) {
	uuid := protocol.UUID{0, 1}
	uuidString := protocol.UUIDToString(uuid)
	player := entity.NewPlayer(1, uuid, "defender", entity.Vec3{Y: 64}, entity.Vec2{}, nil, 2)
	player.Effects().Add(effect.Instance{ID: effectdata.EffectWither, Duration: 200, Flags: effect.FlagParticles | effect.FlagIcon})
	data := snapshotPlayerData(nil, uuidString, &session.OnlinePlayer{UUID: uuid}, player)
	player.Effects().Tick()
	if len(data.Effects) != 1 || data.Effects[0].Duration != 200 {
		t.Fatal("player effect snapshot aliases live duration")
	}
	directory := t.TempDir()
	if err := data.Save(directory); err != nil {
		t.Fatal(err)
	}
	restored, err := level.LoadPlayerData(directory, uuidString)
	if err != nil {
		t.Fatal(err)
	}
	if restored == nil || len(restored.Effects) != 1 || restored.Effects[0] != data.Effects[0] {
		t.Fatal("saved melee effect lost its type, duration or client flags")
	}
}
