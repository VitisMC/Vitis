package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// MapData sends map item data to the client.
type MapData struct {
	Data []byte
}

func NewMapData() protocol.Packet { return &MapData{} }

func (p *MapData) ID() int32 {
	return int32(packetid.ClientboundMap)
}

func (p *MapData) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *MapData) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
