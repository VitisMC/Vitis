package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/logger"

	"github.com/vitismc/vitis/internal/command"
	"github.com/vitismc/vitis/internal/config"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/data"
	"github.com/vitismc/vitis/internal/data/vanilla"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/equipment"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/loot"
	"github.com/vitismc/vitis/internal/metrics"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/network/query"
	"github.com/vitismc/vitis/internal/network/rcon"
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
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/level"
	"github.com/vitismc/vitis/internal/world/mapdata"
	"github.com/vitismc/vitis/internal/world/persistence"
	"github.com/vitismc/vitis/internal/world/spawning"
	"github.com/vitismc/vitis/internal/world/worldgen"
)

const (
	shutdownGracePeriod = 5 * time.Second
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", config.DefaultConfigPath, "path to configuration file (created from defaults if missing)")
	flag.Parse()

	startupContext, stopStartup := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopStartup()
	if err := initializeServerFiles(".", *configPath); err != nil {
		logger.Error("initialize server files failed", "error", err)
		return 1
	}

	cfgManager := config.NewManager(*configPath)
	cfg, err := cfgManager.Load()
	if err != nil {
		logger.Error("load config failed", "error", err)
		return 1
	}

	logger.Init(cfg.Logging.Level, cfg.Logging.Format, nil)
	worldDirectory := cfg.World.DefaultWorldName
	if worldDirectory == "" {
		worldDirectory = "world"
	}
	vanillaPackPath := filepath.Join(worldDirectory, "vanilla", protocol.CurrentMinecraftVersion)
	expected := vanilla.Version{ID: protocol.CurrentMinecraftVersion, Protocol: protocol.CurrentVersion, WorldVersion: protocol.CurrentDataVersion}
	expected.PackVersion.Data = datapack.CurrentPackFormat
	if err := preflightWorld(worldDirectory, cfg); err != nil {
		logger.Error("prepare world failed", "error", err)
		return 1
	}
	logger.Info("preparing server resources", "minecraft", expected.ID, "path", vanillaPackPath)
	legacyPackPath := filepath.Join(".mcdata", expected.ID, "vanilla")
	if err := vanilla.PrepareWithSource(startupContext, expected, vanillaPackPath, legacyPackPath); err != nil {
		logger.Error("prepare server resources failed", "error", err)
		return 1
	}
	if err := startupContext.Err(); err != nil {
		logger.Error("startup canceled", "error", err)
		return 1
	}
	stopStartup()
	resourceManager := &datapack.Manager{}
	worldgenRuntime, err := worldgen.NewRuntime(resourceManager)
	if err != nil {
		logger.Error("initialize worldgen resources failed", "error", err)
		return 1
	}
	advancementRuntime, err := advancement.NewRuntime(resourceManager)
	if err != nil {
		logger.Error("initialize advancement resources failed", "error", err)
		return 1
	}
	craftingRuntime, err := crafting.NewRuntime(resourceManager)
	if err != nil {
		logger.Error("initialize recipe resources failed", "error", err)
		return 1
	}
	mapStore, err := mapdata.Open(worldDirectory)
	if err != nil {
		logger.Error("load map data failed", "error", err)
		return 1
	}
	craftingRuntime.SetMapExtensionValidator(mapStore.CanExtend)
	lootRuntime, err := loot.NewRuntime(resourceManager)
	if err != nil {
		logger.Error("initialize loot resources failed", "error", err)
		return 1
	}
	spawningRuntime, err := spawning.NewRuntime(resourceManager)
	if err != nil {
		logger.Error("initialize spawning resources failed", "error", err)
		return 1
	}
	equipmentRuntime, err := equipment.NewRuntime(resourceManager)
	if err != nil {
		logger.Error("initialize equipment resources failed", "error", err)
		return 1
	}
	if err := resourceManager.ReloadWorld(vanillaPackPath, worldDirectory); err != nil {
		logger.Error("load server resources failed", "error", err)
		return 1
	}
	resourceCount := 0
	if snapshot := resourceManager.Current(); snapshot != nil {
		resourceCount = len(snapshot.Names())
	}
	logger.Info("server resources loaded", "resources", resourceCount, "advancements", advancementRuntime.Count(), "recipes", craftingRuntime.RecipeCount(), "loot_tables", lootRuntime.TableCount(), "worldgen", worldgenRuntime.Count())

	chunkActivity := &metrics.ChunkActivity{}
	worldManager, tickLoop, err := bootstrapWorldRuntime(cfg, worldgenRuntime, lootRuntime, chunkActivity, spawningRuntime)
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

	sessionManager, err := session.NewManager(session.ManagerConfig{
		Registry:     protocolRegistry,
		Router:       router,
		InitialState: protocol.StateHandshake,
		PhaseTimeout: toDuration(cfg.Network.IdleTimeoutSeconds),
	})
	if err != nil {
		logger.Error("create session manager failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	favicon := os.Getenv("VITIS_FAVICON")
	statusProvider := session.StatusInfoProviderFunc(func(_ session.Session) session.StatusInfo {
		activeConfig := cfgManager.Get()

		return session.StatusInfo{
			VersionName:     protocol.CurrentMinecraftVersion,
			ProtocolVersion: protocol.CurrentVersion,
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
		cancel()
		return 1
	}

	var entityIDCounter atomic.Int32
	playerManager := session.NewPlayerManager()
	playerManager.SetMaxPlayers(cfg.Server.MaxPlayers)

	tabCfg := config.LoadTabConfig("configs/tab.yaml")

	playBootstrapCfg := session.DefaultPlayBootstrapConfig()
	playBootstrapCfg.Crafting = craftingRuntime
	playBootstrapCfg.RecipeAdvancements = advancementRuntime
	playBootstrapCfg.ViewDistance = int32(cfg.World.ViewDistance)
	if playBootstrapCfg.ViewDistance <= 0 {
		playBootstrapCfg.ViewDistance = 10
	}
	playBootstrapCfg.SimulationDistance = int32(cfg.World.SimulationDistance)
	playBootstrapCfg.GameMode = session.GameModeFromString(cfg.Server.DefaultGameMode)
	playBootstrapCfg.Difficulty = session.DifficultyFromString(cfg.World.Difficulty)
	playBootstrapCfg.RegistryManager = regMgr
	playBootstrapCfg.TabHeader, playBootstrapCfg.TabFooter = tabCfg.RenderLive(config.TabStats{Max: cfg.Server.MaxPlayers, TPS: 20})
	playBootstrapCfg.ChatRateLimit = cfg.Security.ChatMessagesPer10Seconds
	playBootstrapCfg.CommandRateLimit = cfg.Security.CommandsPer10Seconds
	failPolicyLoad := func(policy string, loadErr error) int {
		logger.Error("refusing to start with unreadable access policy", "policy", policy, "error", loadErr)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	opList := operator.NewList("ops.json")
	if err := opList.Load(); err != nil {
		return failPolicyLoad("ops.json", err)
	} else {
		logger.Info("operator list loaded", "count", opList.Count())
	}
	playBootstrapCfg.OperatorList = opList

	whitelist := data.NewWhitelist("whitelist.json")
	if err := whitelist.Load(); err != nil {
		return failPolicyLoad("whitelist.json", err)
	} else {
		logger.Info("whitelist loaded", "count", whitelist.Count())
	}

	banList := data.NewBanList("banned-players.json")
	if err := banList.Load(); err != nil {
		return failPolicyLoad("banned-players.json", err)
	} else {
		logger.Info("ban list loaded", "count", banList.Count())
	}

	ipBanList := data.NewIPBanList("banned-ips.json")
	if err := ipBanList.Load(); err != nil {
		return failPolicyLoad("banned-ips.json", err)
	} else {
		logger.Info("ip ban list loaded", "count", ipBanList.Count())
	}

	shutdownSignal := make(chan os.Signal, 1)
	cmdRegistry := command.NewRegistry()
	serverControl := session.NewServerControl(playerManager, func() {
		shutdownSignal <- syscall.SIGTERM
	}, opList)
	if err := serverControl.SetDefaultGameMode(playBootstrapCfg.GameMode); err != nil {
		logger.Warn("initialize default game mode failed", "error", err)
	}
	if err := serverControl.SetWorldSpawn(int(playBootstrapCfg.SpawnX), int(playBootstrapCfg.SpawnY), int(playBootstrapCfg.SpawnZ)); err != nil {
		logger.Warn("initialize world spawn failed", "error", err)
	}
	if control, ok := serverControl.(*session.ServerControlAdapter); ok {
		control.PregenerationWorlds = worldManager
		control.DefaultPregenerationWorld = worldDirectory
		control.ResourceReload = func() error {
			return resourceManager.ReloadWorld(vanillaPackPath, worldDirectory)
		}
	}
	playBootstrapCfg.KeepInventory = func() bool {
		if control, ok := serverControl.(*session.ServerControlAdapter); ok {
			value, _ := control.GetGameRule("keepInventory")
			return value == "true"
		}
		return false
	}
	playerLookup := session.NewPlayerLookup(playerManager, opList)
	command.RegisterBuiltinCommands(cmdRegistry, playerLookup, serverControl)
	command.RegisterMultiplayerCommands(cmdRegistry, nil, nil, nil)
	command.RegisterWhitelistCommands(cmdRegistry, whitelist)
	command.RegisterBanCommands(cmdRegistry, &data.BanListAdapter{BL: banList}, ipBanList, &banKicker{players: playerLookup, server: serverControl})

	playBootstrapCfg.CommandRegistry = cmdRegistry
	logger.Info("command system initialized", "commands", cmdRegistry.Count())

	loginCfg := session.LoginConfig{
		CompressionThreshold: cfg.Network.CompressionThreshold,
		OnlineMode:           cfg.Server.OnlineMode,
		OnLoginSuccess: func(s session.Session, name string, uuid protocol.UUID) {
			logger.Info("player login", "name", name, "uuid", protocol.UUIDToString(uuid), "session", s.ID())
		},
		WhitelistChecker: func(uuid protocol.UUID) bool {
			if !whitelist.Enabled() {
				return true
			}
			if opList.IsOp(uuid) {
				return true
			}
			return whitelist.IsWhitelisted(uuid)
		},
		BanChecker: func(uuid protocol.UUID) (bool, string) {
			if banList.IsBanned(uuid) {
				return true, banList.GetBanReason(uuid)
			}
			return false, ""
		},
		IPBanChecker: func(remote net.Addr) (bool, string) {
			if remote == nil {
				return false, ""
			}
			host, _, splitErr := net.SplitHostPort(remote.String())
			if splitErr != nil {
				host = remote.String()
			}
			if ipBanList.IsBanned(host) {
				return true, ipBanList.GetBanReason(host)
			}
			return false, ""
		},
	}
	if cfg.Server.OnlineMode {
		privKey, err := protocrypto.GenerateKeyPair()
		if err != nil {
			logger.Error("generate rsa keypair failed", "error", err)
			shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
			cancel()
			return 1
		}
		pubKeyDER, err := protocrypto.EncodePublicKey(&privKey.PublicKey)
		if err != nil {
			logger.Error("encode public key failed", "error", err)
			shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
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
		cancel()
		return 1
	}

	defaultWorld, worldFound := worldManager.Get(cfg.World.DefaultWorldName)
	if !worldFound {
		dw, _ := worldManager.Get("world")
		defaultWorld = dw
	}

	for _, ticker := range worldManager.Worlds() {
		w := ticker.(*world.World)
		localPlayers := playerManager.ForDimension(w.Dimension())
		if !w.Submit(func() {
			w.SetEquipmentRuntime(equipmentRuntime)
			w.SetPlayerInventoryHandler(func(player *entity.Player) *inventory.WindowManager {
				if online := localPlayers.GetByEntityID(player.ID()); online != nil {
					return online.Windows
				}
				return nil
			})
			w.SetEntitySpawnStateHandler(func(observer *entity.Player, base *entity.Entity) {
				if online := localPlayers.GetByEntityID(base.ID()); online != nil {
					if packet := session.BuildFullEquipmentPacket(base.ID(), online.Windows); packet != nil {
						_ = observer.Session().Send(packet)
					}
				}
			})
			w.SetItemPickupHandler(func(player *entity.Player, stack inventory.Slot) int32 {
				playerSession, ok := player.Session().(session.Session)
				if !ok {
					return 0
				}
				op := localPlayers.GetBySession(playerSession)
				if op == nil || op.Windows == nil {
					return 0
				}
				indices := make([]int, 0, 36)
				for i := inventory.HotbarStart; i <= inventory.HotbarEnd; i++ {
					indices = append(indices, i)
				}
				for i := inventory.MainInventoryStart; i <= inventory.MainInventoryEnd; i++ {
					indices = append(indices, i)
				}
				remainder, _ := op.Windows.Inventory().Insert(stack, stack.MaxStackSize(), indices)
				count := stack.ItemCount - remainder.ItemCount
				if count > 0 {
					stateID, slots, cursor := op.Windows.SnapshotForResync(0)
					_ = op.Session.Send(&playpacket.SetContainerContent{WindowID: 0, StateID: stateID, Slots: slots, Carried: cursor})
				}
				return count
			})
			w.SetFurnaceUpdateHandler(func(x, y, z int32, slots []inventory.Slot, properties [4]int16) {
				for _, op := range localPlayers.Players() {
					if op == nil || op.Windows == nil {
						continue
					}
					active := op.Windows.ActiveWindow()
					if active == nil || active.BlockX != int(x) || active.BlockY != int(y) || active.BlockZ != int(z) {
						continue
					}
					if active.Type != inventory.WindowTypeFurnace && active.Type != inventory.WindowTypeBlastFurnace && active.Type != inventory.WindowTypeSmoker {
						continue
					}
					changed := false
					for i := range slots {
						current := active.Container.Get(i)
						if current.ItemCount != slots[i].ItemCount || !current.StackableWith(slots[i]) {
							changed = true
							break
						}
					}
					if changed {
						stateID, contents, cursor := op.Windows.SnapshotForResync(active.ID)
						_ = op.Session.Send(&playpacket.SetContainerContent{WindowID: active.ID, StateID: stateID, Slots: contents, Carried: cursor})
					}
					for i, value := range properties {
						_ = op.Session.Send(&playpacket.SetContainerProperty{WindowID: active.ID, Property: int16(i), Value: value})
					}
				}
			})
		}) {
			logger.Error("install item pickup handler failed", "error", "world command queue is full")
		}
	}
	if defaultWorld != nil {
		if sc, ok := serverControl.(*session.ServerControlAdapter); ok {
			sc.World = defaultWorld
			sc.WeatherWorld = defaultWorld
			sc.MobSpawn = defaultWorld
			sc.SeedValue = defaultWorld.Seed()
		}
	}

	playerWriter := level.NewPlayerWriter(maxInt(cfg.World.AutosaveQueueCapacity, 1), func(pd *level.PlayerData) error {
		dir := cfg.World.DefaultWorldName
		if dir == "" {
			dir = "world"
		}
		if err := mapStore.Save(); err != nil {
			return err
		}
		return pd.Save(dir)
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		if err := playerWriter.Close(ctx); err != nil {
			logger.Error("close player writer failed", "error", err)
		}
	}()
	var snapshotSequence atomic.Uint64
	// Autosave bases preserve uninterpreted player fields and are owned by the tick.
	playerBases := make(map[protocol.UUID]*level.PlayerData)
	pendingPlayerSaves := make(map[protocol.UUID]level.PlayerWrite)
	var playerLifecycle sync.WaitGroup
	var playerLifecycleMu sync.Mutex
	var playerSaveFailed atomic.Bool
	playerLifecycleAccepting := true
	startPlayerLifecycle := func(task func()) bool {
		playerLifecycleMu.Lock()
		defer playerLifecycleMu.Unlock()
		if !playerLifecycleAccepting {
			return false
		}
		playerLifecycle.Add(1)
		go func() {
			defer playerLifecycle.Done()
			task()
		}()
		return true
	}

	var worldAccessor session.WorldAccessor
	worldAccessors := make(map[string]*session.DefaultWorldAccessor)
	playConfigs := make(map[string]session.PlayBootstrapConfig)
	var playWorlds []session.PlayWorld
	for _, ticker := range worldManager.Worlds() {
		w := ticker.(*world.World)
		accessor := newWorldAccessor(w)
		worldAccessors[w.Dimension()] = accessor
		playConfig := playConfigForWorld(playBootstrapCfg, w)
		playConfig.CreateMap = func(x, z int32) (inventory.ItemStack, error) { return mapStore.Create(w.Dimension(), x, z) }
		playConfigs[w.Dimension()] = playConfig
		playWorlds = append(playWorlds, session.PlayWorld{Dimension: w.Dimension(), Accessor: accessor, Config: playConfig})
	}
	if defaultWorld != nil {
		worldAccessor = worldAccessors[defaultWorld.Dimension()]
	}
	transfers, transferErr := session.NewDimensionTransfers(playerManager, worldManager, playWorlds)
	if transferErr != nil {
		logger.Error("configure dimension transfers failed", "error", transferErr)
		return 1
	}
	for i := range playWorlds {
		playWorlds[i].Config.RequestRespawn = transfers.RequestRespawn
		playConfigs[playWorlds[i].Dimension] = playWorlds[i].Config
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
		if defaultWorld != nil && defaultWorld.EntityManager() != nil {
			entityID = defaultWorld.EntityManager().AllocateID()
		}
		loginWorld := defaultWorld
		cfg := playBootstrapCfg
		cfg.Difficulty = int32(serverControl.GetDifficulty())
		cfg.GameMode = serverControl.GetDefaultGameMode()
		cfg.PlayerUUID = uuid
		cfg.PlayerName = username

		spawnX, spawnY, spawnZ := cfg.SpawnX, cfg.SpawnY, cfg.SpawnZ
		spawnYaw, spawnPitch := cfg.SpawnYaw, cfg.SpawnPitch
		if cfg.ReadWorldSpawn != nil {
			x, y, z, angle := cfg.ReadWorldSpawn()
			spawnX, spawnY, spawnZ = float64(x)+0.5, float64(y), float64(z)+0.5
			spawnYaw = angle
		}
		gameMode := cfg.GameMode
		uuidStr := protocol.UUIDToString(uuid)
		if playerManager.GetByUUID(uuid) != nil {
			_ = s.ForceClose(errors.New("player is already connected"))
			return
		}
		worldDir := "world"
		if defaultWorld != nil {
			worldDir = defaultWorld.Name()
		}
		if worldDir == "" {
			worldDir = "world"
		}
		var loadedPD *level.PlayerData
		if pd, loadErr := loadPlayerSnapshot(playerWriter, worldDir, uuidStr); loadErr == nil {
			spawnX, spawnY, spawnZ = pd.PosX, pd.PosY, pd.PosZ
			spawnYaw, spawnPitch = pd.Yaw, pd.Pitch
			gameMode = pd.GameMode
			if pd.Dimension != "" {
				var found bool
				loginWorld, found = worldManager.GetByDimension(pd.Dimension)
				if !found {
					_ = s.ForceClose(fmt.Errorf("saved player dimension %s is unavailable", pd.Dimension))
					return
				}
			}
			loadedPD = pd
			logger.Info("loaded playerdata", "session", s.ID(), "player", username, "x", spawnX, "y", spawnY, "z", spawnZ, "gamemode", gameMode)
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			logger.Error("playerdata load failed", "session", s.ID(), "player", username, "error", loadErr)
			_ = s.ForceClose(loadErr)
			return
		}
		if loginWorld != nil {
			ds.BindWorldDimension(loginWorld.Dimension())
			cfg = playConfigs[loginWorld.Dimension()]
			cfg.Difficulty = loginWorld.Difficulty()
			cfg.PlayerUUID, cfg.PlayerName = uuid, username
		}
		if loginWorld != nil {
			ctx, cancel := context.WithTimeout(s.Context(), 30*time.Second)
			defer releaseLoginSpawn(loginWorld, s.ID())
			position := entity.Vec3{X: spawnX, Y: spawnY, Z: spawnZ}
			err := retainLoginSpawn(ctx, loginWorld, s.ID(), position)
			if err == nil {
				position, err = session.PreparePlayerSpawn(ctx, worldAccessors[loginWorld.Dimension()], position, loadedPD != nil)
			}
			cancel()
			if err != nil {
				logger.Warn("prepare login spawn failed", "player", username, "error", err)
				_ = s.ForceClose(err)
				return
			}
			spawnX, spawnY, spawnZ = position.X, position.Y, position.Z
		}
		cfg.SpawnX, cfg.SpawnY, cfg.SpawnZ = spawnX, spawnY, spawnZ
		cfg.SpawnYaw, cfg.SpawnPitch = spawnYaw, spawnPitch
		cfg.GameMode = gameMode
		if loadedPD != nil {
			cfg.InitialHealth = loadedPD.Health
			cfg.InitialFood = loadedPD.FoodLevel
			cfg.InitialFoodSaturation = loadedPD.FoodSat
			cfg.InitialXPLevel = loadedPD.XPLevel
			cfg.InitialXPTotal = loadedPD.XPTotal
			cfg.InitialXPBar = loadedPD.XPBar
			cfg.InitialSelectedSlot = loadedPD.SelectedItemSlot
			cfg.PortalCooldown = loadedPD.PortalCooldown
		}
		playerViewDistance := cfg.ViewDistance
		if raw, found := ds.SessionData().Load("client_view_distance"); found {
			if requested, ok := raw.(int32); ok && requested >= 2 && requested < playerViewDistance {
				playerViewDistance = requested
			}
		}

		if loginWorld != nil && loginWorld.Weather() != nil {
			cfg.WorldAge = loginWorld.WorldAge()
			cfg.TimeOfDay = loginWorld.TimeOfDay()
			cfg.TickDayTime = loginWorld.DaylightCycle()
			wJoin := loginWorld.Weather().JoinPackets()
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
			entity.Vec2{X: spawnYaw, Y: spawnPitch},
			s,
			playerViewDistance,
		)
		player.Living().SetGameMode(gameMode)
		if loadedPD != nil {
			living := player.Living()
			living.SetHealth(loadedPD.Health)
			living.SetFoodLevel(loadedPD.FoodLevel)
			living.SetFoodSaturation(loadedPD.FoodSat)
			living.SetFoodExhaustion(loadedPD.FoodExhaustion)
			living.SetXP(loadedPD.XPLevel, loadedPD.XPTotal, loadedPD.XPBar)
			for _, active := range loadedPD.Effects {
				if living.Effects().Add(active) {
					_ = s.Send(&playpacket.EntityEffect{EntityID: entityID, EffectID: active.ID, Amplifier: active.Amplifier, Duration: active.Duration, Flags: active.Flags})
				}
			}
		}
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

		onlinePlayer := &session.OnlinePlayer{
			Session:         s,
			EntityID:        entityID,
			UUID:            uuid,
			Name:            username,
			GameMode:        gameMode,
			ManagedTracking: loginWorld != nil,
			Properties:      playerProps,
			X:               spawnX,
			Y:               spawnY,
			Z:               spawnZ,
			Windows:         inventory.NewWindowManager(),
		}
		onlinePlayer.RecipeBook = crafting.NewRecipeBook()
		if loadedPD != nil {
			onlinePlayer.PortalCooldown = loadedPD.PortalCooldown
			onlinePlayer.RecipeBook = crafting.RestoreBook(loadedPD.RecipeBook)
			onlinePlayer.RecipeProgress = loadedPD.RecipeProgress.Clone()
		}
		if loadedPD != nil && loadedPD.HasSpawnPoint {
			onlinePlayer.SpawnX, onlinePlayer.SpawnY, onlinePlayer.SpawnZ = int(loadedPD.SpawnX), int(loadedPD.SpawnY), int(loadedPD.SpawnZ)
			onlinePlayer.HasSpawnPoint = true
			onlinePlayer.SpawnDimension, onlinePlayer.SpawnAngle, onlinePlayer.SpawnForced = loadedPD.SpawnDimension, loadedPD.SpawnAngle, loadedPD.SpawnForced
		}

		if op := onlinePlayer; op.Windows != nil {
			op.Windows.SetHeldSlot(cfg.InitialSelectedSlot)
			op.Windows.SetCraftMatcher(craftingRuntime.Match)
			op.Windows.SetStonecuttingMatcher(craftingRuntime.StonecuttingChoices)
			op.Windows.SetCraftEvaluator(func(grid []inventory.ItemStack, width int) (inventory.CraftResult, bool) {
				result, ok := craftingRuntime.Evaluate(grid, width)
				if ok && playConfigs[session.SessionDimension(s)].GameRule != nil && playConfigs[session.SessionDimension(s)].GameRule("doLimitedCrafting") == "true" && !craftingRuntime.IsSpecial(result.RecipeID) && !op.RecipeBook.KnowsRecipe(result.RecipeID) {
					return inventory.CraftResult{}, false
				}
				return result, ok
			})
			op.Windows.SetCraftCommit(mapStore.CommitCraft)
			if loadedPD != nil && len(loadedPD.Inventory) > 0 {
				inv := op.Windows.Inventory()
				for _, is := range loadedPD.Inventory {
					itemID := item.IDByName(is.ItemName)
					if itemID < 0 {
						continue
					}
					windowSlot, valid := level.NBTSlotToWindow(is.Slot)
					if !valid {
						continue
					}
					slot := is.Stack.Clone()
					if slot.Empty() {
						slot = inventory.NewSlot(itemID, is.Count)
						slot.SetDamage(is.Damage)
						for id, level := range is.Enchantments {
							slot.SetEnchant(id, level)
						}
					}
					inv.Set(windowSlot, slot)
				}
			}
			op.Windows.UpdateCraftOutput()
			stateID, slots, cursor := op.Windows.Snapshot(0)
			if err := s.Send(&playpacket.SetContainerContent{WindowID: 0, StateID: stateID, Slots: slots, Carried: cursor}); err != nil {
				playerManager.RemovePlayer(uuid, s)
				_ = s.ForceClose(err)
				return
			}
			session.UpdateArmorStats(op)
		}

		if loginWorld != nil {
			w := loginWorld
			if !w.Submit(func() {
				if s.Context().Err() != nil {
					return
				}
				if err := playerManager.AddPlayer(onlinePlayer); err != nil {
					_ = s.ForceClose(err)
					return
				}
				w.AddPlayer(player)
				playerBases[uuid] = loadedPD
				logger.Info("player added to world", "session", s.ID(), "player", username)
			}) {
				playerManager.RemovePlayer(uuid, s)
				_ = s.ForceClose(errors.New("world command queue is full"))
				return
			}

			if !startPlayerLifecycle(func() {
				<-s.Context().Done()
				defer playerManager.RemovePlayer(uuid, s)
				ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
				defer cancel()
				result := make(chan level.PlayerWrite, 1)
				snapshot := func() {
					var pd *level.PlayerData
					if playerManager.GetByUUID(uuid) == onlinePlayer {
						session.PreparePlayerDisconnect(playerManager.ForDimension(session.SessionDimension(s)), onlinePlayer, worldAccessors[session.SessionDimension(s)])
						pd = snapshotPlayerData(loadedPD, uuidStr, onlinePlayer, player)
					}
					if current, found := worldManager.GetByDimension(session.SessionDimension(s)); found {
						current.RemovePlayer(player)
					}
					delete(playerBases, uuid)
					delete(pendingPlayerSaves, uuid)
					result <- level.PlayerWrite{Data: pd, Sequence: snapshotSequence.Add(1)}
				}
				for !w.Submit(snapshot) {
					select {
					case <-ctx.Done():
						playerSaveFailed.Store(true)
						logger.Error("player removal queue remained full", "player", username, "error", ctx.Err())
						return
					case <-time.After(10 * time.Millisecond):
					}
				}
				select {
				case snapshot := <-result:
					if snapshot.Data == nil {
						return
					}
					if saveErr := playerWriter.Enqueue(ctx, snapshot); saveErr != nil {
						playerSaveFailed.Store(true)
						logger.Error("save playerdata failed", "player", username, "error", saveErr)
					} else {
						logger.Info("saved playerdata", "player", username, "x", snapshot.Data.PosX, "y", snapshot.Data.PosY, "z", snapshot.Data.PosZ)
					}
				case <-ctx.Done():
					playerSaveFailed.Store(true)
					logger.Error("player snapshot timed out", "player", username, "error", ctx.Err())
				}
			}) {
				playerManager.RemovePlayer(uuid, s)
			}
		} else {
			if err := playerManager.AddPlayer(onlinePlayer); err != nil {
				_ = s.ForceClose(err)
				return
			}
			if !startPlayerLifecycle(func() {
				<-s.Context().Done()
				playerManager.RemovePlayer(uuid, s)
			}) {
				playerManager.RemovePlayer(uuid, s)
			}
		}
	}); err != nil {
		logger.Error("register configuration handlers failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	if defaultWorld != nil {
		playBootstrapCfg.ReadWorldSpawn = defaultWorld.WorldSpawn
		playBootstrapCfg.GameRule = defaultWorld.GameRule
		playBootstrapCfg.CreateMap = func(x, z int32) (inventory.ItemStack, error) { return mapStore.Create("minecraft:overworld", x, z) }
	}
	if defaultWorld != nil {
		if control, ok := serverControl.(*session.ServerControlAdapter); ok {
			control.WorldAccess = worldAccessor
			control.ResolveWorldAccess = func(s session.Session) session.WorldAccessor { return worldAccessors[session.SessionDimension(s)] }
		}
	}
	for _, ticker := range worldManager.Worlds() {
		w := ticker.(*world.World)
		accessor, localConfig := worldAccessors[w.Dimension()], playConfigs[w.Dimension()]
		localPlayers := playerManager.ForDimension(w.Dimension())
		if !w.Submit(func() {
			if w == defaultWorld {
				worldManager.SetTickHandler(transfers.Tick)
			}
			w.SetPlayerTickHandler(func(tick uint64) {
				session.TickPlayerMovement(localPlayers, accessor)
				session.TickPlayerMenus(localPlayers, accessor)
				session.TickPlayerLifecycle(localPlayers, accessor, localConfig)
				session.TickRecipeBooks(localPlayers, accessor, localConfig)
				interval := maxInt(cfg.World.AutosaveIntervalTicks, 1)
				if w == defaultWorld && tick%uint64(interval) == 0 {
					for _, online := range playerManager.Players() {
						player, ok := online.Session.Player().(*entity.Player)
						if _, present := playerBases[online.UUID]; !present || !ok || player == nil || player.Removed() || online.Session.Context().Err() != nil {
							continue
						}
						pd := snapshotPlayerData(playerBases[online.UUID], protocol.UUIDToString(online.UUID), online, player)
						pendingPlayerSaves[online.UUID] = level.PlayerWrite{Data: pd, Sequence: snapshotSequence.Add(1)}
					}
				}
				if w == defaultWorld {
					for uuid, snapshot := range pendingPlayerSaves {
						if playerWriter.Offer(snapshot) {
							delete(pendingPlayerSaves, uuid)
						}
					}
				}
			})
			w.SetPlayerDeathHandler(func(player *entity.Player) {
				xp := session.ProcessPlayerDeath(player, localPlayers, accessor, localConfig.KeepInventory())
				if xp <= 0 {
					return
				}
				position := player.Position()
				w.AwardExperience(position, xp)
			})
		}) {
			logger.Error("install player death handler failed", "error", "world command queue is full")
		}
	}

	if err := session.RegisterDimensionPlayHandlers(router, playerManager, playWorlds); err != nil {
		logger.Error("register play handlers failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	inboundHandler := session.NewInboundHandler(sessionManager)
	listenerAddress := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	listener, err := network.NewListener(network.ListenerConfig{
		Address:             listenerAddress,
		MaxConnections:      cfg.Network.MaxConnections,
		AdmissionRate:       float64(cfg.Network.AdmissionRatePerSecond),
		AdmissionBurst:      cfg.Network.AdmissionBurst,
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
			IdleTimeout:         toDuration(cfg.Network.IdleTimeoutSeconds),
		},
	})
	if err != nil {
		logger.Error("create listener failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	if err := cfgManager.RegisterReloadHook(validateRuntimeReloadableChange); err != nil {
		logger.Error("register config reload hook failed", "error", err)
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = listener.Close()
		_ = shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
		cancel()
		return 1
	}

	reloadContext, reloadCancel := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		watchErr := cfgManager.Watch(reloadContext, config.WatchOptions{
			OnReload: func(next *config.Config) {
				listener.SetMaxConnections(next.Network.MaxConnections)
				playerManager.SetMaxPlayers(next.Server.MaxPlayers)
				logger.Info("config hot reload applied", "path", cfgManager.Path(), "max_connections", next.Network.MaxConnections, "max_players", next.Server.MaxPlayers, "motd", next.Server.MOTD)
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
		cancel()
		return 1
	}

	logger.Info("Vitis listening", "address", listener.Addr().String())
	tabContext, tabCancel := context.WithCancel(context.Background())
	tabDone := make(chan struct{})
	go func() {
		defer close(tabDone)
		updateLiveTab(tabContext, playerManager, tickLoop, tabCfg, chunkActivity)
	}()

	var rconServer *rcon.Server
	if cfg.RCON.Enabled {
		rconPort := cfg.RCON.Port
		if rconPort <= 0 {
			rconPort = 25575
		}
		rconServer = rcon.New(rcon.Config{
			Address:  net.JoinHostPort(cfg.RCON.Address, strconv.Itoa(rconPort)),
			Password: cfg.RCON.Password,
			Executor: func(cmd string) string {
				var response strings.Builder
				sender := &rconSender{out: &response}
				if err := cmdRegistry.Dispatch(sender, cmd); err != nil {
					return "Error: " + err.Error()
				}
				return response.String()
			},
		})
		if err := rconServer.Start(); err != nil {
			logger.Error("rcon start failed", "error", err)
		} else {
			logger.Info("rcon enabled", "address", cfg.RCON.Address, "port", rconPort)
		}
	}

	var queryServer *query.Server
	if cfg.Query.Enabled {
		queryPort := cfg.Query.Port
		if queryPort <= 0 {
			queryPort = 25565
		}
		queryServer = query.New(query.Config{
			Address: net.JoinHostPort("", strconv.Itoa(queryPort)),
			Info: &query.DynamicServerInfo{
				MotdValue:    cfg.Server.MOTD,
				VersionValue: protocol.CurrentMinecraftVersion,
				MaxPlayersV:  cfg.Server.MaxPlayers,
				Port:         cfg.Server.Port,
				IP:           cfg.Server.Host,
				PlayerList:   func() []string { return playerLookup.OnlinePlayers() },
			},
		})
		if err := queryServer.Start(); err != nil {
			logger.Error("query start failed", "error", err)
		} else {
			logger.Info("query enabled", "port", queryPort)
		}
	}

	session.StartConsoleReader(cmdRegistry, func() {
		shutdownSignal <- syscall.SIGTERM
	})

	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal
	signal.Stop(shutdownSignal)
	tabCancel()
	<-tabDone

	reloadCancel()
	<-watchDone

	if queryServer != nil {
		queryServer.Stop()
	}
	if rconServer != nil {
		rconServer.Stop()
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	playerLifecycleMu.Lock()
	playerLifecycleAccepting = false
	playerLifecycleMu.Unlock()
	listenerErr := listener.Shutdown(shutdownContext)
	playerLifecycleDone := make(chan struct{})
	go func() {
		playerLifecycle.Wait()
		close(playerLifecycleDone)
	}()
	select {
	case <-playerLifecycleDone:
	case <-shutdownContext.Done():
		logger.Error("player lifecycle shutdown timed out", "error", shutdownContext.Err())
		return 1
	}
	writerErr := playerWriter.Close(shutdownContext)
	if writerErr != nil {
		playerSaveFailed.Store(true)
		logger.Error("player writer flush failed", "error", writerErr)
	}
	worldErr := shutdownWorldRuntime(shutdownContext, worldManager, tickLoop)
	if err := mapStore.Save(); err != nil {
		playerSaveFailed.Store(true)
		logger.Error("map data flush failed", "error", err)
	}

	if listenerErr != nil && !errors.Is(listenerErr, context.Canceled) && !errors.Is(listenerErr, context.DeadlineExceeded) {
		logger.Error("listener shutdown failed", "error", listenerErr)
		return 1
	}
	if worldErr != nil && !errors.Is(worldErr, context.Canceled) && !errors.Is(worldErr, context.DeadlineExceeded) {
		logger.Error("world runtime shutdown failed", "error", worldErr)
		return 1
	}
	if playerSaveFailed.Load() {
		logger.Error("shutdown completed with player persistence failures")
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
	if oldConfig.Network.AdmissionRatePerSecond != nextConfig.Network.AdmissionRatePerSecond {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.admission_rate_per_second requires restart"))
	}
	if oldConfig.Network.AdmissionBurst != nextConfig.Network.AdmissionBurst {
		reloadErr = errors.Join(reloadErr, fmt.Errorf("network.admission_burst requires restart"))
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

func bootstrapWorldRuntime(cfg *config.Config, worldgenRuntime *worldgen.Runtime, lootRuntime *loot.Runtime, activity *metrics.ChunkActivity, spawningRuntimes ...*spawning.Runtime) (*world.Manager, *tick.Loop, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("bootstrap world runtime: nil config")
	}

	worldManager := world.NewManager()
	var spawningRuntime *spawning.Runtime
	if len(spawningRuntimes) > 0 {
		spawningRuntime = spawningRuntimes[0]
	}
	defaultWorldName := cfg.World.DefaultWorldName
	if defaultWorldName == "" {
		defaultWorldName = "world"
	}

	worldDir := defaultWorldName
	worldLevel, err := prepareWorldLevel(worldDir, defaultWorldName, cfg)
	if err != nil {
		return nil, nil, err
	}
	createDimension := func(name, dimensionID, storageDir, metadataDir string) error {
		var generator chunk.Generator
		if worldgenRuntime != nil {
			terrainSource, err := worldgenRuntime.NewDimensionTerrainGenerator(dimensionID, worldLevel.Seed, 0)
			if err != nil {
				return fmt.Errorf("create terrain generator %s: %w", dimensionID, err)
			}
			if dimensionID == "minecraft:overworld" && worldLevel.BonusChestEnabled() {
				terrainSource.EnableBonusChest(worldLevel.SpawnX, worldLevel.SpawnZ)
			}
			generator, err = chunk.NewWorldgenGenerator(terrainSource)
			if err != nil {
				return err
			}
		}
		_, err := worldManager.CreateWithConfig(world.Config{
			Name:                   name,
			Dimension:              dimensionID,
			WorldDir:               metadataDir,
			PregenerationStatePath: filepath.Join(storageDir, "pregeneration.json"),
			Seed:                   worldLevel.Seed,
			BiomeID:                0,
			Difficulty:             int32(worldLevel.Difficulty),
			InitialWorldAge:        worldLevel.Time,
			InitialTimeOfDay:       worldLevel.DayTime,
			Metadata:               worldLevel,
			ChunkStore:             persistence.NewChunkStore(storageDir),
			ChunkActivity:          activity,
			Generator:              generator,
			LootRuntime:            lootRuntime,
			SpawningRuntime:        spawningRuntime,
			ReadSeaLevel:           dimensionSeaLevelReader(worldgenRuntime, dimensionID),

			SchedulerQueueCapacity: cfg.World.SchedulerQueueCapacity,
			SchedulerDrainPerTick:  cfg.World.SchedulerDrainPerTick,

			ChunkLoadWorkers:         cfg.World.ChunkLoadWorkers,
			ChunkWorkerQueueSize:     cfg.World.ChunkWorkerQueueSize,
			ChunkRequestQueueSize:    cfg.World.ChunkRequestQueueSize,
			ChunkResultQueueSize:     cfg.World.ChunkResultQueueSize,
			ChunkRequestPumpBatch:    cfg.World.ChunkRequestPumpBatch,
			ChunkCompletionsPerTick:  cfg.World.ChunkCompletionBatch,
			ChunkUnloadBatchPerTick:  cfg.World.ChunkUnloadBatch,
			ChunkUnloadScanBatch:     cfg.World.ChunkUnloadScanBatch,
			ChunkUnloadTTLTicks:      cfg.World.ChunkUnloadTTLTicks,
			ChunkStorageInitCap:      cfg.World.ChunkStorageInitCap,
			ChunkUnloadQueueSize:     cfg.World.ChunkUnloadQueueSize,
			ChunkStreamChunksPerTick: cfg.World.ChunkStreamChunksPerTick,
			ChunkStreamViewDistance:  int32(cfg.World.ViewDistance),
			SimulationDistance:       int32(cfg.World.SimulationDistance),
			AutosaveIntervalTicks:    cfg.World.AutosaveIntervalTicks,
			AutosaveBatchPerTick:     cfg.World.AutosaveBatchPerTick,
			AutosaveQueueCapacity:    cfg.World.AutosaveQueueCapacity,
		})
		return err
	}
	dimensions := []struct {
		name, dimensionID, storageDir, metadataDir string
	}{
		{defaultWorldName, "minecraft:overworld", worldDir, worldDir},
		{"minecraft:the_nether", "minecraft:the_nether", filepath.Join(worldDir, "DIM-1"), ""},
		{"minecraft:the_end", "minecraft:the_end", filepath.Join(worldDir, "DIM1"), ""},
	}
	if worldgenRuntime != nil {
		for _, dimensionID := range worldgenRuntime.Dimensions() {
			if dimensionID == "minecraft:overworld" || dimensionID == "minecraft:the_nether" || dimensionID == "minecraft:the_end" {
				continue
			}
			namespace, local, ok := strings.Cut(dimensionID, ":")
			if !ok {
				return nil, nil, fmt.Errorf("bootstrap world runtime: invalid dimension ID %q", dimensionID)
			}
			storageDir := filepath.Join(worldDir, "dimensions", namespace, filepath.FromSlash(local))
			dimensions = append(dimensions, struct {
				name, dimensionID, storageDir, metadataDir string
			}{dimensionID, dimensionID, storageDir, ""})
		}
	}
	for _, dimension := range dimensions {
		if err := createDimension(dimension.name, dimension.dimensionID, dimension.storageDir, dimension.metadataDir); err != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = worldManager.Close(shutdownCtx)
			cancel()
			return nil, nil, fmt.Errorf("bootstrap world runtime: create %s: %w", dimension.name, err)
		}
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

type rconSender struct {
	out *strings.Builder
}

func (r *rconSender) Name() string                 { return "Rcon" }
func (r *rconSender) SendMessage(text string)      { r.out.WriteString(text + "\n") }
func (r *rconSender) HasPermission(level int) bool { return true }
func (r *rconSender) IsPlayer() bool               { return false }

type banKicker struct {
	players command.PlayerLookup
	server  command.ServerControl
}

func (bk *banKicker) KickPlayer(name string, reason string) error {
	return bk.server.KickPlayer(name, reason)
}

func (bk *banKicker) FindPlayerByName(name string) command.PlayerSender {
	return bk.players.FindPlayerByName(name)
}

func (bk *banKicker) OnlinePlayers() []string {
	return bk.players.OnlinePlayers()
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
