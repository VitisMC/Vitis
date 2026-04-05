package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// NbtQueryResponse is sent by the server in response to a block/entity NBT query.
type NbtQueryResponse struct {
	TransactionID int32
	Data          []byte
}

func NewNbtQueryResponse() protocol.Packet { return &NbtQueryResponse{} }

func (p *NbtQueryResponse) ID() int32 {
	return int32(packetid.ClientboundNbtQueryResponse)
}

func (p *NbtQueryResponse) Decode(buf *protocol.Buffer) error {
	var err error
	if p.TransactionID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		p.Data, err = buf.ReadBytes(remaining)
	}
	return err
}

func (p *NbtQueryResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.TransactionID)
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
