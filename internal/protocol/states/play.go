package states

import (
	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// RegisterPlay registers play-state packet mappings for a version.
func RegisterPlay(registry *protocol.Registry, version int32) error {
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewKeepAliveServerbound); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewKeepAliveClientbound); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSpawnEntity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewRemoveEntities); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateEntityPosition); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateEntityPositionAndRotation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateEntityRotation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewTeleportEntity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetHeadRotation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetEntityMetadata); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewChunkBatchStart); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewChunkBatchFinished); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewChunkBatchReceived); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewChunkDataAndUpdateLight); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUnloadChunk); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewLoginPlay); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSyncPlayerPosition); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetDefaultSpawnPosition); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetCenterChunk); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewGameEvent); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewConfirmTeleportation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetPlayerPosition); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetPlayerPositionAndRotation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetPlayerRotation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetPlayerOnGround); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewPlayerInfoUpdate); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewPlayerInfoRemove); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewPlayerLoaded); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDisconnect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewPlayerAbilitiesClientbound); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewPlayerAbilitiesServerbound); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateTime); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSystemChatMessage); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDisguisedChatMessage); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewChatCommand); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewChatCommandSigned); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetTickingState); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewServerData); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewBlockDig); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewUseItemOn); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewBlockUpdate); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewAcknowledgeBlockChange); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDeclareCommands); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateHealth); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewArmAnimation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewEntityAction); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundHeldItemSlot); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewClientCommand); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundSettings); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSoundEffect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEntityAnimation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetExperience); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewRespawn); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEntityVelocity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetCreativeSlot); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewUseItem); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundChatMessage); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSyncEntityPosition); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewBundleDelimiter); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewPlayerlistHeader); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetTitleText); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetTitleSubtitle); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetTitleTime); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewActionBar); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewClearTitles); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewInitializeWorldBorder); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetBorderCenter); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetBorderSize); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetBorderWarningDelay); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetBorderWarningDistance); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEntityEquipment); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewTickEnd); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundPingRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundPong); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundPlayCustomPayload); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewWindowClick); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewCloseWindow); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewPlayerInput); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewScoreboardObjective); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDisplayObjective); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateScore); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewBossBar); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetContainerContent); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetContainerSlot); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewClientboundCloseContainer); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewOpenScreen); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetContainerProperty); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewRecipeBookSettings); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewRecipeBookAdd); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewServerboundRecipeBook); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetCursorItem); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewClientboundHeldItemSlot); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetPlayerInventorySlot); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDamageEvent); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewHurtAnimation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDeathCombatEvent); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewBlockEntityData); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewCollectItem); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewUseEntity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetPassengers); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewMessageAcknowledgement); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewChatSessionUpdate); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewConfigurationAcknowledged); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewVehicleMove); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSteerBoat); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewUpdateSign); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewResourcePackReceive); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewMultiBlockChange); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewBlockBreakAnimation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewBlockAction); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewExplosion); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewStartConfiguration); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewPlayerChat); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEnterCombatEvent); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEndCombatEvent); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEntityUpdateAttributes); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEntityEffect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewRemoveEntityEffect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateLight); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewDifficulty); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetDifficulty); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewLockDifficulty); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewEnchantItem); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewEditBook); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewPickItemFromBlock); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewPickItemFromEntity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewCraftRecipeRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewDisplayedRecipe); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewNameItem); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewAdvancementTab); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSelectTrade); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetBeaconEffect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSpectate); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionInbound, playpacket.NewSetSlotState); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSpawnEntityExperienceOrb); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewStatistics); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSetCooldown); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewClientboundCustomPayload); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewClientboundVehicleMove); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewOpenBook); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewOpenSignEntity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewFacePlayer); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewClientboundPlayerRotation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewAttachEntity); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewEntitySoundEffect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewStopSound); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSelectAdvancementTab); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewSimulationDistance); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewUpdateViewDistance); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewCamera); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewCraftRecipeResponse); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewRecipeBookRemove); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StatePlay, protocol.DirectionOutbound, playpacket.NewAdvancements); err != nil {
		return err
	}
	return nil
}
