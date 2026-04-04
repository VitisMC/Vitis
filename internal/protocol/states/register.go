package states

import "github.com/vitismc/vitis/internal/protocol"

// RegisterCore registers handshake, status, login, configuration and play packet mappings for one protocol version.
func RegisterCore(registry *protocol.Registry, version int32) error {
	if err := RegisterHandshake(registry, version); err != nil {
		return err
	}
	if err := RegisterStatus(registry, version); err != nil {
		return err
	}
	if err := RegisterLogin(registry, version); err != nil {
		return err
	}
	if err := RegisterConfiguration(registry, version); err != nil {
		return err
	}
	if err := RegisterPlay(registry, version); err != nil {
		return err
	}
	return nil
}
