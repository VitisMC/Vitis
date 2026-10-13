package config

import "runtime"

const (
	DefaultConfigPath = "configs/vitis.yaml"
)

// Default returns a fully initialized configuration with production-safe defaults.
func Default() *Config {
	parallelism := runtime.GOMAXPROCS(0)
	if parallelism < 2 {
		parallelism = 2
	}
	chunkWorkers := defaultChunkWorkers(parallelism)

	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            25565,
			MaxPlayers:      200,
			MOTD:            "Vitis Server",
			OnlineMode:      true,
			DefaultGameMode: "survival",
		},
		Network: NetworkConfig{
			MaxConnections:          512,
			AdmissionRatePerSecond:  10,
			AdmissionBurst:          20,
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
			Seed:             42,
			Difficulty:       "normal",
			BackupWorld:      true,

			ViewDistance:       10,
			SimulationDistance: 6,

			SchedulerQueueCapacity: 8192,
			SchedulerDrainPerTick:  1024,

			ChunkLoadWorkers:      chunkWorkers,
			ChunkWorkerQueueSize:  chunkWorkers,
			ChunkRequestQueueSize: 8192,
			ChunkResultQueueSize:  8192,
			ChunkRequestPumpBatch: 256,
			ChunkCompletionBatch:  256,
			ChunkUnloadBatch:      64,
			ChunkUnloadScanBatch:  512,
			ChunkUnloadTTLTicks:   600,
			ChunkStorageInitCap:   2048,
			ChunkUnloadQueueSize:  8192,

			ChunkStreamChunksPerTick: 32,
			AutosaveIntervalTicks:    1200,
			AutosaveBatchPerTick:     2,
			AutosaveQueueCapacity:    64,
		},
		Logging: LoggingConfig{
			Level:       "info",
			Format:      "console",
			FilePath:    "",
			EnableColor: true,
		},
		Performance: PerformanceConfig{
			IOWorkerPoolSize:  parallelism,
			ReadBufferSize:    64 * 1024,
			WriteBufferSize:   64 * 1024,
			OutboundQueueSize: 2048,
		},
		RCON: RCONConfig{
			Address: "127.0.0.1",
			Port:    25575,
		},
		Query: QueryConfig{Port: 25565},
		Security: SecurityConfig{
			ChatMessagesPer10Seconds: 5,
			CommandsPer10Seconds:     10,
		},
	}
}

func defaultChunkWorkers(parallelism int) int {
	workers := parallelism * 2 / 3
	if workers < 2 {
		return 2
	}
	return workers
}
