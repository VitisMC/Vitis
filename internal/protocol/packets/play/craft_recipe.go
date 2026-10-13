package play

import (
	"fmt"

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
	p.WindowID, err = buf.ReadVarInt()
	if err != nil {
		return err
	}
	if p.WindowID < 0 {
		return fmt.Errorf("invalid container ID")
	}
	if p.RecipeID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	p.MakeAll, err = buf.ReadBool()
	if err == nil && p.RecipeID < 0 {
		return fmt.Errorf("recipe display ID out of range: %d", p.RecipeID)
	}
	return err
}

func (p *CraftRecipeRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.WindowID)
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
	if err == nil && p.RecipeID < 0 {
		return fmt.Errorf("recipe display ID out of range: %d", p.RecipeID)
	}
	return err
}

func (p *DisplayedRecipe) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.RecipeID)
	return nil
}
