package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// TradeList sends the villager trade list to the client.
type TradeList struct {
	Data []byte
}

func NewTradeList() protocol.Packet { return &TradeList{} }

func (p *TradeList) ID() int32 {
	return int32(packetid.ClientboundTradeList)
}

func (p *TradeList) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *TradeList) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
