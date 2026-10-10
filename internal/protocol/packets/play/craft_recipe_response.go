package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// CraftRecipeResponse is sent by the server in response to a craft recipe request.
type CraftRecipeResponse struct {
	WindowID int32
	RecipeID int32
	Display  RecipeDisplay
}

func NewCraftRecipeResponse() protocol.Packet { return &CraftRecipeResponse{} }

func (p *CraftRecipeResponse) ID() int32 {
	return int32(packetid.ClientboundCraftRecipeResponse)
}

func (p *CraftRecipeResponse) Decode(buf *protocol.Buffer) error {
	var err error
	p.WindowID, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	p.Display, err = decodeRecipeDisplay(buf)
	return err
}

func (p *CraftRecipeResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
	return p.Display.Encode(buf)
}
