package session

import (
	"fmt"

	"github.com/vitismc/vitis/internal/crafting"
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// validatedNoOp describes a decoded, bounded packet that is intentionally
// accepted without changing authoritative server state.
type validatedNoOp struct {
	factory func() protocol.Packet
	reason  string
}

func playValidatedNoOps() []validatedNoOp {
	return []validatedNoOp{
		{playpacket.NewTickEnd, "client tick delimiter is advisory"},
		{playpacket.NewServerboundPong, "no clientbound play ping is outstanding"},
		{playpacket.NewConfigurationAcknowledged, "runtime reconfiguration is not initiated in the MVP"},
		{playpacket.NewServerboundPlayCustomPayload, "plugin channels are unavailable before the plugin API"},
		{playpacket.NewMessageAcknowledgement, "signed chat is disabled for the MVP"},
		{playpacket.NewChatSessionUpdate, "signed chat is disabled for the MVP"},
		{playpacket.NewResourcePackReceive, "the MVP does not offer a resource pack"},
		{playpacket.NewAdvancementTab, "advancements are outside the MVP"},
		{playpacket.NewServerboundCookieResponse, "no play-state cookie request is issued"},
		{playpacket.NewDebugSampleSubscription, "remote debug samples are disabled"},
		{playpacket.NewVehicleMove, "vehicle ownership is not implemented"},
		{playpacket.NewSteerBoat, "vehicle ownership is not implemented"},
		{playpacket.NewSetDifficulty, "client difficulty changes are not authoritative"},
		{playpacket.NewLockDifficulty, "client difficulty changes are not authoritative"},
		{playpacket.NewPickItemFromBlock, "pick-block is not authoritative in survival"},
		{playpacket.NewPickItemFromEntity, "pick-entity is not authoritative in survival"},
		{playpacket.NewNameItem, "anvil containers are not implemented"},
		{playpacket.NewSelectTrade, "villager trading is outside the MVP"},
		{playpacket.NewSetBeaconEffect, "beacon containers are not implemented"},
		{playpacket.NewSpectate, "spectator teleport is not implemented"},
		{playpacket.NewSetSlotState, "crafter slot control is not implemented"},
		{playpacket.NewQueryBlockNbt, "operator NBT queries are not implemented"},
		{playpacket.NewQueryEntityNbt, "operator NBT queries are not implemented"},
		{playpacket.NewSelectBundleItem, "bundle item selection is not implemented"},
		{playpacket.NewGenerateStructure, "structure block generation is outside the MVP"},
		{playpacket.NewUpdateCommandBlock, "command block editing is outside the MVP"},
		{playpacket.NewUpdateCommandBlockMinecart, "command block editing is outside the MVP"},
		{playpacket.NewUpdateJigsawBlock, "jigsaw editing is outside the MVP"},
		{playpacket.NewUpdateStructureBlock, "structure block editing is outside the MVP"},
	}
}

// registerRecipeBookHandlers registers authoritative per-player recipe book updates.
func registerRecipeBookHandlers(router PacketRouter, players *PlayerManager) error {
	settingsID := playpacket.NewServerboundRecipeBook().ID()
	if err := router.Register(protocol.StatePlay, settingsID, func(session Session, packet protocol.Packet) error {
		settings, ok := packet.(*playpacket.ServerboundRecipeBook)
		if !ok {
			return protocol.ErrNilPacket
		}
		if players == nil {
			return nil
		}
		player := players.GetBySession(session)
		if player == nil {
			return nil
		}
		if player.RecipeBook == nil {
			player.RecipeBook = crafting.NewRecipeBook()
		}
		return player.RecipeBook.SetSettings(crafting.BookType(settings.BookType), settings.Open, settings.Filtering)
	}); err != nil {
		return err
	}
	seenID := playpacket.NewDisplayedRecipe().ID()
	return router.Register(protocol.StatePlay, seenID, func(session Session, packet protocol.Packet) error {
		seen, ok := packet.(*playpacket.DisplayedRecipe)
		if !ok {
			return protocol.ErrNilPacket
		}
		if players != nil {
			if player := players.GetBySession(session); player != nil && player.RecipeBook != nil {
				player.RecipeBook.MarkSeen(seen.RecipeID)
			}
		}
		return nil
	})
}

func registerPlayValidatedNoOps(router PacketRouter) error {
	for _, policy := range playValidatedNoOps() {
		policy := policy
		packet := policy.factory()
		if packet == nil || policy.reason == "" {
			return fmt.Errorf("invalid validated no-op policy")
		}
		expectedID := packet.ID()
		if err := router.Register(protocol.StatePlay, expectedID, func(_ Session, packet protocol.Packet) error {
			if packet == nil || packet.ID() != expectedID {
				return protocol.ErrNilPacket
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
