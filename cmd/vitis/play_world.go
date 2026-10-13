package main

import (
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/world"
)

// newWorldAccessor connects gameplay operations to one dimension's runtime.
func newWorldAccessor(w *world.World) *session.DefaultWorldAccessor {
	return &session.DefaultWorldAccessor{
		DimensionName: w.Dimension(), Chunks: w.ChunkManager(), TickScheduler: w.TickScheduler(),
		CurrentTick: w.CurrentTick, NextEntityID: w.EntityManager().AllocateID,
		Items: w.ItemEntities(), RegisterItem: w.SpawnItemEntity, BlockEntityStore: w.BlockEntities(),
		SubmitTask: w.Submit, ResetPlayerChunks: w.ResendPlayerChunks, PrimeTNT: w.IgniteTNT,
		ReadTimeOfDay: w.TimeOfDay, WriteTimeOfDay: w.SetTimeOfDay, ReadSleepThreat: w.SleepThreat,
		ReadThunder: w.Weather().IsThundering, ClearWeatherAfterSleep: w.ClearSleepWeather,
		CreateBedExplosion: w.ExplodeBed,
		UpdateNeighbors:    w.NotifyNeighbors,
		AttackMobEntity:    w.AttackMob,
	}
}

// playConfigForWorld resolves level-dependent bootstrap and respawn settings.
func playConfigForWorld(base session.PlayBootstrapConfig, w *world.World) session.PlayBootstrapConfig {
	base.DimensionName = w.Dimension()
	base.ReadWorldSpawn, base.GameRule = w.WorldSpawn, w.GameRule
	base.KeepInventory = func() bool { return w.GameRule("keepInventory") == "true" }
	base.WorldAge, base.TimeOfDay, base.TickDayTime = w.WorldAge(), w.TimeOfDay(), w.DaylightCycle()
	base.WeatherPackets = make([]protocol.Packet, 0, 3)
	for _, packet := range w.Weather().JoinPackets() {
		base.WeatherPackets = append(base.WeatherPackets, &play.GameEvent{Event: packet.Event, Value: packet.Value})
	}
	return base
}
