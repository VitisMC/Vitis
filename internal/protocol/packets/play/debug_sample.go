package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// DebugSampleSubscription is sent by the client to subscribe to debug sample data.
type DebugSampleSubscription struct {
	SampleType int32
}

func NewDebugSampleSubscription() protocol.Packet { return &DebugSampleSubscription{} }

func (p *DebugSampleSubscription) ID() int32 {
	return int32(packetid.ServerboundDebugSampleSubscription)
}

func (p *DebugSampleSubscription) Decode(buf *protocol.Buffer) error {
	var err error
	p.SampleType, err = buf.ReadVarInt()
	return err
}

func (p *DebugSampleSubscription) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.SampleType)
	return nil
}

// DebugSample sends debug sample data to the client.
type DebugSample struct {
	Data []byte
}

func NewDebugSample() protocol.Packet { return &DebugSample{} }

func (p *DebugSample) ID() int32 {
	return int32(packetid.ClientboundDebugSample)
}

func (p *DebugSample) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *DebugSample) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
