package states

import (
	"github.com/vitismc/vitis/internal/protocol"
	handshakepacket "github.com/vitismc/vitis/internal/protocol/packets/handshake"
)

// RegisterHandshake registers handshake-state packet mappings for a version.
func RegisterHandshake(registry *protocol.Registry, version int32) error {
	if err := registry.RegisterPacket(version, protocol.StateHandshake, protocol.DirectionInbound, handshakepacket.New); err != nil {
		return err
	}
	return nil
}
