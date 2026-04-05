package configuration

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundCustomReportDetails sends custom report details to the client during configuration.
type ClientboundCustomReportDetails struct {
	Details map[string]string
}

func NewClientboundCustomReportDetails() protocol.Packet { return &ClientboundCustomReportDetails{} }

func (p *ClientboundCustomReportDetails) ID() int32 {
	return int32(packetid.ClientboundConfigCustomReportDetails)
}

func (p *ClientboundCustomReportDetails) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Details = make(map[string]string, count)
	for i := int32(0); i < count; i++ {
		key, err := buf.ReadString()
		if err != nil {
			return err
		}
		val, err := buf.ReadString()
		if err != nil {
			return err
		}
		p.Details[key] = val
	}
	return nil
}

func (p *ClientboundCustomReportDetails) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Details)))
	for k, v := range p.Details {
		buf.WriteString(k)
		buf.WriteString(v)
	}
	return nil
}

// ServerboundCustomReportDetails is sent by the client to acknowledge custom report details during configuration.
type ServerboundCustomReportDetails struct{}

func NewServerboundCustomReportDetails() protocol.Packet { return &ServerboundCustomReportDetails{} }

func (p *ServerboundCustomReportDetails) ID() int32 {
	return int32(packetid.ServerboundConfigCustomReportDetails)
}

func (p *ServerboundCustomReportDetails) Decode(_ *protocol.Buffer) error { return nil }
func (p *ServerboundCustomReportDetails) Encode(_ *protocol.Buffer) error { return nil }
