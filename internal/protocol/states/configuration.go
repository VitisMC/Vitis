package states

import (
	"github.com/vitismc/vitis/internal/protocol"
	cfgpacket "github.com/vitismc/vitis/internal/protocol/packets/configuration"
)

// RegisterConfiguration registers configuration-state packet mappings for a version.
func RegisterConfiguration(registry *protocol.Registry, version int32) error {
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewClientInformation); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewServerboundKnownPacks); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewAcknowledgeFinishConfiguration); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewServerboundPluginMessage); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewClientboundKnownPacks); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewRegistryData); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewFinishConfiguration); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewClientboundPluginMessage); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewUpdateTags); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewCookieRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewCookieResponse); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewDisconnect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewClientboundKeepAlive); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewServerboundKeepAlive); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewPing); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewPong); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewResetChat); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewRemoveResourcePack); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewAddResourcePack); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewResourcePackReceive); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewStoreCookie); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewTransfer); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewFeatureFlags); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewClientboundCustomReportDetails); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewServerboundCustomReportDetails); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionOutbound, cfgpacket.NewClientboundServerLinks); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateConfiguration, protocol.DirectionInbound, cfgpacket.NewServerboundServerLinks); err != nil {
		return err
	}
	return nil
}
