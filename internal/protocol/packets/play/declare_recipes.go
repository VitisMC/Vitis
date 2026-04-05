package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// DeclareRecipes sends the full recipe list to the client.
type DeclareRecipes struct {
	Data []byte
}

func NewDeclareRecipes() protocol.Packet { return &DeclareRecipes{} }

func (p *DeclareRecipes) ID() int32 {
	return int32(packetid.ClientboundDeclareRecipes)
}

func (p *DeclareRecipes) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *DeclareRecipes) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
