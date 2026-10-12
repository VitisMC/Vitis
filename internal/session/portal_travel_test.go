package session

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"testing"
)

// installTestPortal creates a loaded two-by-three frame in an isolated fixture.
func installTestPortal(world WorldAccessor, x, y, z int, axis string) {
	dx, dz := 1, 0
	if axis == "z" {
		dx, dz = 0, 1
	}
	for along := -1; along <= 2; along++ {
		for up := -1; up <= 3; up++ {
			state := block.DefaultStateID("minecraft:obsidian")
			if along >= 0 && along < 2 && up >= 0 && up < 3 {
				state = block.StateID("minecraft:nether_portal", map[string]string{"axis": axis})
			}
			world.SetBlock(x+dx*along, y+up, z+dz*along, state)
		}
	}
}

// TestNetherPortalContactUsesDelayScaleGeometryAndCooldown covers automatic player travel.
func TestNetherPortalContactUsesDelayScaleGeometryAndCooldown(t *testing.T) {
	f := newDimensionFixture(t)
	f.worlds[0].Config.GameRule = func(name string) string {
		if name == "playersNetherPortalDefaultDelay" {
			return "2"
		}
		return ""
	}
	installTestPortal(f.worlds[0].Accessor, 8, 64, 8, "x")
	installTestPortal(f.worlds[1].Accessor, 1, 64, 2, "z")
	transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
	if err != nil {
		t.Fatal(err)
	}
	s := f.sessions[0]
	p := s.Player().(*entity.Player)
	transfers.Tick()
	transfers.Tick()
	if len(transfers.pending) != 0 || SessionDimension(s) != "minecraft:overworld" {
		t.Fatal("portal ignored the configured wait")
	}
	for i := 0; i < 4 && SessionDimension(s) == "minecraft:overworld"; i++ {
		transfers.Tick()
	}
	if SessionDimension(s) != "minecraft:the_nether" || p.Position().X != 1.5 || p.Position().Z != 2.5 || p.Rotation().X != 90 {
		t.Fatalf("portal exit = %s %+v %+v", SessionDimension(s), p.Position(), p.Rotation())
	}
	online := f.players.GetBySession(s)
	if online.PortalCooldown != 300 {
		t.Fatal("portal cooldown was not installed")
	}
	if err := s.acknowledgeTeleport(s.pendingTeleport.Load()); err != nil {
		t.Fatal(err)
	}
	p.SetClientLoaded(true)
	transfers.Tick()
	if online.PortalCooldown != 300 || SessionDimension(s) != "minecraft:the_nether" || len(transfers.pending) != 0 {
		t.Fatal("standing in exit portal retriggered travel")
	}
	p.SetPosition(entity.Vec3{X: 8.5, Y: 64, Z: 8.5})
	transfers.Tick()
	if online.PortalCooldown != 299 {
		t.Fatal("leaving portal did not advance cooldown")
	}
}

// TestNetherPortalCreatesPreparedDestination checks bounded creation and observer updates.
func TestNetherPortalCreatesPreparedDestination(t *testing.T) {
	f := newDimensionFixture(t)
	f.worlds[0].Config.GameRule = func(string) string { return "0" }
	installTestPortal(f.worlds[0].Accessor, 8, 64, 8, "x")
	transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
	if err != nil {
		t.Fatal(err)
	}
	s := f.sessions[0]
	for i := 0; i < 150 && SessionDimension(s) == "minecraft:overworld"; i++ {
		transfers.Tick()
	}
	if SessionDimension(s) != "minecraft:the_nether" {
		pending := transfers.pending[s.Player().(*entity.Player).UUID()]
		if pending != nil {
			t.Fatalf("portal still pending: position=%+v job=%+v creation=%+v", pending.position, pending.portal, pending.portal.creation)
		}
		for len(f.conns[0].sent) > 0 {
			packet := <-f.conns[0].sent
			if packet.ID == (&play.SystemChatMessage{}).ID() {
				t.Logf("portal message: %q", packet.Payload)
			}
		}
		t.Fatalf("prepared portal was not created: lifecycle=%v cooldown=%d", s.Lifecycle(), f.players.GetBySession(s).PortalCooldown)
	}
	p := s.Player().(*entity.Player)
	kind, _ := portalContact(f.worlds[1].Accessor, p.Position())
	if kind != "minecraft:nether_portal" {
		t.Fatal("new portal exit missed its interior")
	}
	if packetCount(f.conns[1], (&play.BlockUpdate{}).ID()) == 0 {
		t.Fatal("destination observer did not receive portal blocks")
	}
}

