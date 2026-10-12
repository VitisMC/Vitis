package config

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/network/rcon"
)

const (
	minPort                 = 1
	maxPort                 = 65535
	minDefaultWorldNameLen  = 1
	minViewDistance         = 2
	maxViewDistance         = 96
	minSimulationDistance   = 2
	maxSimulationDistance   = 96
	minCompressionThreshold = -1
	minMaxPacketSize        = 512
	minQueueCapacity        = 1
	minWorkers              = 1
	minBufferSize           = 1024
	minTPS                  = 1
	maxTPS                  = 1000
	minMaxCatchUpTicks      = 0
	maxMaxCatchUpTicks      = 100
	minWorldTickBatch       = 1
	minChunkUnloadTTLTicks  = 1
)

// Validate checks whether cfg satisfies all server configuration constraints.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if cfg.Server.Host == "" {
		return fmt.Errorf("server.host must not be empty")
	}
	if cfg.Server.Port < minPort || cfg.Server.Port > maxPort {
		return fmt.Errorf("server.port must be between %d and %d, got %d", minPort, maxPort, cfg.Server.Port)
	}
	if cfg.Server.MaxPlayers <= 0 {
		return fmt.Errorf("server.max_players must be > 0, got %d", cfg.Server.MaxPlayers)
	}
	if strings.TrimSpace(cfg.Server.MOTD) == "" {
		return fmt.Errorf("server.motd must not be empty")
	}
	switch strings.ToLower(cfg.Server.DefaultGameMode) {
	case "survival", "creative", "adventure", "spectator":
	default:
		return fmt.Errorf("server.default_game_mode must be one of survival|creative|adventure|spectator, got %q", cfg.Server.DefaultGameMode)
	}

	if cfg.Network.ReadTimeoutSeconds <= 0 {
		return fmt.Errorf("network.read_timeout_seconds must be > 0, got %d", cfg.Network.ReadTimeoutSeconds)
	}
	if cfg.Network.MaxConnections <= 0 {
		return fmt.Errorf("network.max_connections must be > 0, got %d", cfg.Network.MaxConnections)
	}
	if cfg.Network.AdmissionRatePerSecond <= 0 {
		return fmt.Errorf("network.admission_rate_per_second must be > 0, got %d", cfg.Network.AdmissionRatePerSecond)
	}
	if cfg.Network.AdmissionBurst <= 0 {
		return fmt.Errorf("network.admission_burst must be > 0, got %d", cfg.Network.AdmissionBurst)
	}
	if cfg.Network.WriteTimeoutSeconds <= 0 {
		return fmt.Errorf("network.write_timeout_seconds must be > 0, got %d", cfg.Network.WriteTimeoutSeconds)
	}
	if cfg.Network.IdleTimeoutSeconds <= 0 {
		return fmt.Errorf("network.idle_timeout_seconds must be > 0, got %d", cfg.Network.IdleTimeoutSeconds)
	}
	if cfg.Network.CompressionThreshold < minCompressionThreshold {
		return fmt.Errorf("network.compression_threshold must be >= %d, got %d", minCompressionThreshold, cfg.Network.CompressionThreshold)
	}
	if cfg.Network.MaxPacketSize < minMaxPacketSize {
		return fmt.Errorf("network.max_packet_size must be >= %d, got %d", minMaxPacketSize, cfg.Network.MaxPacketSize)
	}
	if cfg.Network.MaxInboundQueueCapacity < minQueueCapacity {
		return fmt.Errorf("network.max_inbound_queue_capacity must be >= %d, got %d", minQueueCapacity, cfg.Network.MaxInboundQueueCapacity)
	}

	if cfg.Tick.TargetTPS < minTPS || cfg.Tick.TargetTPS > maxTPS {
		return fmt.Errorf("tick.target_tps must be between %d and %d, got %d", minTPS, maxTPS, cfg.Tick.TargetTPS)
	}
	if cfg.Tick.MaxCatchUpTicks < minMaxCatchUpTicks || cfg.Tick.MaxCatchUpTicks > maxMaxCatchUpTicks {
		return fmt.Errorf("tick.max_catch_up_ticks must be between %d and %d, got %d", minMaxCatchUpTicks, maxMaxCatchUpTicks, cfg.Tick.MaxCatchUpTicks)
	}

	if cfg.World.ViewDistance < minViewDistance || cfg.World.ViewDistance > maxViewDistance {
		return fmt.Errorf("world.view_distance must be between %d and %d, got %d", minViewDistance, maxViewDistance, cfg.World.ViewDistance)
	}
	if cfg.World.SimulationDistance < minSimulationDistance || cfg.World.SimulationDistance > maxSimulationDistance {
		return fmt.Errorf("world.simulation_distance must be between %d and %d, got %d", minSimulationDistance, maxSimulationDistance, cfg.World.SimulationDistance)
	}
	if cfg.World.SimulationDistance > cfg.World.ViewDistance {
		return fmt.Errorf("world.simulation_distance must be <= world.view_distance, got simulation=%d view=%d", cfg.World.SimulationDistance, cfg.World.ViewDistance)
	}
	if cfg.World.AutosaveIntervalTicks < 1 {
		return fmt.Errorf("world.autosave_interval_ticks must be >= 1")
	}
	if cfg.World.AutosaveBatchPerTick < 1 {
		return fmt.Errorf("world.autosave_batch_per_tick must be >= 1")
	}
	if cfg.World.AutosaveQueueCapacity < 1 {
		return fmt.Errorf("world.autosave_queue_capacity must be >= 1")
	}
	if len(strings.TrimSpace(cfg.World.DefaultWorldName)) < minDefaultWorldNameLen {
		return fmt.Errorf("world.default_world_name must not be empty")
	}
	switch strings.ToLower(cfg.World.Difficulty) {
	case "peaceful", "easy", "normal", "hard":
	default:
		return fmt.Errorf("world.difficulty must be one of peaceful|easy|normal|hard, got %q", cfg.World.Difficulty)
	}
	if cfg.World.SchedulerQueueCapacity < minQueueCapacity {
		return fmt.Errorf("world.scheduler_queue_capacity must be >= %d, got %d", minQueueCapacity, cfg.World.SchedulerQueueCapacity)
	}
	if cfg.World.SchedulerDrainPerTick < minWorldTickBatch {
		return fmt.Errorf("world.scheduler_drain_per_tick must be >= %d, got %d", minWorldTickBatch, cfg.World.SchedulerDrainPerTick)
	}
	if cfg.World.ChunkLoadWorkers < minWorkers {
		return fmt.Errorf("world.chunk_load_workers must be >= %d, got %d", minWorkers, cfg.World.ChunkLoadWorkers)
	}
	if cfg.World.ChunkWorkerQueueSize < minQueueCapacity {
		return fmt.Errorf("world.chunk_worker_queue_size must be >= %d, got %d", minQueueCapacity, cfg.World.ChunkWorkerQueueSize)
	}
	if cfg.World.ChunkRequestQueueSize < minQueueCapacity {
		return fmt.Errorf("world.chunk_request_queue_size must be >= %d, got %d", minQueueCapacity, cfg.World.ChunkRequestQueueSize)
	}
	if cfg.World.ChunkResultQueueSize < minQueueCapacity {
		return fmt.Errorf("world.chunk_result_queue_size must be >= %d, got %d", minQueueCapacity, cfg.World.ChunkResultQueueSize)
	}
	if cfg.World.ChunkRequestPumpBatch < minWorldTickBatch {
		return fmt.Errorf("world.chunk_request_pump_batch must be >= %d, got %d", minWorldTickBatch, cfg.World.ChunkRequestPumpBatch)
	}
	if cfg.World.ChunkCompletionBatch < minWorldTickBatch {
		return fmt.Errorf("world.chunk_completion_batch must be >= %d, got %d", minWorldTickBatch, cfg.World.ChunkCompletionBatch)
	}
	if cfg.World.ChunkUnloadBatch < minWorldTickBatch {
		return fmt.Errorf("world.chunk_unload_batch must be >= %d, got %d", minWorldTickBatch, cfg.World.ChunkUnloadBatch)
	}
	if cfg.World.ChunkUnloadScanBatch < minWorldTickBatch {
		return fmt.Errorf("world.chunk_unload_scan_batch must be >= %d, got %d", minWorldTickBatch, cfg.World.ChunkUnloadScanBatch)
	}
	if cfg.World.ChunkUnloadTTLTicks < minChunkUnloadTTLTicks {
		return fmt.Errorf("world.chunk_unload_ttl_ticks must be >= %d, got %d", minChunkUnloadTTLTicks, cfg.World.ChunkUnloadTTLTicks)
	}
	if cfg.World.ChunkStorageInitCap < minQueueCapacity {
		return fmt.Errorf("world.chunk_storage_initial_capacity must be >= %d, got %d", minQueueCapacity, cfg.World.ChunkStorageInitCap)
	}
	if cfg.World.ChunkUnloadQueueSize < minQueueCapacity {
		return fmt.Errorf("world.chunk_unload_queue_size must be >= %d, got %d", minQueueCapacity, cfg.World.ChunkUnloadQueueSize)
	}
	if cfg.World.ChunkStreamChunksPerTick < minWorldTickBatch {
		return fmt.Errorf("world.chunk_stream_chunks_per_tick must be >= %d, got %d", minWorldTickBatch, cfg.World.ChunkStreamChunksPerTick)
	}

	level := strings.ToLower(cfg.Logging.Level)
	switch level {
	case "trace", "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level must be one of trace|debug|info|warn|error, got %q", cfg.Logging.Level)
	}

	format := strings.ToLower(cfg.Logging.Format)
	switch format {
	case "console", "json":
	default:
		return fmt.Errorf("logging.format must be one of console|json, got %q", cfg.Logging.Format)
	}

	if cfg.Performance.IOWorkerPoolSize < minWorkers {
		return fmt.Errorf("performance.io_worker_pool_size must be >= %d, got %d", minWorkers, cfg.Performance.IOWorkerPoolSize)
	}
	if cfg.Performance.ReadBufferSize < minBufferSize {
		return fmt.Errorf("performance.read_buffer_size must be >= %d, got %d", minBufferSize, cfg.Performance.ReadBufferSize)
	}
	if cfg.Performance.WriteBufferSize < minBufferSize {
		return fmt.Errorf("performance.write_buffer_size must be >= %d, got %d", minBufferSize, cfg.Performance.WriteBufferSize)
	}
	if cfg.Performance.OutboundQueueSize < minQueueCapacity {
		return fmt.Errorf("performance.outbound_queue_size must be >= %d, got %d", minQueueCapacity, cfg.Performance.OutboundQueueSize)
	}

	if cfg.RCON.Enabled {
		if strings.TrimSpace(cfg.RCON.Address) == "" {
			return fmt.Errorf("rcon.address must not be empty when RCON is enabled")
		}
		if cfg.RCON.Port < minPort || cfg.RCON.Port > maxPort {
			return fmt.Errorf("rcon.port must be between %d and %d, got %d", minPort, maxPort, cfg.RCON.Port)
		}
		if !rcon.ValidPassword(cfg.RCON.Password) {
			return fmt.Errorf("rcon.password must be a strong secret of at least 16 characters")
		}
	}
	if cfg.Query.Enabled && (cfg.Query.Port < minPort || cfg.Query.Port > maxPort) {
		return fmt.Errorf("query.port must be between %d and %d, got %d", minPort, maxPort, cfg.Query.Port)
	}
	if cfg.Security.ChatMessagesPer10Seconds <= 0 {
		return fmt.Errorf("security.chat_messages_per_10_seconds must be > 0")
	}
	if cfg.Security.CommandsPer10Seconds <= 0 {
		return fmt.Errorf("security.commands_per_10_seconds must be > 0")
	}

	return nil
}
