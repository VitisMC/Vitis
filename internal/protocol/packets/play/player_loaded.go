package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PlayerLoaded is sent by the client when it has finished loading after login.
type PlayerLoaded struct{}

func NewPlayerLoaded() protocol.Packet { return &PlayerLoaded{} }

func (p *PlayerLoaded) ID() int32 { return int32(packetid.ServerboundPlayerLoaded) }

func (p *PlayerLoaded) Decode(_ *protocol.Buffer) error { return nil }

func (p *PlayerLoaded) Encode(_ *protocol.Buffer) error { return nil }
