package config

// Config is the root server configuration.
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Network     NetworkConfig     `yaml:"network"`
	Tick        TickConfig        `yaml:"tick"`
	World       WorldConfig       `yaml:"world"`
	Logging     LoggingConfig     `yaml:"logging"`
	Performance PerformanceConfig `yaml:"performance"`
}

// ServerConfig contains listener and gameplay capacity settings.
type ServerConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	MaxPlayers      int    `yaml:"max_players"`
	MOTD            string `yaml:"motd"`
	OnlineMode      bool   `yaml:"online_mode"`
	DefaultGameMode string `yaml:"default_game_mode"`
}

// NetworkConfig contains network transport and protocol settings.
type NetworkConfig struct {
	ReadTimeoutSeconds      int `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds     int `yaml:"write_timeout_seconds"`
	IdleTimeoutSeconds      int `yaml:"idle_timeout_seconds"`
	CompressionThreshold    int `yaml:"compression_threshold"`
	MaxPacketSize           int `yaml:"max_packet_size"`
	MaxInboundQueueCapacity int `yaml:"max_inbound_queue_capacity"`
}

// TickConfig contains tick loop timing settings.
type TickConfig struct {
	TargetTPS       int `yaml:"target_tps"`
	MaxCatchUpTicks int `yaml:"max_catch_up_ticks"`
}

// WorldConfig contains world bootstrap and simulation settings.
type WorldConfig struct {
	DefaultWorldName string `yaml:"default_world_name"`

	ViewDistance       int `yaml:"view_distance"`
	SimulationDistance int `yaml:"simulation_distance"`

	SchedulerQueueCapacity int `yaml:"scheduler_queue_capacity"`
	SchedulerDrainPerTick  int `yaml:"scheduler_drain_per_tick"`

	ChunkLoadWorkers      int `yaml:"chunk_load_workers"`
	ChunkWorkerQueueSize  int `yaml:"chunk_worker_queue_size"`
	ChunkRequestQueueSize int `yaml:"chunk_request_queue_size"`
	ChunkResultQueueSize  int `yaml:"chunk_result_queue_size"`
	ChunkRequestPumpBatch int `yaml:"chunk_request_pump_batch"`
	ChunkCompletionBatch  int `yaml:"chunk_completion_batch"`
	ChunkUnloadBatch      int `yaml:"chunk_unload_batch"`
	ChunkUnloadScanBatch  int `yaml:"chunk_unload_scan_batch"`
	ChunkUnloadTTLTicks   int `yaml:"chunk_unload_ttl_ticks"`
	ChunkStorageInitCap   int `yaml:"chunk_storage_initial_capacity"`
	ChunkUnloadQueueSize  int `yaml:"chunk_unload_queue_size"`

	ChunkStreamChunksPerTick int `yaml:"chunk_stream_chunks_per_tick"`
}

// LoggingConfig contains runtime logging options.
type LoggingConfig struct {
	Level       string `yaml:"level"`
	Format      string `yaml:"format"`
	FilePath    string `yaml:"file_path"`
	EnableColor bool   `yaml:"enable_color"`
}

// PerformanceConfig contains pool and buffer sizing options.
type PerformanceConfig struct {
	IOWorkerPoolSize     int `yaml:"io_worker_pool_size"`
	PacketWorkerPoolSize int `yaml:"packet_worker_pool_size"`
	ReadBufferSize       int `yaml:"read_buffer_size"`
	WriteBufferSize      int `yaml:"write_buffer_size"`
	OutboundQueueSize    int `yaml:"outbound_queue_size"`
}
