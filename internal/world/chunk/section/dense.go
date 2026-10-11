package section

import "fmt"

// UseDenseBlocks attaches an exclusively owned generation buffer in block-index order.
// Call PackDenseBlocks before exposing section palettes to runtime consumers.
func (c *Chunk) UseDenseBlocks(blocks []int32) error {
	if c == nil || len(blocks) != len(c.Sections)*BlocksPerSection {
		return fmt.Errorf("invalid dense chunk buffer length")
	}
	c.denseBlocks = blocks
	return nil
}

// DenseBlocks returns the generation buffer, or nil for a packed runtime chunk.
func (c *Chunk) DenseBlocks() []int32 { return c.denseBlocks }

// PackDenseBlocks packs generation states into owned palettes and detaches the buffer.
func (c *Chunk) PackDenseBlocks() {
	if c.denseBlocks == nil {
		return
	}
	for index, current := range c.Sections {
		current.FillBlocks((*[BlocksPerSection]int32)(c.denseBlocks[index*BlocksPerSection:]))
	}
	c.denseBlocks = nil
}
