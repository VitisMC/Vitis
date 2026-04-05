package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/vitismc/vitis/internal/logger"

	"sync/atomic"

	"github.com/vitismc/vitis/internal/command"
	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/operator"
	"github.com/vitismc/vitis/internal/protocol"
	protocrypto "github.com/vitismc/vitis/internal/protocol/crypto"
	loginpacket "github.com/vitismc/vitis/internal/protocol/packets/login"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/protocol/states"
	"github.com/vitismc/vitis/internal/registry"
	"github.com/vitismc/vitis/internal/session"
	"github.com/vitismc/vitis/internal/tick"
	"github.com/vitismc/vitis/internal/world"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/persistence"
)

const (
	statusVersionName   = "1.21.4"
	statusProtocol      = int32(769)
	shutdownGracePeriod = 5 * time.Second
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", config.DefaultConfigPath, "path to configuration file")
	flag.Parse()

	cfgManager := config.NewManager(*configPath)
	cfg, err := cfgManager.Load()
	if err != nil {
		logger.Error("load config failed", "error", err)
		return 1
	}

	logger.Init(cfg.Logging.Level, cfg.Logging.Format, nil)

	worldManager, tickLoop, err := bootstrapWorldRuntime(cfg)
	if err != nil {
		logger.Error("bootstrap world runtime failed", "error", err)
		return 1
	}

	if err := tickLoop.Start(); err != nil {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		logger.Error("start tick loop failed", "error", err)
		return 1
	}

	regMgr, err := registry.NewManager()
	if err != nil {
		logger.Error("create registry manager failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}
	logger.Info("registry manager initialized", "registries", regMgr.RegistryCount(), "config_registries", regMgr.ConfigRegistryCount())

	protocolRegistry := protocol.NewRegistry()
	if err := states.RegisterCore(protocolRegistry, protocol.AnyVersion); err != nil {
		logger.Error("register protocol states failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	router := session.NewPacketRouter()
	packetPool := network.NewWorkerPool(network.WorkerPoolConfig{
		Size:          maxInt(cfg.Performance.PacketWorkerPoolSize, 1),
		QueueCapacity: maxInt(cfg.Network.MaxInboundQueueCapacity, 1024),
	})

	sessionManager, err := session.NewManager(session.ManagerConfig{
		Registry:     protocolRegistry,
		Router:       router,
		WorkerPool:   packetPool,
		InitialState: protocol.StateHandshake,
	})
	if err != nil {
		logger.Error("create session manager failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	favicon := os.Getenv("VITIS_FAVICON")
	statusProvider := session.StatusInfoProviderFunc(func(currentSession session.Session) session.StatusInfo {
		activeConfig := cfgManager.Get()
		protocolVersion := statusProtocol
		if currentSession != nil && currentSession.ProtocolVersion() > 0 {
			protocolVersion = currentSession.ProtocolVersion()
		}

		return session.StatusInfo{
			VersionName:     statusVersionName,
			ProtocolVersion: protocolVersion,
			MaxPlayers:      activeConfig.Server.MaxPlayers,
			OnlinePlayers:   saturatingInt64ToInt(sessionManager.CountOnlinePlayers()),
			Description:     activeConfig.Server.MOTD,
			Favicon:         favicon,
		}
	})

	if err := session.RegisterStatusHandlers(router, statusProvider); err != nil {
		logger.Error("register status handlers failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	var entityIDCounter atomic.Int32
	playerManager := session.NewPlayerManager()

	tabCfg := config.LoadTabConfig("configs/tab.yaml")

	playBootstrapCfg := session.DefaultPlayBootstrapConfig()
	playBootstrapCfg.ViewDistance = int32(cfg.World.ViewDistance)
	if playBootstrapCfg.ViewDistance <= 0 {
		playBootstrapCfg.ViewDistance = 10
	}
	playBootstrapCfg.GameMode = session.GameModeFromString(cfg.Server.DefaultGameMode)
	playBootstrapCfg.RegistryManager = regMgr
	playBootstrapCfg.TabHeader = tabCfg.RenderHeader(0, cfg.Server.MaxPlayers, 20.0)
	playBootstrapCfg.TabFooter = tabCfg.RenderFooter(0, cfg.Server.MaxPlayers, 20.0)

	opList := operator.NewList("ops.json")
	if err := opList.Load(); err != nil {
		logger.Warn("failed to load ops.json", "error", err)
	} else {
		logger.Info("operator list loaded", "count", opList.Count())
	}
	playBootstrapCfg.OperatorList = opList

	shutdownSignal := make(chan os.Signal, 1)
	cmdRegistry := command.NewRegistry()
	serverControl := session.NewServerControl(playerManager, func() {
		shutdownSignal <- syscall.SIGTERM
	}, opList)
	playerLookup := session.NewPlayerLookup(playerManager, opList)
	command.RegisterBuiltinCommands(cmdRegistry, playerLookup, serverControl)
	command.RegisterMultiplayerCommands(cmdRegistry, nil, nil, nil)

	playBootstrapCfg.CommandRegistry = cmdRegistry
	logger.Info("command system initialized", "commands", cmdRegistry.Count())

	loginCfg := session.LoginConfig{
		CompressionThreshold: cfg.Network.CompressionThreshold,
		OnlineMode:           cfg.Server.OnlineMode,
		OnLoginSuccess: func(s session.Session, name string, uuid protocol.UUID) {
			logger.Info("player login", "name", name, "uuid", protocol.UUIDToString(uuid), "session", s.ID())
		},
	}
	if cfg.Server.OnlineMode {
		privKey, err := protocrypto.GenerateKeyPair()
		if err != nil {
			logger.Error("generate rsa keypair failed", "error", err)
			shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
			_ = packetPool.Stop(shutdownContext)
			cancel()
			return 1
		}
		pubKeyDER, err := protocrypto.EncodePublicKey(&privKey.PublicKey)
		if err != nil {
			logger.Error("encode public key failed", "error", err)
			shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
			_ = packetPool.Stop(shutdownContext)
			cancel()
			return 1
		}
		loginCfg.PrivateKey = privKey
		loginCfg.PublicKeyDER = pubKeyDER
		logger.Info("online mode: RSA keypair generated")
	} else {
		logger.Info("offline mode: encryption disabled")
	}
	if err := session.RegisterLoginHandlers(router, loginCfg); err != nil {
		logger.Error("register login handlers failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	defaultWorld, worldFound := worldManager.Get(cfg.World.DefaultWorldName)
	if !worldFound {
		dw, _ := worldManager.Get("world")
		defaultWorld = dw
	}

	if defaultWorld != nil {
		playBootstrapCfg.SpawnChunks = &session.WorldChunkProvider{
			Chunks:   defaultWorld.ChunkManager(),
			Fallback: nil,
		}
		if sc, ok := serverControl.(*session.ServerControlAdapter); ok {
			sc.World = defaultWorld
			sc.WeatherWorld = defaultWorld
			sc.SeedValue = 42
		}
	}

	if err := session.RegisterConfigurationHandlers(router, regMgr, func(s session.Session) {
		ds, ok := s.(*session.DefaultSession)
		if !ok {
			return
		}
		username := ""
		uuid := protocol.UUID{}
		if raw, found := ds.SessionData().Load("login_name"); found {
			username, _ = raw.(string)
		}
		if raw, found := ds.SessionData().Load("login_uuid"); found {
			uuid, _ = raw.(protocol.UUID)
		}

		entityID := entityIDCounter.Add(1)
		cfg := playBootstrapCfg
		cfg.PlayerUUID = uuid
		cfg.PlayerName = username

		spawnX, spawnY, spawnZ := cfg.SpawnX, cfg.SpawnY, cfg.SpawnZ
		gameMode := cfg.GameMode
		uuidStr := protocol.UUIDToString(uuid)
		worldDir := "world"
		if pd, loadErr := level.LoadPlayerData(worldDir, uuidStr); loadErr == nil {
			spawnX, spawnY, spawnZ = pd.PosX, pd.PosY, pd.PosZ
			gameMode = pd.GameMode
			logger.Info("loaded playerdata", "session", s.ID(), "player", username, "x", spawnX, "y", spawnY, "z", spawnZ, "gamemode", gameMode)
		}
		cfg.SpawnX, cfg.SpawnY, cfg.SpawnZ = spawnX, spawnY, spawnZ
		cfg.GameMode = gameMode

		if defaultWorld != nil && defaultWorld.Weather() != nil {
			wJoin := defaultWorld.Weather().JoinPackets()
			weatherPkts := make([]protocol.Packet, 0, len(wJoin))
			for _, wp := range wJoin {
				weatherPkts = append(weatherPkts, &playpacket.GameEvent{Event: wp.Event, Value: wp.Value})
			}
			cfg.WeatherPackets = weatherPkts
		}

		if err := session.SendPlayBootstrap(s, entityID, cfg); err != nil {
			logger.Error("play bootstrap failed", "session", s.ID(), "error", err)
			_ = s.ForceClose(err)
			return
		}
		session.StartPlayKeepAlive(s, 10*time.Second)

		player := entity.NewPlayer(
			entityID,
			uuid,
			username,
			entity.Vec3{X: spawnX, Y: spawnY, Z: spawnZ},
			entity.Vec2{},
			s,
			playBootstrapCfg.ViewDistance,
		)
		player.Living().SetGameMode(gameMode)
		s.BindPlayer(player)

		var playerProps []playpacket.PlayerProperty
		if raw, found := ds.SessionData().Load("login_properties"); found {
			if loginProps, ok := raw.([]loginpacket.LoginProperty); ok {
				for _, lp := range loginProps {
					pp := playpacket.PlayerProperty{
						Name:  lp.Name,
						Value: lp.Value,
					}
					if lp.HasSig {
						pp.IsSigned = true
						pp.Signature = lp.Signature
					}
					playerProps = append(playerProps, pp)
				}
			}
		}

		playerManager.AddPlayer(&session.OnlinePlayer{
			Session:    s,
			EntityID:   entityID,
			UUID:       uuid,
			Name:       username,
			GameMode:   gameMode,
			Properties: playerProps,
			X:          spawnX,
			Y:          spawnY,
			Z:          spawnZ,
			Windows:    inventory.NewWindowManager(),
		})

		if op := playerManager.GetByUUID(uuid); op != nil && op.Windows != nil {
			op.Windows.SetCraftMatcher(crafting.Match)
		}

		if defaultWorld != nil {
			w := defaultWorld
			w.Scheduler().Schedule(func() {
				w.AddPlayer(player)
				logger.Info("player added to world", "session", s.ID(), "player", username)
			})

			go func() {
				<-s.Context().Done()
				if op := playerManager.GetByUUID(uuid); op != nil {
					pd := &level.PlayerData{
						UUID:      uuidStr,
						PosX:      op.X,
						PosY:      op.Y,
						PosZ:      op.Z,
						Yaw:       op.Yaw,
						Pitch:     op.Pitch,
						GameMode:  op.GameMode,
						Health:    20.0,
						FoodLevel: 20,
						FoodSat:   5.0,
						Dimension: "minecraft:overworld",
					}
					if saveErr := pd.Save(worldDir); saveErr != nil {
						logger.Error("save playerdata failed", "player", username, "error", saveErr)
					} else {
						logger.Info("saved playerdata", "player", username, "x", op.X, "y", op.Y, "z", op.Z)
					}
				}
				playerManager.RemovePlayer(uuid)
				w.Scheduler().Schedule(func() {
					w.RemovePlayer(player)
					logger.Info("player removed from world", "player", username)
				})
			}()
		} else {
			go func() {
				<-s.Context().Done()
				playerManager.RemovePlayer(uuid)
			}()
		}
	}); err != nil {
		logger.Error("register configuration handlers failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	var worldAccessor session.WorldAccessor
	if defaultWorld != nil {
		worldAccessor = &session.DefaultWorldAccessor{
			Chunks:        defaultWorld.ChunkManager(),
			TickScheduler: defaultWorld.TickScheduler(),
			CurrentTick:   defaultWorld.CurrentTick,
			NextEntityID:  func() int32 { return entityIDCounter.Add(1) },
			Items:         defaultWorld.ItemEntities(),
		}
	}

	if err := session.RegisterPlayHandlers(router, playBootstrapCfg, playerManager, worldAccessor); err != nil {
		logger.Error("register play handlers failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	inboundHandler := session.NewInboundHandler(sessionManager)
	listenerAddress := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	listener, err := network.NewListener(network.ListenerConfig{
		Address:             listenerAddress,
		MaxConnections:      cfg.Server.MaxPlayers,
		WorkerPoolSize:      maxInt(cfg.Performance.IOWorkerPoolSize, 1),
		WorkerQueueCapacity: maxInt(cfg.Network.MaxInboundQueueCapacity, 1024),
		PipelineFactory: func(bufferPool *network.BufferPool, maxFrameSize int) network.Pipeline {
			pipeline := network.NewPipeline(bufferPool, maxFrameSize)
			_ = pipeline.AddLast("session_inbound", inboundHandler)
			return pipeline
		},
		OnConnection: func(conn *network.Conn) error {
			_, createErr := sessionManager.Create(conn)
			return createErr
		},
		OnError: func(listenerErr error) {
			if listenerErr != nil {
				logger.Error("listener error", "error", listenerErr)
			}
		},
		ConnConfig: network.ConnConfig{
			ReadBufferSize:      maxInt(cfg.Performance.ReadBufferSize, 4096),
			ReadAccumulatorSize: maxInt(cfg.Performance.ReadBufferSize*2, 8192),
			MaxFrameSize:        maxInt(cfg.Network.MaxPacketSize, 2<<20),
			WriteQueueCapacity:  maxInt(cfg.Performance.OutboundQueueSize, 1024),
			ReadDeadline:        toDuration(cfg.Network.ReadTimeoutSeconds),
			WriteDeadline:       toDuration(cfg.Network.WriteTimeoutSeconds),
		},
	})
	if err != nil {
		logger.Error("create listener failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	if err := cfgManager.RegisterReloadHook(validateRuntimeReloadableChange); err != nil {
		logger.Error("register config reload hook failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = listener.Close()
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	reloadContext, reloadCancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		watchErr := cfgManager.Watch(reloadContext, config.WatchOptions{
			OnReload: func(next *config.Config) {
				listener.SetMaxConnections(next.Server.MaxPlayers)
				logger.Info("config hot reload applied", "path", cfgManager.Path(), "max_players", next.Server.MaxPlayers, "motd", next.Server.MOTD)
			},
			OnError: func(watchErr error) {
				logger.Error("config hot reload failed", "error", watchErr)
			},
		})
		if watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			logger.Warn("config watcher stopped", "error", watchErr)
		}
	}()

	if err := listener.Start(); err != nil {
		logger.Error("start listener failed", "error", err)
		reloadCancel()
		<-watchDone
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = listener.Close()
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		_ = packetPool.Stop(shutdownContext)
		cancel()
		return 1
	}

	logger.Info("Vitis listening", "address", listener.Addr().String())

	session.StartConsoleReader(cmdRegistry, func() {
		shutdownSignal <- syscall.SIGTERM
	})

	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal
	signal.Stop(shutdownSignal)

	reloadCancel()
	<-watchDone

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	listenerErr := listener.Shutdown(shutdownContext)
	worldErr := shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
	poolErr := packetPool.Stop(shutdownContext)

	if listenerErr != nil && !errors.Is(listenerErr, context.Canceled) && !errors.Is(listenerErr, context.DeadlineExceeded) {
		logger.Error("listener shutdown failed", "error", listenerErr)
		return 1
	}
	if poolErr != nil && !errors.Is(poolErr, context.Canceled) && !errors.Is(poolErr, context.DeadlineExceeded) {
		logger.Error("packet worker shutdown failed", "error", poolErr)
		return 1
	}
	if worldErr != nil && !errors.Is(worldErr, context.Canceled) && !errors.Is(worldErr, context.DeadlineExceeded) {
		logger.Error("world runtime shutdown failed", "error", worldErr)
		return 1
	}

	logger.Info("Vitis shutdown complete")
	return 0
}

func validateRuntimeReloadableChange(oldConfig *config.Config, nextConfig *config.Config) error {
	if nextConfig == nil {
		return fmt.Errorf("validate runtime reloadability: nil config")
	}
	if oldConfig == nil {
		return nil
	}

	var reloadErr error

	if oldConfig.Server.Host != nextConfig.Server.Host {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("server.host requires restart"))
	}
	if oldConfig.Server.Port != nextConfig.Server.Port {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("server.port requires restart"))
	}
	if oldConfig.Server.OnlineMode != nextConfig.Server.OnlineMode {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("server.online_mode requires restart"))
	}
	if oldConfig.Network.ReadTimeoutSeconds != nextConfig.Network.ReadTimeoutSeconds {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.read_timeout_seconds requires restart"))
	}
	if oldConfig.Network.WriteTimeoutSeconds != nextConfig.Network.WriteTimeoutSeconds {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.write_timeout_seconds requires restart"))
	}
	if oldConfig.Network.IdleTimeoutSeconds != nextConfig.Network.IdleTimeoutSeconds {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.idle_timeout_seconds requires restart"))
	}
	if oldConfig.Network.MaxPacketSize != nextConfig.Network.MaxPacketSize {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.max_packet_size requires restart"))
	}
	if oldConfig.Network.MaxInboundQueueCapacity != nextConfig.Network.MaxInboundQueueCapacity {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.max_inbound_queue_capacity requires restart"))
	}
	if oldConfig.Tick != nextConfig.Tick {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("tick settings require restart"))
	}
	if oldConfig.World != nextConfig.World {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("world settings require restart"))
	}
	if oldConfig.Performance.IOWorkerPoolSize != nextConfig.Performance.IOWorkerPoolSize {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("performance.io_worker_pool_size requires restart"))
	}
	if oldConfig.Performance.PacketWorkerPoolSize != nextConfig.Performance.PacketWorkerPoolSize {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("performance.packet_worker_pool_size requires restart"))
	}
	if oldConfig.Performance.ReadBufferSize != nextConfig.Performance.ReadBufferSize {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("performance.read_buffer_size requires restart"))
	}
	if oldConfig.Performance.WriteBufferSize != nextConfig.Performance.WriteBufferSize {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("performance.write_buffer_size requires restart"))
	}
	if oldConfig.Performance.OutboundQueueSize != nextConfig.Performance.OutboundQueueSize {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("performance.outbound_queue_size requires restart"))
	}

	if reloadErr != nil {
		return fmt.Errorf("hot reload rejected: %w", reloadErr)
	}
	return nil
}

func saturatingInt64ToInt(value int64) int {
	if value <= 0 {
		return 0
	}
	maxInt := int64(int(^uint(0) >> 1))
	if value > maxInt {
		return int(maxInt)
	}
	return int(value)
}

func toDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func bootstrapWorldRuntime(cfg *config.Config) (*world.Manager, *tick.Loop, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("bootstrap world runtime: nil config")
	}

	worldManager := world.NewManager()
	defaultWorldName := cfg.World.DefaultWorldName
	if defaultWorldName == "" {
		defaultWorldName = "world"
	}

	worldDir := defaultWorldName
	chunkStore := persistence.NewChunkStore(worldDir)

	_, err := worldManager.CreateWithConfig(world.Config{
		Name:     defaultWorldName,
		WorldDir: worldDir,
		Seed:     42,
		BiomeID:  0,

		ChunkStore: chunkStore,

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
		return nil, nil, err
	}

	loop, err := tick.NewLoop(tick.LoopConfig{
		TargetTPS:          cfg.Tick.TargetTPS,
		MaxCatchUpTicks:    cfg.Tick.MaxCatchUpTicks,
		OverloadMode:       tick.OverloadCatchUp,
		CancelPendingTasks: true,
	}, worldManager)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = worldManager.Close(shutdownCtx)
		cancel()
		return nil, nil, err
	}

	return worldManager, loop, nil
}

func shutdownWorldRuntime(ctx context.Context, worldManager *world.Manager, tickLoop *tick.Loop) error {
	var shutdownErr error

	if tickLoop != nil {
		if err := tickLoop.Stop(ctx); err != nil && !errors.Is(err, tick.ErrLoopStopped) {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	if worldManager != nil {
		if err := worldManager.Close(ctx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	return shutdownErr
}
