package session

import (
	"context"
	"testing"
	"time"

	effectdata "github.com/vitismc/vitis/internal/data/generated/effect"
	pose "github.com/vitismc/vitis/internal/data/generated/entity_pose"
	"github.com/vitismc/vitis/internal/effect"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/network"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/tick"
)

// transferFixture connects a transition manager to isolated dimension adapters.
func transferFixture(t *testing.T) (*dimensionFixture, *DimensionTransfers) {
	t.Helper()
	f := newDimensionFixture(t)
	transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
	if err != nil {
		t.Fatal(err)
	}
	return f, transfers
}

// TestTransfersRunBeforeAllWorlds verifies one lifecycle advance regardless of world order.
func TestTransfersRunBeforeAllWorlds(t *testing.T) {
	for _, dimensions := range [][]string{
		{"minecraft:overworld", "minecraft:the_nether"},
		{"minecraft:the_nether", "minecraft:overworld"},
	} {
		for source := range dimensions {
			t.Run(dimensions[0]+"/"+dimensions[source], func(t *testing.T) {
				f := newDimensionFixtureWith(t, dimensions)
				transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
				if err != nil {
					t.Fatal(err)
				}
				s := f.sessions[source]
				online := f.players.GetBySession(s)
				online.RespawnRetryTicks = 10
				online.Flags |= 0x80
				online.Pose = pose.PoseFallFlying
				online.movement = movementState{floatingTicks: 42, glidingTicks: 19}
				if err := transfers.Request(s, dimensions[1-source], entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}); err != nil {
					t.Fatal(err)
				}
				f.manager.SetTickHandler(transfers.Tick)
				advanced := make(chan struct{}, 1)
				for index, w := range f.runtime {
					players, entry := f.players.ForDimension(w.Dimension()), f.worlds[index]
					w.SetPlayerTickHandler(func(uint64) {
						TickPlayerLifecycle(players, entry.Accessor, entry.Config)
						if index == len(f.runtime)-1 {
							select {
							case advanced <- struct{}{}:
							default:
							}
						}
					})
				}
				loop, err := tick.NewLoop(tick.LoopConfig{TargetTPS: 1, CancelPendingTasks: true}, f.manager)
				if err != nil {
					t.Fatal(err)
				}
				if err := loop.Start(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-advanced:
				case <-time.After(5 * time.Second):
					t.Error("world tick did not complete")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := loop.Stop(ctx); err != nil {
					t.Fatal(err)
				}
				if transfers.tick != 1 || online.RespawnRetryTicks != 9 || SessionDimension(s) != dimensions[1-source] {
					t.Fatalf("transfer/lifecycle order: tick=%d retry=%d dimension=%s", transfers.tick, online.RespawnRetryTicks, SessionDimension(s))
				}
				if online.Flags&0x80 != 0 || online.Pose != pose.PoseStanding || online.movement.floatingTicks != 0 || online.movement.glidingTicks != 0 {
					t.Fatal("dimension transfer retained transient movement state")
				}
			})
		}
	}
}

