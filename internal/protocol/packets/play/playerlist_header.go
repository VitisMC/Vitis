package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// PlayerlistHeader sets the tab list header and footer text.
// Both are Text Components encoded as NBT in 1.21.4.
type PlayerlistHeader struct {
	Header string
	Footer string
}

func NewPlayerlistHeader() protocol.Packet { return &PlayerlistHeader{} }

func (p *PlayerlistHeader) ID() int32 { return int32(packetid.ClientboundPlayerlistHeader) }

func (p *PlayerlistHeader) Decode(_ *protocol.Buffer) error { return nil }

func (p *PlayerlistHeader) Encode(buf *protocol.Buffer) error {
	for _, text := range []string{p.Header, p.Footer} {
		comp := nbt.NewCompound()
		comp.PutString("text", text)
		enc := nbt.NewEncoder(128)
		_ = enc.WriteRootCompound(comp)
		buf.WriteBytes(enc.Bytes())
	}
	return nil
}
