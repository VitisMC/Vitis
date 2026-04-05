package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ResetChat resets the client's chat session state during configuration.
type ResetChat struct{}

func NewResetChat() protocol.Packet { return &ResetChat{} }

func (p *ResetChat) ID() int32 {
	return int32(packetid.ClientboundConfigResetChat)
}

func (p *ResetChat) Decode(_ *protocol.Buffer) error { return nil }
func (p *ResetChat) Encode(_ *protocol.Buffer) error { return nil }
