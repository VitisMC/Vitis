package config

import "runtime"

const (
	DefaultConfigPath = "configs/vitis.yaml"
)

// Default returns a fully initialized configuration with production-safe defaults.
func Default() *Config {
	workers := runtime.GOMAXPROCS(0)
	if workers < 2 {
		workers = 2
	}

	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            25565,
			MaxPlayers:      200,
			MOTD:            "Vitis Server",
			OnlineMode:      false,
			DefaultGameMode: "survival",
		},
		Network: NetworkConfig{
			ReadTimeoutSeconds:      30,
			WriteTimeoutSeconds:     30,
			IdleTimeoutSeconds:      120,
			CompressionThreshold:    256,
			MaxPacketSize:           2 << 20,
			MaxInboundQueueCapacity: 4096,
		},
		Tick: TickConfig{
			TargetTPS:       20,
			MaxCatchUpTicks: 5,
		},
		World: WorldConfig{
			DefaultWorldName: "world",

			ViewDistance:       10,
			SimulationDistance: 10,

			SchedulerQueueCapacity: 8192,
			SchedulerDrainPerTick:  1024,

			ChunkLoadWorkers:      workers,
			ChunkWorkerQueueSize:  workers * 64,
			ChunkRequestQueueSize: 8192,
			ChunkResultQueueSize:  8192,
			ChunkRequestPumpBatch: 256,
			ChunkCompletionBatch:  256,
			ChunkUnloadBatch:      64,
			ChunkUnloadScanBatch:  512,
			ChunkUnloadTTLTicks:   600,
			ChunkStorageInitCap:   2048,
			ChunkUnloadQueueSize:  8192,

			ChunkStreamChunksPerTick: 4,
		},
		Logging: LoggingConfig{
			Level:       "info",
			Format:      "console",
			FilePath:    "",
			EnableColor: true,
		},
		Performance: PerformanceConfig{
			IOWorkerPoolSize:     workers,
			PacketWorkerPoolSize: workers * 2,
			ReadBufferSize:       64 * 1024,
			WriteBufferSize:      64 * 1024,
			OutboundQueueSize:    2048,
		},
	}
}
