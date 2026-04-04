package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// DeclareCommands sends the command graph to the client.
// For now, sends a minimal graph with just the root node.
type DeclareCommands struct {
	Data []byte
}

func NewDeclareCommands() protocol.Packet { return &DeclareCommands{} }

func (p *DeclareCommands) ID() int32 { return int32(packetid.ClientboundDeclareCommands) }

func (p *DeclareCommands) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Data, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}

func (p *DeclareCommands) Encode(buf *protocol.Buffer) error {
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
		return nil
	}
	buf.WriteVarInt(1)
	buf.WriteByte(0)
	buf.WriteVarInt(0)
	buf.WriteVarInt(0)
	return nil
}

// EmptyCommandGraph returns a DeclareCommands with a single root node.
func EmptyCommandGraph() *DeclareCommands {
	return &DeclareCommands{}
}
