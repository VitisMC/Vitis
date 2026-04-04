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
	return nil
}