// TestEndPortalPreparesPlatformAndReturnsLivingPlayer preserves health and experience.
func TestEndPortalPreparesPlatformAndReturnsLivingPlayer(t *testing.T) {
	f := newDimensionFixtureWith(t, []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"})
	for x := int32(4); x <= 8; x++ {
		for z := int32(-2); z <= 2; z++ {
			f.storage[2].Set(chunk.New(x, z))
		}
	}
	s := f.sessions[0]
	p := s.Player().(*entity.Player)
	p.SetPosition(entity.Vec3{X: 8.5, Y: 63.5, Z: 8.5})
	f.players.GetBySession(s).Y = 63.5
	p.Living().SetHealth(13)
	p.Living().SetXP(3, 30, .4)
	f.worlds[0].Accessor.SetBlock(8, 63, 8, block.DefaultStateID("minecraft:end_portal"))
	transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
	if err != nil {
		t.Fatal(err)
	}
	transfers.Tick()
	if SessionDimension(s) != "minecraft:the_end" || p.Position() != (entity.Vec3{X: 100.5, Y: 49, Z: .5}) {
		t.Fatal("End entry did not prepare the service platform")
	}
	for x := 98; x <= 102; x++ {
		for z := -2; z <= 2; z++ {
			if block.NameFromState(f.worlds[2].Accessor.GetBlock(x, 48, z)) != "minecraft:obsidian" {
				t.Fatal("End platform floor is missing")
			}
			for y := 49; y <= 51; y++ {
				if !block.IsAir(f.worlds[2].Accessor.GetBlock(x, y, z)) {
					t.Fatal("End platform clearance is obstructed")
				}
			}
		}
	}
	if err := s.acknowledgeTeleport(s.pendingTeleport.Load()); err != nil {
		t.Fatal(err)
	}
	p.SetClientLoaded(true)
	online := f.players.GetBySession(s)
	online.PortalCooldown = 0
	p.SetPosition(entity.Vec3{X: 100.5, Y: 48.5, Z: .5})
	online.X, online.Y, online.Z = 100.5, 48.5, .5
	f.worlds[2].Accessor.SetBlock(100, 48, 0, block.DefaultStateID("minecraft:end_portal"))
	transfers.Tick()
	if SessionDimension(s) != "minecraft:overworld" || p.Living().Health() != 13 || p.Living().XPLevel() != 3 {
		t.Fatal("living End return applied death respawn resets")
	}
}

// TestPortalContactShapeAndDelayDecay rejects distant contacts and preserves partial wait decay.
func TestPortalContactShapeAndDelayDecay(t *testing.T) {
	f := newDimensionFixture(t)
	installTestPortal(f.worlds[0].Accessor, 8, 64, 8, "x")
	if kind, _ := portalContact(f.worlds[0].Accessor, entity.Vec3{X: 8.5, Y: 64, Z: 7.9}); kind != "" {
		t.Fatal("portal triggered outside its thin plane")
	}
	transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		transfers.Tick()
	}
	online := f.players.GetBySession(f.sessions[0])
	if online.portalTime != 8 {
		t.Fatal("portal timer did not advance")
	}
	f.sessions[0].Player().(*entity.Player).SetPosition(entity.Vec3{X: 8.5, Y: 64, Z: 10.5})
	transfers.Tick()
	if online.portalTime != 4 {
		t.Fatal("portal timer did not decay")
	}
}

// TestTransferBlocksActionsUntilTeleportConfirmation protects the prepared destination inventory.
func TestTransferBlocksActionsUntilTeleportConfirmation(t *testing.T) {
	f, transfers := transferFixture(t)
	s := f.sessions[0]
	if err := transfers.Request(s, "minecraft:the_nether", entity.Vec3{X: 8.5, Y: 64, Z: 8.5}, entity.Vec2{}); err != nil {
		t.Fatal(err)
	}
	transfers.Tick()
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(s, protocol.StatePlay, &play.ServerboundHeldItemSlot{Slot: 1}); err != nil {
		t.Fatal(err)
	}
	f.runtime[1].Scheduler().Drain(128)
	if f.players.GetBySession(s).Windows.HeldSlot() != 0 {
		t.Fatal("pending teleport accepted inventory mutation")
	}
}

