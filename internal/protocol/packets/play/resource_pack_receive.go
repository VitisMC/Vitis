package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ResourcePackReceive is sent by the client to report the status of a resource pack download.
type ResourcePackReceive struct {
	UUID   protocol.UUID
	Result int32
}

func NewResourcePackReceive() protocol.Packet { return &ResourcePackReceive{} }

func (p *ResourcePackReceive) ID() int32 {
	return int32(packetid.ServerboundResourcePackReceive)
}

func (p *ResourcePackReceive) Decode(buf *protocol.Buffer) error {
	var err error
	if p.UUID, err = buf.ReadUUID(); err != nil {
		return err
	}
	p.Result, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	if p.Result < 0 || p.Result > 7 {
		return fmt.Errorf("resource pack result out of range: %d", p.Result)
	}
	return nil
}

func (p *ResourcePackReceive) Encode(buf *protocol.Buffer) error {
	buf.WriteUUID(p.UUID)
	buf.WriteVarInt(p.Result)
	return nil
}
