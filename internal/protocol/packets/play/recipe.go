package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// RecipeBookSettings controls which recipe book tabs are open/filtering.
type RecipeBookSettings struct {
	CraftingOpen       bool
	CraftingFilter     bool
	SmeltingOpen       bool
	SmeltingFilter     bool
	BlastFurnaceOpen   bool
	BlastFurnaceFilter bool
	SmokerOpen         bool
	SmokerFilter       bool
}

func NewRecipeBookSettings() protocol.Packet { return &RecipeBookSettings{} }
func (p *RecipeBookSettings) ID() int32 {
	return int32(packetid.ClientboundRecipeBookSettings)
}
func (p *RecipeBookSettings) Decode(buf *protocol.Buffer) error {
	settings := []*bool{
		&p.CraftingOpen, &p.CraftingFilter,
		&p.SmeltingOpen, &p.SmeltingFilter,
		&p.BlastFurnaceOpen, &p.BlastFurnaceFilter,
		&p.SmokerOpen, &p.SmokerFilter,
	}
	for _, setting := range settings {
		value, err := buf.ReadBool()
		if err != nil {
			return err
		}
		*setting = value
	}
	return nil
}
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

// RecipeBookAdd adds typed recipe displays and optionally replaces the client book.
type RecipeBookAdd struct {
	Entries []byte
	Recipes []RecipeBookEntry
	Replace bool
}

// NewRecipeBookAdd constructs an empty recipe book packet.
func NewRecipeBookAdd() protocol.Packet { return &RecipeBookAdd{} }

// ID returns the protocol 769 recipe-book packet ID.
func (p *RecipeBookAdd) ID() int32 { return int32(packetid.ClientboundRecipeBookAdd) }

// Decode reads bounded definitions and the mandatory replacement flag.
func (p *RecipeBookAdd) Decode(buf *protocol.Buffer) error {
	n, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	if n < 0 || n > 16384 {
		return fmt.Errorf("recipe count exceeds limit")
	}
	p.Recipes = make([]RecipeBookEntry, n)
	for i := range p.Recipes {
		e := &p.Recipes[i]
		e.DisplayID, err = buf.ReadVarInt()
		if err != nil || e.DisplayID < 0 {
			return fmt.Errorf("invalid display ID")
		}
		e.Display, err = decodeRecipeDisplay(buf)
		if err != nil {
			return err
		}
		group, err := buf.ReadVarInt()
		if err != nil || group < 0 {
			return fmt.Errorf("invalid display group")
		}
		e.Group = group - 1
		e.Category, err = buf.ReadVarInt()
		if err != nil || e.Category < 0 || e.Category > 12 {
			return fmt.Errorf("invalid recipe category")
		}
		present, err := buf.ReadBool()
		if err != nil {
			return err
		}
		if present {
			n, err := buf.ReadVarInt()
			if err != nil || n < 0 || n > 9 {
				return fmt.Errorf("invalid requirements")
			}
			e.Requirements = make([][]int32, n)
			for j := range e.Requirements {
				count, err := buf.ReadVarInt()
				if err != nil || count < 1 || count > 4097 {
					return fmt.Errorf("invalid ingredient set")
				}
				e.Requirements[j] = make([]int32, count-1)
				for k := range e.Requirements[j] {
					id, err := buf.ReadVarInt()
					if err != nil || id < 0 {
						return fmt.Errorf("invalid ingredient ID")
					}
					e.Requirements[j][k] = id
				}
			}
		}
		e.Flags, err = buf.ReadByte()
		if err != nil || e.Flags > 3 {
			return fmt.Errorf("invalid recipe flags")
		}
	}
	p.Replace, err = buf.ReadBool()
	return err
}

// Encode writes recipe definitions followed by the mandatory replacement flag.
func (p *RecipeBookAdd) Encode(buf *protocol.Buffer) error {
	if len(p.Entries) > 0 {
		buf.WriteBytes(p.Entries)
	} else {
		if len(p.Recipes) > 16384 {
			return fmt.Errorf("recipe count exceeds limit")
		}
		buf.WriteVarInt(int32(len(p.Recipes)))
		for _, e := range p.Recipes {
			if e.DisplayID < 0 || e.Group < -1 || e.Category < 0 || e.Category > 12 || e.Flags > 3 {
				return fmt.Errorf("invalid recipe entry")
			}
			buf.WriteVarInt(e.DisplayID)
			if err := e.Display.Encode(buf); err != nil {
				return err
			}
			buf.WriteVarInt(e.Group + 1)
			buf.WriteVarInt(e.Category)
			buf.WriteBool(e.Requirements != nil)
			if e.Requirements != nil {
				if len(e.Requirements) > 9 {
					return fmt.Errorf("requirements exceed grid")
				}
				buf.WriteVarInt(int32(len(e.Requirements)))
				for _, ids := range e.Requirements {
					if len(ids) > 4096 {
						return fmt.Errorf("ingredient set too large")
					}
					buf.WriteVarInt(int32(len(ids) + 1))
					for _, id := range ids {
						if id < 0 {
							return fmt.Errorf("invalid ingredient ID")
						}
						buf.WriteVarInt(id)
					}
				}
			}
			buf.WriteByte(e.Flags)
		}
	}
	buf.WriteBool(p.Replace)
	return nil
}

// ServerboundRecipeBook is sent when the player interacts with the recipe book.
type ServerboundRecipeBook struct {
	BookType  int32
	Open      bool
	Filtering bool
}

func NewServerboundRecipeBook() protocol.Packet { return &ServerboundRecipeBook{} }
func (p *ServerboundRecipeBook) ID() int32 {
	return int32(packetid.ServerboundRecipeBook)
}
func (p *ServerboundRecipeBook) Decode(buf *protocol.Buffer) error {
	var err error
	if p.BookType, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.BookType < 0 || p.BookType > 3 {
		return fmt.Errorf("recipe book type out of range: %d", p.BookType)
	}
	if p.Open, err = buf.ReadBool(); err != nil {
		return err
	}
	p.Filtering, err = buf.ReadBool()
	return err
}
func (p *ServerboundRecipeBook) Encode(buf *protocol.Buffer) error {
	if p.BookType < 0 || p.BookType > 3 {
		return fmt.Errorf("recipe book type out of range: %d", p.BookType)
	}
	buf.WriteVarInt(p.BookType)
	buf.WriteBool(p.Open)
	buf.WriteBool(p.Filtering)
	return nil
}