// TestTransferSynchronizesSelectedSlot verifies inventory selection for travel and death respawn.
func TestTransferSynchronizesSelectedSlot(t *testing.T) {
	for _, respawn := range []bool{false, true} {
		t.Run(map[bool]string{false: "travel", true: "respawn"}[respawn], func(t *testing.T) {
			f, transfers := transferFixture(t)
			s := f.sessions[1]
			online := f.players.GetBySession(s)
			online.Windows.SetHeldSlot(7)
			stack := inventory.NewSlot(item.IDByName("minecraft:diamond_pickaxe"), 1)
			stack.SetDamage(9)
			online.Windows.Inventory().Set(inventory.HotbarStart+7, stack)
			var err error
			if respawn {
				player := s.Player().(*entity.Player)
				player.Living().SetHealth(0)
				player.Living().MarkDeathHandled()
				online.DeathProcessed, online.DeathKeepInventory = true, true
				err = transfers.RequestRespawn(s)
			} else {
				err = transfers.Request(s, "minecraft:overworld", entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{})
			}
			if err != nil {
				t.Fatal(err)
			}
			transfers.Tick()
			heldSlot := int32(-1)
			for len(f.conns[1].sent) > 0 {
				packet := <-f.conns[1].sent
				if packet.ID == (&play.ClientboundHeldItemSlot{}).ID() {
					heldSlot, err = protocol.WrapBuffer(packet.Payload).ReadVarInt()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if SessionDimension(s) != "minecraft:overworld" || heldSlot != 7 || online.Windows.HeldSlot() != 7 || online.Windows.HeldItem().Damage != 9 {
				t.Fatal("transition lost the authoritative inventory selection")
			}
		})
	}
}

// TestTravelResendsLivePlayerState synchronizes effects, experience and destination difficulty.
func TestTravelResendsLivePlayerState(t *testing.T) {
	f, transfers := transferFixture(t)
	s := f.sessions[0]
	player := s.Player().(*entity.Player)
	player.Living().SetHealth(13)
	player.Living().SetFoodLevel(11)
	player.Living().SetXP(3, 30, .4)
	active := effect.Instance{ID: effectdata.EffectSpeed, Amplifier: 1, Duration: 1234, Flags: effect.FlagParticles | effect.FlagIcon}
	if !player.Living().Effects().Add(active) {
		t.Fatal("effect was rejected")
	}
	f.runtime[1].SetDifficulty(1)
	packetCount(f.conns[0], -1)
	if err := transfers.Request(s, "minecraft:the_nether", entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}); err != nil {
		t.Fatal(err)
	}
	transfers.Tick()
	respawnSeen, effectSeen, healthSeen, xpSeen, difficultySeen := false, false, false, false, false
	for len(f.conns[0].sent) > 0 {
		packet := <-f.conns[0].sent
		buffer := protocol.WrapBuffer(packet.Payload)
		switch packet.ID {
		case (&play.Respawn{}).ID():
			respawnSeen = true
		case (&play.EntityEffect{}).ID():
			decoded := &play.EntityEffect{}
			if err := decoded.Decode(buffer); err != nil {
				t.Fatal(err)
			}
			effectSeen = respawnSeen && decoded.EntityID == player.ID() && decoded.EffectID == active.ID && decoded.Amplifier == active.Amplifier && decoded.Duration == active.Duration && decoded.Flags == active.Flags
		case (&play.UpdateHealth{}).ID():
			decoded := &play.UpdateHealth{}
			if err := decoded.Decode(buffer); err != nil {
				t.Fatal(err)
			}
			healthSeen = respawnSeen && decoded.Health == 13 && decoded.Food == 11
		case (&play.SetExperience{}).ID():
			decoded := &play.SetExperience{}
			if err := decoded.Decode(buffer); err != nil {
				t.Fatal(err)
			}
			xpSeen = respawnSeen && decoded.ExperienceLevel == 3 && decoded.TotalExperience == 30 && decoded.ExperienceBar == .4
		case (&play.Difficulty{}).ID():
			difficultySeen = respawnSeen && len(packet.Payload) == 2 && packet.Payload[0] == 1
		}
	}
	if !effectSeen || !healthSeen || !xpSeen || !difficultySeen || player.Living().Effects().Count() != 1 {
		t.Fatal("travel did not synchronize live state after the level reset")
	}
}

// findRespawn consumes transmitted packets and decodes the dimension transition.
func findRespawn(t *testing.T, conn *mockConn) *play.Respawn {
	t.Helper()
	var result *play.Respawn
	for {
		select {
		case packet := <-conn.sent:
			if packet.ID == (&play.Respawn{}).ID() {
				buffer := protocol.WrapBuffer(packet.Payload)
				dimensionType, err := buffer.ReadVarInt()
				if err != nil {
					t.Fatal(err)
				}
				dimension, err := buffer.ReadString()
				if err != nil {
					t.Fatal(err)
				}
				result = &play.Respawn{DimensionType: dimensionType, DimensionName: dimension, DataKept: packet.Payload[len(packet.Payload)-1]}
			}
		default:
			return result
		}
	}
}

// TestTransferWaitsForDestinationAndCancelsStalePackets verifies publication and tracking ownership.
func TestTransferWaitsForDestinationAndCancelsStalePackets(t *testing.T) {
	f, transfers := transferFixture(t)
	s := f.sessions[0]
	p := s.Player().(*entity.Player)
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	op := f.players.GetBySession(s)
	op.Windows.Inventory().Set(inventory.HotbarStart, inventory.NewSlot(item.IDByName("minecraft:stone"), 3))
	if err := router.Handle(s, protocol.StatePlay, &play.BlockDig{Status: play.DigDropStack, Sequence: 1}); err != nil {
		t.Fatal(err)
	}
	missing, _ := f.storage[1].Get(1, 1)
	f.storage[1].Delete(1, 1)
	oldBinding := s.world.Load()
	p.TrackedEntities()[777] = struct{}{}
	if err := transfers.Request(s, "minecraft:the_nether", entity.Vec3{X: 9.5, Y: 64, Z: 8.5}, entity.Vec2{X: 90}); err != nil {
		t.Fatal(err)
	}
	if err := transfers.Request(s, "minecraft:the_nether", entity.Vec3{}, entity.Vec2{}); err == nil {
		t.Fatal("duplicate transfer accepted")
	}
	transfers.Tick()
	if s.world.Load() != oldBinding || f.runtime[0].GetEntity(p.ID()) != p.Entity || f.runtime[1].GetEntity(p.ID()) != nil || findRespawn(t, f.conns[0]) != nil {
		t.Fatal("transfer published before destination preparation")
	}
	f.storage[1].Set(missing)
	transfers.Tick()
	if SessionDimension(s) != "minecraft:the_nether" || f.runtime[0].GetEntity(p.ID()) != nil || f.runtime[1].GetEntity(p.ID()) != p.Entity || p.Removed() || len(p.TrackedEntities()) != 0 || len(transfers.pending) != 0 {
		t.Fatal("transition did not move world ownership atomically")
	}
	if len(f.runtime[0].EntityTracker().Players()) != 0 || len(f.runtime[1].EntityTracker().Players()) != 2 || p.HasClientLoaded() || !s.hasPendingTeleport() {
		t.Fatal("tracking or loading state not reset")
	}
	respawn := findRespawn(t, f.conns[0])
	if respawn == nil || respawn.DimensionName != "minecraft:the_nether" || respawn.DataKept != 3 {
		t.Fatal("wrong transfer packet")
	}
	f.runtime[0].Scheduler().Drain(128)
	if f.runtime[0].ItemEntities().Count() != 0 || f.runtime[1].ItemEntities().Count() != 0 || op.Windows.HeldItem().ItemCount != 3 {
		t.Fatal("old queued action survived transfer")
	}
	if err := router.Handle(s, protocol.StatePlay, &play.ConfirmTeleportation{TeleportID: s.pendingTeleport.Load()}); err != nil {
		t.Fatal(err)
	}
	f.runtime[1].Scheduler().Drain(128)
	if s.hasPendingTeleport() {
		t.Fatal("destination teleport acknowledgement was not applied")
	}
}

// TestTransferDisconnectAndTimeoutRetainOrigin verifies cancellation without removal or duplication.
func TestTransferDisconnectAndTimeoutRetainOrigin(t *testing.T) {
	for _, disconnected := range []bool{true, false} {
		t.Run(map[bool]string{true: "disconnect", false: "timeout"}[disconnected], func(t *testing.T) {
			f, transfers := transferFixture(t)
			s := f.sessions[0]
			p := s.Player().(*entity.Player)
			if err := transfers.Request(s, "minecraft:the_nether", entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}); err != nil {
				t.Fatal(err)
			}
			if disconnected {
				f.conns[0].cancel()
			} else {
				transfers.tick = transferTimeoutTicks
			}
			transfers.Tick()
			if len(transfers.pending) != 0 || SessionDimension(s) != "minecraft:overworld" || f.runtime[0].GetEntity(p.ID()) != p.Entity || f.runtime[1].GetEntity(p.ID()) != nil || p.Removed() {
				t.Fatal("cancelled transfer changed membership")
			}
		})
	}
}

