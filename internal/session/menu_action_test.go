package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/datapack"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// menuTestWorld supplies a resident source block without loading terrain.
type menuTestWorld struct {
	*DefaultWorldAccessor
	state int32
}

// GetBlock reads the source block used by the temporary menu.
func (w *menuTestWorld) GetBlock(x, y, z int) int32 {
	if x == 1 && y == 64 && z == 0 {
		return w.state
	}
	return 0
}

// stonecuttingSession creates a real packet router, resource runtime and opened menu.
func stonecuttingSession(t *testing.T) (*entity.Player, *OnlinePlayer, *PlayerManager, *menuTestWorld, PacketRouter, *crafting.Runtime) {
	t.Helper()
	pack := t.TempDir()
	if err := os.WriteFile(filepath.Join(pack, "pack.mcmeta"), []byte(`{"pack":{"description":"menu","pack_format":61}}`), 0600); err != nil {
		t.Fatal(err)
	}
	recipes := filepath.Join(pack, "data", "test", "recipe")
	if err := os.MkdirAll(recipes, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipes, "slabs.json"), []byte(`{"type":"minecraft:stonecutting","ingredient":"minecraft:stone","result":{"id":"minecraft:stone_slab","count":2}}`), 0600); err != nil {
		t.Fatal(err)
	}
	resources := &datapack.Manager{}
	runtime, err := crafting.NewRuntime(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Reload([]string{pack}); err != nil {
		t.Fatal(err)
	}
	player, online, movement := movementTestPlayer(t)
	online.Windows = inventory.NewWindowManager()
	online.Windows.SetStonecuttingMatcher(runtime.StonecuttingChoices)
	online.RecipeCatalog = runtime.Catalog()
	world := &menuTestWorld{DefaultWorldAccessor: &DefaultWorldAccessor{}, state: block.DefaultStateID("minecraft:stonecutter")}
	router := NewPacketRouter()
	if err := RegisterPlayHandlers(router, PlayBootstrapConfig{Crafting: runtime}, movement.players, world); err != nil {
		t.Fatal(err)
	}
	(&openBlockWindowCommand{session: online.Session, players: movement.players, world: world, x: 1, y: 64, z: 0, sequence: 1}).Execute()
	active := online.Windows.ActiveWindow()
	if active == nil || active.Type != inventory.WindowTypeStonecutter || active.OwnedSlots != 2 || active.Container.Size() != 38 {
		t.Fatalf("menu = %+v", active)
	}
	drainMenuPackets(online.Session.(*DefaultSession).conn.(*mockConn))
	return player, online, movement.players, world, router, runtime
}

// drainMenuPackets empties the bounded mock network after a verified transaction.
func drainMenuPackets(conn *mockConn) {
	for {
		select {
		case <-conn.sent:
		default:
			return
		}
	}
}

// TestStonecutterRuntimeTransaction verifies packet requests, tick ownership and client contents.
func TestStonecutterRuntimeTransaction(t *testing.T) {
	_, online, _, world, router, _ := stonecuttingSession(t)
	var queued func()
	world.SubmitTask = func(task func()) bool { queued = task; return true }
	window := online.Windows.ActiveWindow()
	window.Backing.Set(0, inventory.NewSlot(item.IDByName("minecraft:stone"), 3))
	selection := &play.EnchantItem{WindowID: window.ID, Enchantment: 0}
	if err := router.Handle(online.Session, protocol.StatePlay, selection); err != nil {
		t.Fatal(err)
	}
	if queued == nil || !window.Backing.Get(1).Empty() {
		t.Fatal("selection did not wait for world tick")
	}
	queued()
	conn := online.Session.(*DefaultSession).conn.(*mockConn)
	content := <-conn.sent
	var decoded play.SetContainerContent
	var err error
	if content.ID != decoded.ID() {
		t.Fatalf("packet ID = %d", content.ID)
	}
	buffer := protocol.WrapBuffer(content.Payload)
	if decoded.WindowID, err = buffer.ReadVarInt(); err != nil {
		t.Fatal(err)
	}
	if decoded.StateID, err = buffer.ReadVarInt(); err != nil {
		t.Fatal(err)
	}
	count, err := buffer.ReadVarInt()
	if err != nil || count != 38 {
		t.Fatalf("slot count = %d, error = %v", count, err)
	}
	decoded.Slots = make([]inventory.Slot, count)
	for i := range decoded.Slots {
		decoded.Slots[i], err = inventory.DecodeItemStack(buffer)
		if err != nil {
			t.Fatal(err)
		}
	}
	if decoded.Carried, err = inventory.DecodeItemStack(buffer); err != nil || buffer.Remaining() != 0 {
		t.Fatalf("carried decode: %v", err)
	}
	if len(decoded.Slots) != 38 || decoded.Slots[1].ItemID != item.IDByName("minecraft:stone_slab") || decoded.Slots[1].ItemCount != 2 || decoded.Slots[0].ItemCount != 3 {
		t.Fatalf("client preview incorrect: slots=%+v, server result=%+v", decoded.Slots, window.Backing.Get(1))
	}
	property := <-conn.sent
	var selected play.SetContainerProperty
	if property.ID != selected.ID() {
		t.Fatal("selected recipe property missing")
	}
	if err := selected.Decode(protocol.WrapBuffer(property.Payload)); err != nil || selected.Value != 0 {
		t.Fatalf("selected = %+v, error = %v", selected, err)
	}
	click := &play.WindowClick{WindowID: window.ID, StateID: online.Windows.CurrentStateID(), Slot: 1, Button: 0, Mode: inventory.ClickTypeNormalClick}
	if err := router.Handle(online.Session, protocol.StatePlay, click); err != nil {
		t.Fatal(err)
	}
	if !online.Windows.CursorItem().Empty() || window.Backing.Get(0).ItemCount != 3 {
		t.Fatal("take changed state before tick")
	}
	queued()
	if online.Windows.CursorItem().ItemCount != 2 || window.Backing.Get(0).ItemCount != 2 {
		t.Fatal("take did not consume exactly one input")
	}
	drainMenuPackets(conn)
	if err := router.Handle(online.Session, protocol.StatePlay, &play.CloseWindow{WindowID: window.ID}); err != nil {
		t.Fatal(err)
	}
	queued()
	stone, slabs := int32(0), int32(0)
	for _, stack := range online.Windows.Inventory().Slots() {
		if stack.ItemID == item.IDByName("minecraft:stone") {
			stone += stack.ItemCount
		}
		if stack.ItemID == item.IDByName("minecraft:stone_slab") {
			slabs += stack.ItemCount
		}
	}
	if online.Windows.ActiveWindow() != nil || stone != 2 || slabs != 2 {
		t.Fatalf("close inventory = %d stone, %d slabs", stone, slabs)
	}
}

