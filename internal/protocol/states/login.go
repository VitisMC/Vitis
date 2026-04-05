package states

import (
	"github.com/vitismc/vitis/internal/protocol"
	loginpacket "github.com/vitismc/vitis/internal/protocol/packets/login"
)

// RegisterLogin registers login-state packet mappings for a version.
func RegisterLogin(registry *protocol.Registry, version int32) error {
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionInbound, loginpacket.NewLoginStart); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionInbound, loginpacket.NewLoginAcknowledged); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionOutbound, loginpacket.NewDisconnect); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionOutbound, loginpacket.NewLoginSuccess); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionOutbound, loginpacket.NewSetCompression); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionOutbound, loginpacket.NewEncryptionRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionInbound, loginpacket.NewEncryptionResponse); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionOutbound, loginpacket.NewLoginPluginRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionInbound, loginpacket.NewLoginPluginResponse); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionOutbound, loginpacket.NewCookieRequest); err != nil {
		return err
	}
	if err := registry.RegisterPacket(version, protocol.StateLogin, protocol.DirectionInbound, loginpacket.NewCookieResponse); err != nil {
		return err
	}
	return nil
}