// TestTransferTransportFailureRestoresOrigin keeps disconnect snapshots consistent.
func TestTransferTransportFailureRestoresOrigin(t *testing.T) {
	f, transfers := transferFixture(t)
	s := f.sessions[0]
	p := s.Player().(*entity.Player)
	f.connections[0].failPacket.Store((&play.Respawn{}).ID())
	if err := transfers.Request(s, "minecraft:the_nether", entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}); err != nil {
		t.Fatal(err)
	}
	transfers.Tick()
	if SessionDimension(s) != "minecraft:overworld" || f.runtime[0].GetEntity(p.ID()) != p.Entity || f.runtime[1].GetEntity(p.ID()) != nil || p.Removed() {
		t.Fatal("failed transition detached its origin")
	}
}

// TestRespawnPacketReturnsNetherPlayerToOverworld checks the connected death-screen handler.
func TestRespawnPacketReturnsNetherPlayerToOverworld(t *testing.T) {
	f, transfers := transferFixture(t)
	s := f.sessions[1]
	p := s.Player().(*entity.Player)
	online := f.players.GetBySession(s)
	p.Living().SetHealth(0)
	p.Living().MarkDeathHandled()
	online.DeathProcessed = true
	for i := range f.worlds {
		f.worlds[i].Config.RequestRespawn = transfers.RequestRespawn
	}
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(s, protocol.StatePlay, &play.ClientCommand{ActionID: 0}); err != nil {
		t.Fatal(err)
	}
	if len(transfers.pending) != 0 {
		t.Fatal("respawn mutated state outside tick")
	}
	f.runtime[1].Scheduler().Drain(128)
	transfers.Tick()
	if p.Living().IsDead() || SessionDimension(s) != "minecraft:overworld" || online.DeathProcessed || online.RespawnPending || f.runtime[0].GetEntity(p.ID()) != p.Entity {
		t.Fatal("death-screen respawn did not return to Overworld")
	}
	if respawn := findRespawn(t, f.conns[1]); respawn == nil || respawn.DimensionName != "minecraft:overworld" || respawn.DataKept != 0 {
		t.Fatal("wrong respawn packet")
	}
}

