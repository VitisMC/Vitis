package world

import (
	"github.com/vitismc/vitis/internal/entity"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/weather"
	"maps"
	"math"

	gamerules "github.com/vitismc/vitis/internal/data/generated/game_rules"
	"github.com/vitismc/vitis/internal/world/level"
)

// initMetadata owns an independent copy of persistent world settings.
func (w *World) initMetadata(source *level.Level) {
	if source == nil {
		source = level.New(w.name, 0, 64, 0)
	}
	cloned := *source
	cloned.GameRules = gamerules.Defaults()
	maps.Copy(cloned.GameRules, source.GameRules)
	w.metadata = &cloned
	w.SetDaylightCycle(w.GameRule("doDaylightCycle") == "true")
	w.SetWeatherCycle(w.GameRule("doWeatherCycle") == "true")
	w.SetFireTick(w.GameRule("doFireTick") == "true")
}

// GameRule returns a rule from the authoritative persistent world state.
func (w *World) GameRule(name string) string {
	w.metadataMu.RLock()
	defer w.metadataMu.RUnlock()
	return w.metadata.GameRules[name]
}

// SetGameRule updates a validated rule and its simulation settings on the tick.
func (w *World) SetGameRule(name, value string) error {
	if err := gamerules.Validate(name, value); err != nil {
		return err
	}
	w.metadataMu.Lock()
	w.metadata.GameRules[name] = value
	w.metadataMu.Unlock()
	switch name {
	case "doDaylightCycle":
		w.SetDaylightCycle(value == "true")
	case "doWeatherCycle":
		w.SetWeatherCycle(value == "true")
	case "doFireTick":
		w.SetFireTick(value == "true")
	}
	return nil
}

// WorldSpawn returns the current persistent world spawn and angle.
func (w *World) WorldSpawn() (int32, int32, int32, float32) {
	w.metadataMu.RLock()
	defer w.metadataMu.RUnlock()
	return w.metadata.SpawnX, w.metadata.SpawnY, w.metadata.SpawnZ, w.metadata.SpawnAngle
}

// SetWorldSpawn updates the world-owned spawn on the authoritative tick.
func (w *World) SetWorldSpawn(x, y, z int32, angle float32) {
	w.metadataMu.Lock()
	defer w.metadataMu.Unlock()
	w.metadata.SpawnX, w.metadata.SpawnY, w.metadata.SpawnZ, w.metadata.SpawnAngle = x, y, z, angle
}

// MetadataSnapshot returns independently mutable world metadata for persistence.
func (w *World) MetadataSnapshot() *level.Level {
	w.metadataMu.RLock()
	defer w.metadataMu.RUnlock()
	if w.metadata == nil {
		return nil
	}
	snapshot := *w.metadata
	snapshot.GameRules = maps.Clone(w.metadata.GameRules)
	snapshot.Time, snapshot.DayTime, snapshot.Difficulty = w.WorldAge(), w.TimeOfDay(), int8(w.Difficulty())
	return &snapshot
}

// SetPlayerTickHandler installs tick-owned player lifecycle and snapshot work.
func (w *World) SetPlayerTickHandler(handler func(uint64)) { w.playerTick = handler }

// SleepThreat checks living monsters near a bed on the authoritative tick.
func (w *World) SleepThreat(x, y, z int) bool {
	if w.mobs == nil {
		return false
	}
	for _, mob := range w.mobs.Mobs() {
		if mob.IsDead() || mob.Category() != entity.MobCategoryHostile {
			continue
		}
		p := mob.Position()
		if math.Abs(p.X-float64(x)) <= 8 && math.Abs(p.Y-float64(y)) <= 5 && math.Abs(p.Z-float64(z)) <= 8 {
			return true
		}
	}
	return false
}

// ClearSleepWeather ends rain and thunder only when natural weather is enabled.
func (w *World) ClearSleepWeather() {
	if w.GameRule("doWeatherCycle") == "true" {
		packets := w.weather.SetWeather(weather.Clear, 0)
		for _, packet := range packets {
			for _, player := range w.tracker.Players() {
				if session := player.Session(); session != nil {
					_ = session.Send(&playpacket.GameEvent{Event: packet.Event, Value: packet.Value})
				}
			}
		}
	}
}