// TestEndPlatformClearsContainerWithoutLosingComponents verifies destruction during prepared entry.
func TestEndPlatformClearsContainerWithoutLosingComponents(t *testing.T) {
	f := newDimensionFixtureWith(t, []string{"minecraft:overworld", "minecraft:the_end"})
	for x := int32(4); x <= 8; x++ {
		for z := int32(-2); z <= 2; z++ {
			f.storage[1].Set(chunk.New(x, z))
		}
	}
	destination := f.worlds[1].Accessor
	destination.SetBlock(100, 50, 0, block.DefaultStateID("minecraft:chest"))
	container := destination.BlockEntities().Container(100, 50, 0, "minecraft:chest", 27)
	if container == nil {
		t.Fatal("destination container unavailable")
	}
	stack := inventory.NewSlot(item.IDByName("minecraft:diamond_pickaxe"), 1)
	stack.SetDamage(7)
	container.Set(0, stack)
	s := f.sessions[0]
	p := s.Player().(*entity.Player)
	p.SetPosition(entity.Vec3{X: 8.5, Y: 63.5, Z: 8.5})
	f.worlds[0].Accessor.SetBlock(8, 63, 8, block.DefaultStateID("minecraft:end_portal"))
	transfers, err := NewDimensionTransfers(f.players, f.manager, f.worlds)
	if err != nil {
		t.Fatal(err)
	}
	transfers.Tick()
	c, _ := destination.GetChunk(6, 0)
	if c.Data().GetBlockEntity(100, 50, 0) != nil || SessionDimension(s) != "minecraft:the_end" {
		t.Fatal("platform retained an obsolete block entity")
	}
	found := 0
	for _, base := range f.runtime[1].EntityManager().Entities() {
		if dropped := f.runtime[1].ItemEntities().Get(base.ID()); dropped != nil && dropped.Stack().ItemID == stack.ItemID {
			if dropped.Stack().Damage != 7 {
				t.Fatal("platform drop lost components")
			}
			found++
		}
	}
	if found != 1 {
		t.Fatal("platform lost or duplicated container contents")
	}
}

// TestForbiddenDimensionBedUsesQueuedExplosionAndDoesNotSetSpawn checks the connected use handler.
func TestForbiddenDimensionBedUsesQueuedExplosionAndDoesNotSetSpawn(t *testing.T) {
	f := newDimensionFixture(t)
	accessor := f.worlds[1].Accessor.(*DefaultWorldAccessor)
	exploded := false
	accessor.CreateBedExplosion = func(x, y, z int) bool { exploded = true; accessor.SetBlock(x, y, z, 0); return true }
	accessor.SetBlock(8, 64, 8, block.StateID("minecraft:red_bed", map[string]string{"facing": "north", "part": "head", "occupied": "false"}))
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(f.sessions[1], protocol.StatePlay, &play.UseItemOn{Position: play.BlockPos{X: 8, Y: 64, Z: 8}, Face: 1, Sequence: 1}); err != nil {
		t.Fatal(err)
	}
	if exploded {
		t.Fatal("bed interaction ran outside tick")
	}
	f.runtime[1].Scheduler().Drain(128)
	if !exploded || f.players.GetBySession(f.sessions[1]).HasSpawnPoint {
		t.Fatal("Nether bed became a respawn point")
	}
}

// TestDimensionMalformedActionClosesOnTick verifies queued packet validation before mutation.
func TestDimensionMalformedActionClosesOnTick(t *testing.T) {
	f := newDimensionFixture(t)
	router := NewPacketRouter()
	if err := RegisterDimensionPlayHandlers(router, f.players, f.worlds); err != nil {
		t.Fatal(err)
	}
	if err := router.Handle(f.sessions[0], protocol.StatePlay, &play.UseItemOn{Hand: 3, Face: 99}); err != nil {
		t.Fatal(err)
	}
	f.runtime[0].Scheduler().Drain(128)
	if f.sessions[0].Lifecycle() == LifecycleActive {
		t.Fatal("malformed action was not rejected")
	}
}
