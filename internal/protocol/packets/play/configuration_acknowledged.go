package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ConfigurationAcknowledged is sent by the client to acknowledge re-entering the configuration state.
type ConfigurationAcknowledged struct{}

func NewConfigurationAcknowledged() protocol.Packet { return &ConfigurationAcknowledged{} }

func (p *ConfigurationAcknowledged) ID() int32 {
	return int32(packetid.ServerboundConfigurationAcknowledged)
}

func (p *ConfigurationAcknowledged) Decode(_ *protocol.Buffer) error { return nil }
func (p *ConfigurationAcknowledged) Encode(_ *protocol.Buffer) error { return nil }
