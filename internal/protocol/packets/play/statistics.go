package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Statistic represents a single statistic entry.
type Statistic struct {
	CategoryID  int32
	StatisticID int32
	Value       int32
}

// Statistics sends player statistics to the client.
type Statistics struct {
	Entries []Statistic
}

func NewStatistics() protocol.Packet { return &Statistics{} }

func (p *Statistics) ID() int32 {
	return int32(packetid.ClientboundStatistics)
}

func (p *Statistics) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Entries = make([]Statistic, count)
	for i := int32(0); i < count; i++ {
		if p.Entries[i].CategoryID, err = buf.ReadVarInt(); err != nil {
			return err
		}
		if p.Entries[i].StatisticID, err = buf.ReadVarInt(); err != nil {
			return err
		}
		if p.Entries[i].Value, err = buf.ReadVarInt(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Statistics) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.Entries)))
	for _, e := range p.Entries {
		buf.WriteVarInt(e.CategoryID)
		buf.WriteVarInt(e.StatisticID)
		buf.WriteVarInt(e.Value)
	}
	return nil
}
