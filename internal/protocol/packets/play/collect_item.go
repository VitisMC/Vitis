package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// CollectItem (clientbound) plays the pickup animation for an item or XP orb.
type CollectItem struct {
	CollectedEntityID int32
	CollectorEntityID int32
	PickupItemCount   int32
}

func NewCollectItem() protocol.Packet { return &CollectItem{} }

func (p *CollectItem) ID() int32 {
	return int32(packetid.ClientboundCollect)
}

func (p *CollectItem) Decode(buf *protocol.Buffer) error {
	var err error
	if p.CollectedEntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode collect item collected: %w", err)
	}
	if p.CollectorEntityID, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode collect item collector: %w", err)
	}
	if p.PickupItemCount, err = buf.ReadVarInt(); err != nil {
		return fmt.Errorf("decode collect item count: %w", err)
	}
	return nil
}

func (p *CollectItem) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.CollectedEntityID)
	buf.WriteVarInt(p.CollectorEntityID)
	buf.WriteVarInt(p.PickupItemCount)
	return nil
}