// TestStonecutterBackpressureAndStaleWindowPreserveInput verifies rejected requests cannot mutate menus.
func TestStonecutterBackpressureAndStaleWindowPreserveInput(t *testing.T) {
	_, online, players, world, router, _ := stonecuttingSession(t)
	window := online.Windows.ActiveWindow()
	window.Backing.Set(0, inventory.NewSlot(item.IDByName("minecraft:stone"), 3))
	state := online.Windows.CurrentStateID()
	world.SubmitTask = func(func()) bool { return false }
	if err := router.Handle(online.Session, protocol.StatePlay, &play.EnchantItem{WindowID: window.ID, Enchantment: 0}); err != nil {
		t.Fatal(err)
	}
	if !window.Backing.Get(1).Empty() || online.Windows.CurrentStateID() != state || window.Backing.Get(0).ItemCount != 3 {
		t.Fatal("overloaded request changed menu")
	}
	(&menuButtonCommand{session: online.Session, players: players, world: world, windowID: window.ID + 1, button: 0}).Execute()
	if !window.Backing.Get(1).Empty() {
		t.Fatal("stale window selected a recipe")
	}
	var queued func()
	world.SubmitTask = func(task func()) bool { queued = task; return true }
	if err := router.Handle(online.Session, protocol.StatePlay, &play.EnchantItem{WindowID: window.ID, Enchantment: 0}); err != nil {
		t.Fatal(err)
	}
	online.Windows.CloseWindowWithDrops()
	queued()
	if !online.Windows.CursorItem().Empty() {
		t.Fatal("closed menu produced output")
	}
}

// TestStonecutterSourceInvalidationReturnsInputs verifies block removal and reach close on tick.
func TestStonecutterSourceInvalidationReturnsInputs(t *testing.T) {
	for _, reason := range []string{"removed", "distance", "spectator"} {
		t.Run(reason, func(t *testing.T) {
			player, online, players, world, _, _ := stonecuttingSession(t)
			window := online.Windows.ActiveWindow()
			window.Backing.Set(0, inventory.NewSlot(item.IDByName("minecraft:stone"), 3))
			online.Windows.SelectStonecuttingRecipe(window.ID, 0)
			switch reason {
			case "removed":
				world.state = 0
			case "distance":
				player.SetPosition(entity.Vec3{X: 30, Y: 64})
			case "spectator":
				player.SetGameMode(3)
			}
			TickPlayerMenus(players, world)
			if online.Windows.ActiveWindow() != nil || online.Windows.Inventory().Get(inventory.MainInventoryStart).ItemCount != 3 {
				t.Fatal("invalid source retained or lost menu inputs")
			}
			if !window.Backing.Get(1).Empty() {
				t.Fatal("preview persisted after close")
			}
			conn := online.Session.(*DefaultSession).conn.(*mockConn)
			<-conn.sent
			if packet := <-conn.sent; packet.ID != (&play.ClientboundCloseContainer{}).ID() {
				t.Fatal("clientbound close packet missing")
			}
		})
	}
}

// TestStonecutterOffhandTakeSynchronizesPlayerInventory verifies an output swap outside menu slots.
func TestStonecutterOffhandTakeSynchronizesPlayerInventory(t *testing.T) {
	_, online, players, world, _, _ := stonecuttingSession(t)
	window := online.Windows.ActiveWindow()
	window.Backing.Set(0, inventory.NewSlot(item.IDByName("minecraft:stone"), 2))
	online.Windows.SelectStonecuttingRecipe(window.ID, 0)
	(&windowClickCommand{session: online.Session, players: players, world: world, packet: &play.WindowClick{WindowID: window.ID, StateID: online.Windows.CurrentStateID(), Slot: 1, Mode: inventory.ClickTypeNumberKey, Button: 40}}).Execute()
	if online.Windows.Inventory().Get(inventory.OffhandSlot).ItemCount != 2 || window.Backing.Get(0).ItemCount != 1 {
		t.Fatal("offhand result or cost incorrect")
	}
	conn := online.Session.(*DefaultSession).conn.(*mockConn)
	<-conn.sent
	<-conn.sent
	packet := <-conn.sent
	windowID, err := protocol.WrapBuffer(packet.Payload).ReadVarInt()
	if err != nil || packet.ID != (&play.SetContainerContent{}).ID() || windowID != 0 {
		t.Fatal("offhand player inventory synchronization missing")
	}
}
