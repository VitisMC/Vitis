package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// FeatureFlags tells the client which feature flags are enabled.
type FeatureFlags struct {
	Flags []string
}

func NewFeatureFlags() protocol.Packet { return &FeatureFlags{} }

func (p *FeatureFlags) ID() int32 {
	return int32(packetid.ClientboundConfigFeatureFlags)
}

func (p *FeatureFlags) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Flags = make([]string, count)
	for i := int32(0); i < count; i++ {
		if p.Flags[i], err = buf.ReadString(); err != nil {
			return err
		}
	}
	return nil
}

func (p *FeatureFlags) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Flags)))
	for _, f := range p.Flags {
		buf.WriteString(f)
	}
	return nil
}
