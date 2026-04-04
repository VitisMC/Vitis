package integration

import (
	"context"
	"testing"
	"time"

	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/tick"
	"github.com/vitismc/vitis/internal/world"
)

func TestWorldTickLoopIntegrationLoadsChunk(t *testing.T) {
	cfg := config.Default()
	cfg.Tick.TargetTPS = 200
	cfg.World.DefaultWorldName = "integration_world"

	manager := world.NewManager()
	instance, err := manager.CreateWithConfig(world.Config{
		Name: cfg.World.DefaultWorldName,

		SchedulerQueueCapacity: cfg.World.SchedulerQueueCapacity,
		SchedulerDrainPerTick:  cfg.World.SchedulerDrainPerTick,

		ChunkLoadWorkers:        cfg.World.ChunkLoadWorkers,
		ChunkWorkerQueueSize:    cfg.World.ChunkWorkerQueueSize,
		ChunkRequestQueueSize:   cfg.World.ChunkRequestQueueSize,
		ChunkResultQueueSize:    cfg.World.ChunkResultQueueSize,
		ChunkRequestPumpBatch:   cfg.World.ChunkRequestPumpBatch,
		ChunkCompletionsPerTick: cfg.World.ChunkCompletionBatch,
		ChunkUnloadBatchPerTick: cfg.World.ChunkUnloadBatch,
		ChunkUnloadScanBatch:    cfg.World.ChunkUnloadScanBatch,
		ChunkUnloadTTLTicks:     cfg.World.ChunkUnloadTTLTicks,
		ChunkStorageInitCap:     cfg.World.ChunkStorageInitCap,
		ChunkUnloadQueueSize:    cfg.World.ChunkUnloadQueueSize,
	})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}

	loop, err := tick.NewLoop(tick.LoopConfig{
		TargetTPS:          cfg.Tick.TargetTPS,
		MaxCatchUpTicks:    cfg.Tick.MaxCatchUpTicks,
		OverloadMode:       tick.OverloadCatchUp,
		CancelPendingTasks: true,
	}, manager)
	if err != nil {
		t.Fatalf("create tick loop failed: %v", err)
	}

	if err := loop.Start(); err != nil {
		t.Fatalf("start tick loop failed: %v", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = loop.Stop(ctx)
		_ = manager.Close(ctx)
	}()

	instance.LoadChunk(8, 12)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		loaded, ok := instance.GetChunk(8, 12)
		if ok && loaded != nil {
			if loaded.X() != 8 || loaded.Z() != 12 {
				t.Fatalf("unexpected loaded coordinates x=%d z=%d", loaded.X(), loaded.Z())
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for loaded chunk")
}

func TestWorldIntegrationRespectsUnloadTTL(t *testing.T) {
	cfg := config.Default()
	cfg.World.DefaultWorldName = "integration_unload_world"
	cfg.World.ChunkUnloadTTLTicks = 2
	cfg.World.ChunkUnloadBatch = 16
	cfg.World.ChunkUnloadScanBatch = 64

	instance, err := world.New(world.Config{
		Name: cfg.World.DefaultWorldName,

		SchedulerQueueCapacity: cfg.World.SchedulerQueueCapacity,
		SchedulerDrainPerTick:  cfg.World.SchedulerDrainPerTick,

		ChunkLoadWorkers:        cfg.World.ChunkLoadWorkers,
		ChunkWorkerQueueSize:    cfg.World.ChunkWorkerQueueSize,
		ChunkRequestQueueSize:   cfg.World.ChunkRequestQueueSize,
		ChunkResultQueueSize:    cfg.World.ChunkResultQueueSize,
		ChunkRequestPumpBatch:   cfg.World.ChunkRequestPumpBatch,
		ChunkCompletionsPerTick: cfg.World.ChunkCompletionBatch,
		ChunkUnloadBatchPerTick: cfg.World.ChunkUnloadBatch,
		ChunkUnloadScanBatch:    cfg.World.ChunkUnloadScanBatch,
		ChunkUnloadTTLTicks:     cfg.World.ChunkUnloadTTLTicks,
		ChunkStorageInitCap:     cfg.World.ChunkStorageInitCap,
		ChunkUnloadQueueSize:    cfg.World.ChunkUnloadQueueSize,
	})
	if err != nil {
		t.Fatalf("create world failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = instance.Close(ctx)
	}()

	instance.LoadChunk(1, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		if instance.ChunkManager().Len() == 0 {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		if instance.ChunkManager().PendingLoadCompletions() == 0 && instance.ChunkManager().PendingLoadRequests() == 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		instance.Tick()
		if instance.ChunkManager().Len() == 0 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("expected chunk to be unloaded by ttl policy")
}
