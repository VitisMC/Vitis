package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// StartConfiguration tells the client to re-enter the configuration state.
type StartConfiguration struct{}

func NewStartConfiguration() protocol.Packet { return &StartConfiguration{} }

func (p *StartConfiguration) ID() int32 {
	return int32(packetid.ClientboundStartConfiguration)
}

func (p *StartConfiguration) Decode(_ *protocol.Buffer) error { return nil }
func (p *StartConfiguration) Encode(_ *protocol.Buffer) error { return nil }
