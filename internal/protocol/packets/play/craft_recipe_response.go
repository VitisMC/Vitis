package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// CraftRecipeResponse is sent by the server in response to a craft recipe request.
type CraftRecipeResponse struct {
	WindowID int32
	RecipeID int32
}

func NewCraftRecipeResponse() protocol.Packet { return &CraftRecipeResponse{} }

func (p *CraftRecipeResponse) ID() int32 {
	return int32(packetid.ClientboundCraftRecipeResponse)
}

func (p *CraftRecipeResponse) Decode(buf *protocol.Buffer) error {
	var err error
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.WindowID = int32(b)
	p.RecipeID, err = buf.ReadVarInt()
	return err
}

func (p *CraftRecipeResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(byte(p.WindowID))
	buf.WriteVarInt(p.RecipeID)
	return nil
}
