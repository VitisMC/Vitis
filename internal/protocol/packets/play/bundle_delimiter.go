package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// BundleDelimiter marks the start/end of a bundle of packets that should be applied atomically.
type BundleDelimiter struct{}

func NewBundleDelimiter() protocol.Packet { return &BundleDelimiter{} }

func (p *BundleDelimiter) ID() int32 { return int32(packetid.ClientboundBundleDelimiter) }

func (p *BundleDelimiter) Decode(_ *protocol.Buffer) error { return nil }

func (p *BundleDelimiter) Encode(_ *protocol.Buffer) error { return nil }
