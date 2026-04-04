package entity

import (
	"math"

	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	EntityTypeFallingBlock EntityType = 10

	fallingBlockGravity  = 0.04
	fallingBlockDrag     = 0.98
	fallingBlockTerminal = -3.92
)

type FallingBlock struct {
	*Entity
	blockStateID int32
	ticksExisted int
}

func NewFallingBlock(id int32, uuid protocol.UUID, pos Vec3, blockStateID int32) *FallingBlock {
	e := NewEntity(id, uuid, EntityTypeFallingBlock, pos, Vec2{})
	e.SetProtocolInfo(genentity.EntityFallingBlock, blockStateID)
	e.SetClientSimulated(true)
	return &FallingBlock{
		Entity:       e,
		blockStateID: blockStateID,
	}
}

func (f *FallingBlock) BlockStateID() int32 {
	return f.blockStateID
}

func (f *FallingBlock) ProtocolType() int32 {
	return genentity.EntityFallingBlock
}

func (f *FallingBlock) SpawnData() int32 {
	return f.blockStateID
}

func (f *FallingBlock) Tick(collisionChecker func(x, y, z float64) bool) {
	if f == nil || f.removed {
		return
	}
	f.ticksExisted++
	if f.ticksExisted <= 1 {
		return
	}

	f.vel.Y -= fallingBlockGravity
	if f.vel.Y < fallingBlockTerminal {
		f.vel.Y = fallingBlockTerminal
	}

	newX := f.pos.X + f.vel.X
	newY := f.pos.Y + f.vel.Y
	newZ := f.pos.Z + f.vel.Z

	landed := false
	if collisionChecker != nil {
		blockX := int(math.Floor(newX))
		blockY := int(math.Floor(newY))
		blockZ := int(math.Floor(newZ))

		if collisionChecker(float64(blockX), float64(blockY), float64(blockZ)) {
			newY = float64(blockY + 1)
			f.vel.Y = 0
			landed = true
		}
	}

	f.pos = Vec3{X: newX, Y: newY, Z: newZ}
	f.dirty |= DirtyPosition

	f.vel.X *= fallingBlockDrag
	f.vel.Y *= fallingBlockDrag
	f.vel.Z *= fallingBlockDrag

	f.onGround = landed

	if f.pos.Y < -128 {
		f.removed = true
	}
}

func (f *FallingBlock) ShouldLand() bool {
	return f.onGround && f.ticksExisted > 1
}

func (f *FallingBlock) LandingPosition() (int, int, int) {
	return int(math.Floor(f.pos.X)), int(math.Floor(f.pos.Y)), int(math.Floor(f.pos.Z))
}

type FallingBlockManager struct {
	blocks map[int32]*FallingBlock
}

func NewFallingBlockManager() *FallingBlockManager {
	return &FallingBlockManager{
		blocks: make(map[int32]*FallingBlock, 64),
	}
}

func (m *FallingBlockManager) Add(fb *FallingBlock) {
	if m == nil || fb == nil {
		return
	}
	m.blocks[fb.ID()] = fb
}

func (m *FallingBlockManager) Remove(id int32) {
	if m == nil {
		return
	}
	delete(m.blocks, id)
}

func (m *FallingBlockManager) Get(id int32) *FallingBlock {
	if m == nil {
		return nil
	}
	return m.blocks[id]
}

func (m *FallingBlockManager) All() map[int32]*FallingBlock {
	if m == nil {
		return nil
	}
	return m.blocks
}

func (m *FallingBlockManager) Count() int {
	if m == nil {
		return 0
	}
	return len(m.blocks)
}
