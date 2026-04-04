package states

import (
	"github.com/vitismc/vitis/internal/protocol"
	statuspacket "github.com/vitismc/vitis/internal/protocol/packets/status"
)

// RegisterStatus registers status-state packet mappings for a version.
func RegisterStatus(registry *protocol.Registry, version int32) error {
	if err := registry.RegisterPacket(version, protocol.StateStatus, protocol.DirectionInbound, statuspacket.NewStatusRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateStatus, protocol.DirectionOutbound, statuspacket.NewStatusResponse); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateStatus, protocol.DirectionInbound, statuspacket.NewPingRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateStatus, protocol.DirectionOutbound, statuspacket.NewPingResponse); err != nil {
		return err
	}
	return nil
}
