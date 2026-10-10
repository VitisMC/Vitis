package play

import (
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
)

// SlotDisplay describes one ingredient, result, station, or recursive alternative.
type SlotDisplay struct {
	Type     int32
	ItemID   int32
	Stack    inventory.ItemStack
	Tag      string
	Children []SlotDisplay
}

// RecipeDisplay is the typed protocol 769 recipe display union.
type RecipeDisplay struct {
	Type                  int32
	Width, Height         int32
	Ingredients           []SlotDisplay
	Result, Station, Fuel SlotDisplay
	Duration              int32
	Experience            float32
}

// RecipeBookEntry binds an ephemeral display ID to a display and requirements.
type RecipeBookEntry struct {
	DisplayID    int32
	Display      RecipeDisplay
	Group        int32
	Category     int32
	Requirements [][]int32
	Flags        byte
}

// Encode writes one bounded slot display.
func (s SlotDisplay) Encode(buf *protocol.Buffer) error { return s.encode(buf, 0) }

// encode validates recursive display depth and the discriminated payload.
func (s SlotDisplay) encode(buf *protocol.Buffer, depth int) error {
	if depth > 16 {
		return fmt.Errorf("slot display nesting exceeds limit")
	}
	buf.WriteVarInt(s.Type)
	switch s.Type {
	case 0, 1:
		return nil
	case 2:
		if s.ItemID <= 0 {
			return fmt.Errorf("invalid display item")
		}
		buf.WriteVarInt(s.ItemID)
	case 3:
		return inventory.EncodeItemStack(buf, s.Stack)
	case 4:
		if s.Tag == "" || len(s.Tag) > 32767 {
			return fmt.Errorf("invalid display tag")
		}
		buf.WriteString(s.Tag)
	case 5, 6, 7:
		if s.Type == 5 && len(s.Children) != 3 || s.Type == 6 && len(s.Children) != 2 || len(s.Children) > 256 {
			return fmt.Errorf("invalid display children")
		}
		if s.Type == 7 {
			buf.WriteVarInt(int32(len(s.Children)))
		}
		for _, child := range s.Children {
			if err := child.encode(buf, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown slot display type %d", s.Type)
	}
	return nil
}

// decodeSlotDisplay reads one bounded recursive display union.
func decodeSlotDisplay(buf *protocol.Buffer, depth int) (SlotDisplay, error) {
	var s SlotDisplay
	if depth > 16 {
		return s, fmt.Errorf("slot display nesting exceeds limit")
	}
	typ, err := buf.ReadVarInt()
	if err != nil {
		return s, err
	}
	s.Type = typ
	switch typ {
	case 0, 1:
	case 2:
		s.ItemID, err = buf.ReadVarInt()
		if err == nil && s.ItemID <= 0 {
			err = fmt.Errorf("invalid display item")
		}
	case 3:
		s.Stack, err = inventory.DecodeItemStack(buf)
	case 4:
		s.Tag, err = buf.ReadString()
	case 5, 6, 7:
		count := int32(2)
		if typ == 5 {
			count = 3
		}
		if typ == 7 {
			count, err = buf.ReadVarInt()
		}
		if err != nil {
			return s, err
		}
		if count < 0 || count > 256 {
			return s, fmt.Errorf("display children exceeds limit")
		}
		s.Children = make([]SlotDisplay, count)
		for i := range s.Children {
			s.Children[i], err = decodeSlotDisplay(buf, depth+1)
			if err != nil {
				return s, err
			}
		}
	default:
		err = fmt.Errorf("unknown slot display type %d", typ)
	}
	return s, err
}

// Encode writes a validated recipe display union.
func (r RecipeDisplay) Encode(buf *protocol.Buffer) error {
	if r.Type < 0 || r.Type > 4 {
		return fmt.Errorf("unknown recipe display type")
	}
	buf.WriteVarInt(r.Type)
	switch r.Type {
	case 0, 1:
		if len(r.Ingredients) > 9 {
			return fmt.Errorf("too many grid ingredients")
		}
		if r.Type == 1 {
			if r.Width < 1 || r.Width > 3 || r.Height < 1 || r.Height > 3 || int(r.Width*r.Height) != len(r.Ingredients) {
				return fmt.Errorf("invalid recipe dimensions")
			}
			buf.WriteVarInt(r.Width)
			buf.WriteVarInt(r.Height)
		}
		buf.WriteVarInt(int32(len(r.Ingredients)))
		for _, ingredient := range r.Ingredients {
			if err := ingredient.Encode(buf); err != nil {
				return err
			}
		}
	case 2:
		if len(r.Ingredients) != 1 || r.Duration < 0 || math.IsNaN(float64(r.Experience)) || math.IsInf(float64(r.Experience), 0) || r.Experience < 0 {
			return fmt.Errorf("invalid cooking display")
		}
		if err := r.Ingredients[0].Encode(buf); err != nil {
			return err
		}
		if err := r.Fuel.Encode(buf); err != nil {
			return err
		}
	case 3, 4:
		expected := 1
		if r.Type == 4 {
			expected = 3
		}
		if len(r.Ingredients) != expected {
			return fmt.Errorf("invalid station ingredients")
		}
		for _, ingredient := range r.Ingredients {
			if err := ingredient.Encode(buf); err != nil {
				return err
			}
		}
	}
	if err := r.Result.Encode(buf); err != nil {
		return err
	}
	if err := r.Station.Encode(buf); err != nil {
		return err
	}
	if r.Type == 2 {
		buf.WriteVarInt(r.Duration)
		buf.WriteFloat32(r.Experience)
	}
	return nil
}

// decodeRecipeDisplay reads and validates a protocol recipe display.
func decodeRecipeDisplay(buf *protocol.Buffer) (RecipeDisplay, error) {
	var r RecipeDisplay
	typ, err := buf.ReadVarInt()
	if err != nil {
		return r, err
	}
	r.Type = typ
	count := int32(0)
	switch typ {
	case 0, 1:
		if typ == 1 {
			r.Width, err = buf.ReadVarInt()
			if err != nil {
				return r, err
			}
			r.Height, err = buf.ReadVarInt()
			if err != nil {
				return r, err
			}
		}
		count, err = buf.ReadVarInt()
	case 2, 3:
		count = 1
	case 4:
		count = 3
	default:
		return r, fmt.Errorf("unknown recipe display type")
	}
	if err != nil || count < 0 || count > 9 {
		return r, fmt.Errorf("invalid recipe ingredients")
	}
	r.Ingredients = make([]SlotDisplay, count)
	for i := range r.Ingredients {
		r.Ingredients[i], err = decodeSlotDisplay(buf, 0)
		if err != nil {
			return r, err
		}
	}
	if typ == 2 {
		r.Fuel, err = decodeSlotDisplay(buf, 0)
		if err != nil {
			return r, err
		}
	}
	r.Result, err = decodeSlotDisplay(buf, 0)
	if err != nil {
		return r, err
	}
	r.Station, err = decodeSlotDisplay(buf, 0)
	if err != nil {
		return r, err
	}
	if typ == 2 {
		r.Duration, err = buf.ReadVarInt()
		if err != nil {
			return r, err
		}
		r.Experience, err = buf.ReadFloat32()
		if err != nil {
			return r, err
		}
	}
	check := protocol.NewBuffer(128)
	if err = r.Encode(check); err != nil {
		return r, err
	}
	return r, nil
}
