package main

import (
	"context"
	"time"

	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/metrics"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/tick"
)

// updateLiveTab publishes process and chunk rates once per second.
func updateLiveTab(ctx context.Context, players *session.PlayerManager, loop *tick.Loop, tab config.TabConfig, activity *metrics.ChunkActivity) {
	if players == nil || loop == nil {
		return
	}
	sampler := metrics.NewLiveSampler(activity)
	sampler.Sample()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := sampler.Sample()
			stats := config.TabStats{
				Online: players.Count(), Max: players.MaxPlayers(),
				TPS:        loop.Timing().SmoothedTPS,
				CPUPercent: snapshot.CPUPercent, RAMBytes: snapshot.RAMBytes,
				GeneratedPerSec: snapshot.GeneratedPerSec, SentPerSec: snapshot.SentPerSec,
				ChunkAvgMS: snapshot.AvgChunkMS, ChunkLastMS: snapshot.LastChunkMS,
				FullPathPerSec: snapshot.FullPathPerSec, FullPathAvgMS: snapshot.AvgFullPathMS, FullPathLastMS: snapshot.LastFullPathMS,
			}
			header, footer := tab.RenderLive(stats)
			packet := &playpacket.PlayerlistHeader{Header: header, Footer: footer}
			players.Broadcast(packet)
		}
	}
}
