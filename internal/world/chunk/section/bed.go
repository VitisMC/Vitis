package section

import (
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/entity"
)

// syncBedBlockEntity maintains the renderer entity for each bed half.
func (c *Chunk) syncBedBlockEntity(x, y, z int, state int32) {
	current := c.GetBlockEntity(int32(x), int32(y), int32(z))
	if strings.HasSuffix(block.NameFromState(state), "_bed") {
		if current == nil || current.TypeName() != "minecraft:bed" {
			c.AddBlockEntity(entity.NewBaseBlockEntity("minecraft:bed", int32(x), int32(y), int32(z)))
		}
	} else if current != nil && current.TypeName() == "minecraft:bed" {
		c.RemoveBlockEntity(int32(x), int32(y), int32(z))
	}
}

// RepairBedBlockEntities restores renderer entities missing from older chunks.
// Existing bed NBT remains owned by its original entity.
func (c *Chunk) RepairBedBlockEntities() int {
	repaired := 0
	for index, section := range c.Sections {
		if palette := section.Blocks.Palette(); c.denseBlocks == nil && palette != nil {
			hasBed := false
			for _, state := range palette {
				if strings.HasSuffix(block.NameFromState(state), "_bed") {
					hasBed = true
					break
				}
			}
			if !hasBed {
				continue
			}
		}
		for y := c.minY + index*SectionHeight; y < min(c.maxY+1, c.minY+(index+1)*SectionHeight); y++ {
			for z := int(c.Z) * 16; z < int(c.Z)*16+16; z++ {
				for x := int(c.X) * 16; x < int(c.X)*16+16; x++ {
					state := c.GetBlock(x, y, z)
					if !strings.HasSuffix(block.NameFromState(state), "_bed") {
						continue
					}
					if current := c.GetBlockEntity(int32(x), int32(y), int32(z)); current == nil || current.TypeName() != "minecraft:bed" {
						c.syncBedBlockEntity(x, y, z, state)
						repaired++
					}
				}
			}
		}
	}
	return repaired
}
