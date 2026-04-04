package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// RecipeBookSettings controls which recipe book tabs are open/filtering.
type RecipeBookSettings struct {
	CraftingOpen     bool
	CraftingFilter   bool
	SmeltingOpen     bool
	SmeltingFilter   bool
	BlastFurnaceOpen bool
	BlastFurnaceFilter bool
	SmokerOpen       bool
	SmokerFilter     bool
}

func NewRecipeBookSettings() protocol.Packet { return &RecipeBookSettings{} }
func (p *RecipeBookSettings) ID() int32 {
	return int32(packetid.ClientboundRecipeBookSettings)
}
func (p *RecipeBookSettings) Decode(_ *protocol.Buffer) error { return nil }
func (p *RecipeBookSettings) Encode(buf *protocol.Buffer) error {
	for _, v := range []bool{
		p.CraftingOpen, p.CraftingFilter,
		p.SmeltingOpen, p.SmeltingFilter,
		p.BlastFurnaceOpen, p.BlastFurnaceFilter,
		p.SmokerOpen, p.SmokerFilter,
	} {
		if v {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	}
	return nil
}

// RecipeBookAdd adds recipes to the client's recipe book.
// Raw format: each entry is complex, so we send an empty list for now.
type RecipeBookAdd struct {
	Entries []byte
}

func NewRecipeBookAdd() protocol.Packet { return &RecipeBookAdd{} }
func (p *RecipeBookAdd) ID() int32 {
	return int32(packetid.ClientboundRecipeBookAdd)
}
func (p *RecipeBookAdd) Decode(_ *protocol.Buffer) error { return nil }
func (p *RecipeBookAdd) Encode(buf *protocol.Buffer) error {
	if len(p.Entries) > 0 {
		buf.WriteBytes(p.Entries)
	} else {
		buf.WriteVarInt(0)
	}
	return nil
}

// ServerboundRecipeBook is sent when the player interacts with the recipe book.
type ServerboundRecipeBook struct {
	Raw []byte
}

func NewServerboundRecipeBook() protocol.Packet { return &ServerboundRecipeBook{} }
func (p *ServerboundRecipeBook) ID() int32 {
	return int32(packetid.ServerboundRecipeBook)
}
func (p *ServerboundRecipeBook) Decode(buf *protocol.Buffer) error {
	remaining := buf.Remaining()
	if remaining > 0 {
		var err error
		p.Raw, err = buf.ReadBytes(remaining)
		return err
	}
	return nil
}
func (p *ServerboundRecipeBook) Encode(buf *protocol.Buffer) error {
	buf.WriteBytes(p.Raw)
	return nil
}
