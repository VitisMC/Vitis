package play

import (
	"fmt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// RecipeBookRemove removes recipes from the client's recipe book.
type RecipeBookRemove struct {
	RecipeIDs []int32
}

func NewRecipeBookRemove() protocol.Packet { return &RecipeBookRemove{} }

func (p *RecipeBookRemove) ID() int32 {
	return int32(packetid.ClientboundRecipeBookRemove)
}

func (p *RecipeBookRemove) Decode(buf *protocol.Buffer) error {
	count, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	if count < 0 || count > 16384 {
		return fmt.Errorf("recipe removal count exceeds limit")
	}
	p.RecipeIDs = make([]int32, count)
	for i := int32(0); i < count; i++ {
		if p.RecipeIDs[i], err = buf.ReadVarInt(); err != nil {
			return err
		}
		if p.RecipeIDs[i] < 0 {
			return fmt.Errorf("negative recipe display ID")
		}
	}
	return nil
}

func (p *RecipeBookRemove) Encode(buf *protocol.Buffer) error {
	if len(p.RecipeIDs) > 16384 {
		return fmt.Errorf("recipe removal count exceeds limit")
	}
	buf.WriteVarInt(int32(len(p.RecipeIDs)))
	for _, id := range p.RecipeIDs {
		if id < 0 {
			return fmt.Errorf("negative recipe display ID")
		}
		buf.WriteVarInt(id)
	}
	return nil
}
