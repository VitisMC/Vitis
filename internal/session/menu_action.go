package session

import (
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// menuButtonCommand validates a recipe selection against the current menu on tick.
type menuButtonCommand struct {
	session          Session
	players          *PlayerManager
	world            WorldAccessor
	runtime          *crafting.Runtime
	windowID, button int32
}

// OwnerSession binds the request to its originating connection and dimension.
func (c *menuButtonCommand) OwnerSession() Session { return c.session }

// Execute applies only valid stonecutter selections and synchronizes menu state.
func (c *menuButtonCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil || c.players == nil {
		return
	}
	online := c.players.GetBySession(c.session)
	if online == nil || online.Windows == nil {
		return
	}
	active := online.Windows.ActiveWindow()
	if active == nil || active.ID != c.windowID || active.Type != inventory.WindowTypeStonecutter {
		return
	}
	if !stonecuttingMenuValid(c.session, c.world, active) {
		closeInvalidMenu(c.session, c.players, c.world, active)
		return
	}
	if c.runtime != nil && online.RecipeCatalog != c.runtime.Catalog() {
		return
	}
	online.Windows.SelectStonecuttingRecipe(c.windowID, c.button)
	if err := sendWindowResync(c.session, online.Windows, c.windowID); err != nil {
		logger.Warn("sync stonecutter contents failed", "session", c.session.ID(), "error", err)
		return
	}
	if err := sendStonecuttingSelection(c.session, online.Windows, c.windowID); err != nil {
		logger.Warn("sync stonecutter selection failed", "session", c.session.ID(), "error", err)
	}
}

// registerMenuButtonHandler queues menu buttons without trusting client results.
func registerMenuButtonHandler(router PacketRouter, players *PlayerManager, world WorldAccessor, runtime *crafting.Runtime) error {
	return router.Register(protocol.StatePlay, play.NewEnchantItem().ID(), func(s Session, packet protocol.Packet) error {
		request, ok := packet.(*play.EnchantItem)
		if !ok {
			return protocol.ErrNilPacket
		}
		if players == nil || request.WindowID <= 0 || request.Enchantment < 0 {
			return nil
		}
		if world == nil || !world.Submit(&menuButtonCommand{session: s, players: players, world: world, runtime: runtime, windowID: request.WindowID, button: request.Enchantment}) {
			if online := players.GetBySession(s); online != nil && online.Windows != nil {
				return sendWindowSnapshot(s, online.Windows, request.WindowID)
			}
		}
		return nil
	})
}

// stonecuttingMenuValid checks current player state, reach and the source block.
func stonecuttingMenuValid(s Session, world WorldAccessor, window *inventory.Window) bool {
	player, ok := s.Player().(*entity.Player)
	if !ok || player == nil || player.IsDead() || player.GameMode() == 3 || world == nil {
		return false
	}
	position := play.BlockPos{X: int32(window.BlockX), Y: int32(window.BlockY), Z: int32(window.BlockZ)}
	return blockWithinReach(player, position, 8) && block.NameFromState(world.GetBlock(window.BlockX, window.BlockY, window.BlockZ)) == "minecraft:stonecutter"
}

// sendStonecuttingSelection synchronizes the selected recipe property.
func sendStonecuttingSelection(s Session, windows *inventory.WindowManager, windowID int32) error {
	return s.Send(&play.SetContainerProperty{WindowID: windowID, Property: 0, Value: windows.SelectedStonecuttingRecipe()})
}

// closeInvalidMenu returns inputs before telling the client that its menu closed.
func closeInvalidMenu(s Session, players *PlayerManager, world WorldAccessor, window *inventory.Window) {
	(&closeWindowCommand{session: s, players: players, world: world, windowID: window.ID}).Execute()
	if err := s.Send(&play.ClientboundCloseContainer{WindowID: window.ID}); err != nil {
		logger.Warn("close invalid menu failed", "session", s.ID(), "error", err)
	}
}

// TickPlayerMenus closes temporary menus when their source block or reach is invalid.
func TickPlayerMenus(players *PlayerManager, world WorldAccessor) {
	if players == nil || world == nil {
		return
	}
	for _, online := range players.Players() {
		if online.Session == nil || online.Session.Context().Err() != nil || online.Windows == nil {
			continue
		}
		active := online.Windows.ActiveWindow()
		if active != nil && active.Type == inventory.WindowTypeStonecutter && !stonecuttingMenuValid(online.Session, world, active) {
			closeInvalidMenu(online.Session, players, world, active)
		}
	}
}
