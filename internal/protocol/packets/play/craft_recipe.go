package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// CraftRecipeRequest is sent when the client clicks a recipe in the recipe book.
type CraftRecipeRequest struct {
	WindowID int32
	RecipeID int32
	MakeAll  bool
}

func NewCraftRecipeRequest() protocol.Packet { return &CraftRecipeRequest{} }

func (p *CraftRecipeRequest) ID() int32 {
	return int32(packetid.ServerboundCraftRecipeRequest)
}

func (p *CraftRecipeRequest) Decode(buf *protocol.Buffer) error {
	var err error
	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.WindowID = int32(b)
	if p.RecipeID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.MakeAll, err = buf.ReadBool()
	return err
}

func (p *CraftRecipeRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(byte(p.WindowID))
	buf.WriteVarInt(p.RecipeID)
	buf.WriteBool(p.MakeAll)
	return nil
}

// DisplayedRecipe is sent when the client interacts with a recipe in the recipe book UI.
type DisplayedRecipe struct {
	RecipeID int32
}

func NewDisplayedRecipe() protocol.Packet { return &DisplayedRecipe{} }

func (p *DisplayedRecipe) ID() int32 {
	return int32(packetid.ServerboundDisplayedRecipe)
}

func (p *DisplayedRecipe) Decode(buf *protocol.Buffer) error {
	var err error
	p.RecipeID, err = buf.ReadVarInt()
	return err
}

func (p *DisplayedRecipe) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.RecipeID)
	return nil
}
