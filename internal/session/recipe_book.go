package session

import (
	"math"

	"github.com/vitismc/vitis/internal/advancement"
	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// TickRecipeBooks updates resource bindings and recipe advancement triggers.
func TickRecipeBooks(players *PlayerManager, world WorldAccessor, config PlayBootstrapConfig) {
	if players == nil || config.Crafting == nil {
		return
	}
	catalog := config.Crafting.Catalog()
	if catalog == nil {
		return
	}
	for _, online := range players.Players() {
		if online.Session == nil || online.Session.Context().Err() != nil || online.Windows == nil || online.RecipeBook == nil {
			continue
		}
		player, ok := online.Session.Player().(*entity.Player)
		if !ok || player == nil || player.Removed() {
			continue
		}
		changed := online.RecipeBook.Bind(catalog)
		if changed {
			online.RecipeCatalog = catalog
			declaration, err := config.Crafting.DeclarePacket()
			if err == nil {
				err = online.Session.Send(declaration)
			}
			if err != nil {
				logger.Warn("send recipe properties failed", "player", online.Name, "error", err)
			}
			settings := &play.RecipeBookSettings{}
			values := []*bool{&settings.CraftingOpen, &settings.CraftingFilter, &settings.SmeltingOpen, &settings.SmeltingFilter, &settings.BlastFurnaceOpen, &settings.BlastFurnaceFilter, &settings.SmokerOpen, &settings.SmokerFilter}
			for i := crafting.BookType(0); i <= crafting.BookSmoker; i++ {
				state, _ := online.RecipeBook.Settings(i)
				*values[2*int(i)], *values[2*int(i)+1] = state.Open, state.Filtering
			}
			if err := online.Session.Send(settings); err != nil {
				logger.Warn("send recipe settings failed", "player", online.Name, "error", err)
			}
		}
		var newlyUnlocked []int32
		grant := func(event advancement.RecipeEvent) {
			if config.RecipeAdvancements == nil {
				return
			}
			recipes := config.RecipeAdvancements.ApplyRecipeEvent(&online.RecipeProgress, event)
			newlyUnlocked = append(newlyUnlocked, online.RecipeBook.UnlockRecipes(recipes...)...)
		}
		grant(advancement.RecipeEvent{Trigger: "minecraft:tick"})
		state, slots, _ := online.Windows.Snapshot(0)
		if changed || state != online.RecipeInventoryState {
			online.RecipeInventoryState = state
			stacks := make([]inventory.ItemStack, 0, 41)
			for i := 5; i < len(slots); i++ {
				stacks = append(stacks, slots[i])
			}
			grant(advancement.RecipeEvent{Trigger: "minecraft:inventory_changed", Inventory: stacks})
		}
		if world != nil {
			p := player.Position()
			name := block.NameFromState(world.GetBlock(int(math.Floor(p.X)), int(math.Floor(p.Y)), int(math.Floor(p.Z))))
			if name != online.RecipeEnteredBlock {
				online.RecipeEnteredBlock = name
				grant(advancement.RecipeEvent{Trigger: "minecraft:enter_block", Block: name})
			}
		}
		for iteration := 0; iteration < 16; iteration++ {
			before := len(newlyUnlocked)
			grant(advancement.RecipeEvent{Trigger: "minecraft:recipe_unlocked", KnowsRecipe: online.RecipeBook.KnowsRecipe})
			if len(newlyUnlocked) == before {
				break
			}
		}
		ids := newlyUnlocked
		if changed {
			ids = online.RecipeBook.Known()
		}
		if changed || len(ids) > 0 {
			packet := &play.RecipeBookAdd{Replace: changed}
			for _, id := range ids {
				if definition, ok := catalog.Display(id); ok {
					entry := definition.Entry
					if online.RecipeBook.Highlighted(id) {
						entry.Flags = 3
					}
					packet.Recipes = append(packet.Recipes, entry)
				}
			}
			if err := online.Session.Send(packet); err != nil {
				logger.Warn("send recipe book failed", "player", online.Name, "error", err)
			}
			online.Windows.UpdateCraftOutput()
			if active := online.Windows.ActiveWindow(); changed && active != nil && active.Type == inventory.WindowTypeStonecutter {
				if err := sendWindowResync(online.Session, online.Windows, active.ID); err != nil {
					logger.Warn("reload stonecutter contents failed", "player", online.Name, "error", err)
				}
				if err := sendStonecuttingSelection(online.Session, online.Windows, active.ID); err != nil {
					logger.Warn("reload stonecutter selection failed", "player", online.Name, "error", err)
				}
			}
		}
		online.Windows.TakeCraftedRecipes()
	}
}

// recipePlacementCommand validates ownership and applies a grid placement on tick.
type recipePlacementCommand struct {
	session Session
	players *PlayerManager
	runtime *crafting.Runtime
	request play.CraftRecipeRequest
}

// Execute resolves only current, unlocked displays in the active crafting window.
func (c *recipePlacementCommand) Execute() {
	if c.session == nil || c.session.Context().Err() != nil {
		return
	}
	if c.players == nil || c.runtime == nil {
		return
	}
	online := c.players.GetBySession(c.session)
	if online == nil || online.Windows == nil || online.RecipeBook == nil || online.GameMode == 3 || online.RecipeCatalog != c.runtime.Catalog() || !online.RecipeBook.Knows(c.request.RecipeID) {
		return
	}
	player, ok := c.session.Player().(*entity.Player)
	if !ok || player == nil || player.Living().IsDead() {
		return
	}
	definition, ok := online.RecipeCatalog.Display(c.request.RecipeID)
	if !ok {
		return
	}
	width := 2
	if c.request.WindowID != 0 {
		active := online.Windows.ActiveWindow()
		if active == nil || active.ID != c.request.WindowID || active.Type != inventory.WindowTypeCrafting {
			return
		}
		width = 3
	}
	requirements, ok := definition.Placement(width)
	if !ok {
		return
	}
	if !online.Windows.PlaceRecipe(c.request.WindowID, requirements, c.request.MakeAll) {
		if err := c.session.Send(&play.CraftRecipeResponse{WindowID: c.request.WindowID, Display: definition.Entry.Display}); err != nil {
			logger.Warn("send ghost recipe failed", "error", err)
		}
	}
	if err := sendWindowResync(c.session, online.Windows, c.request.WindowID); err != nil {
		logger.Warn("resync recipe placement failed", "error", err)
	}
}

// registerRecipePlacementHandler routes client requests to the authoritative tick.
func registerRecipePlacementHandler(router PacketRouter, players *PlayerManager, world WorldAccessor, config PlayBootstrapConfig) error {
	return router.Register(protocol.StatePlay, play.NewCraftRecipeRequest().ID(), func(s Session, packet protocol.Packet) error {
		request, ok := packet.(*play.CraftRecipeRequest)
		if !ok {
			return protocol.ErrNilPacket
		}
		if world == nil {
			return nil
		}
		if !world.Submit(&recipePlacementCommand{session: s, players: players, runtime: config.Crafting, request: *request}) {
			if online := players.GetBySession(s); online != nil && online.Windows != nil {
				return sendWindowResync(s, online.Windows, request.WindowID)
			}
			return nil
		}
		return nil
	})
}