// TestRespawnUsesPreparedForcedDimensionSpawn preserves an explicit spawn and XP.
func TestRespawnUsesPreparedForcedDimensionSpawn(t *testing.T) {
	f, transfers := transferFixture(t)
	s := f.sessions[0]
	p := s.Player().(*entity.Player)
	online := f.players.GetBySession(s)
	online.HasSpawnPoint, online.SpawnForced, online.DeathProcessed, online.DeathKeepInventory = true, true, true, true
	online.SpawnDimension = "minecraft:the_nether"
	online.SpawnX, online.SpawnY, online.SpawnZ = 10, 64, 10
	p.Living().SetXP(4, 40, .5)
	p.Living().SetHealth(0)
	p.Living().MarkDeathHandled()
	if err := transfers.RequestRespawn(s); err != nil {
		t.Fatal(err)
	}
	transfers.Tick()
	if SessionDimension(s) != "minecraft:the_nether" || p.Position().X != 10.5 || p.Living().XPLevel() != 4 || !online.HasSpawnPoint {
		t.Fatal("explicit respawn lost dimension or XP")
	}
}

// TestTransferPreparationQueueIsBounded rejects excess requests without losing admitted players.
func TestTransferPreparationQueueIsBounded(t *testing.T) {
	f, transfers := transferFixture(t)
	for index := 0; index <= maxPreparingTransfers; index++ {
		conn := newMockConn()
		conn.sent = make(chan network.Packet, 2048)
		s, err := New(Config{ID: uint64(index + 100), Connection: conn, Registry: f.sessions[0].registry, InitialVersion: protocol.CurrentVersion, InitialState: protocol.StatePlay})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		s.BindWorldDimension("minecraft:overworld")
		p := entity.NewPlayer(f.runtime[0].EntityManager().AllocateID(), protocol.UUID{99, uint64(index + 1)}, "queued", entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}, s, 2)
		s.BindPlayer(p)
		f.runtime[0].AddPlayer(p)
		if err := f.players.AddPlayer(&OnlinePlayer{Session: s, UUID: p.UUID(), EntityID: p.ID(), ManagedTracking: true}); err != nil {
			t.Fatal(err)
		}
		err = transfers.Request(s, "minecraft:the_nether", p.Position(), p.Rotation())
		if index < maxPreparingTransfers && err != nil {
			t.Fatal(err)
		}
		if index == maxPreparingTransfers && (err == nil || f.runtime[0].GetEntity(p.ID()) != p.Entity) {
			t.Fatal("overflow accepted or removed an origin player")
		}
	}
	if len(transfers.pending) != maxPreparingTransfers {
		t.Fatal("preparation queue exceeded its bound")
	}
}
