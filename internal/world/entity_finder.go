package world

import (
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/entity/projectile"
)

const (
	playerWidth  = 0.6
	playerHeight = 1.8
)

// worldEntityFinder implements projectile.EntityFinder by scanning players and mobs.
type worldEntityFinder struct {
	tracker *entity.Tracker
	mobs    *entity.MobManager
}

// FindEntitiesInAABB returns all living entities whose bounding box overlaps the query box,
// excluding the entity with the given ID.
func (f *worldEntityFinder) FindEntitiesInAABB(box physics.AABB, exclude int32) []projectile.EntityHit {
	var hits []projectile.EntityHit

	// Check players
	if f.tracker != nil {
		for _, p := range f.tracker.Players() {
			if p.ID() == exclude || p.Removed() || p.IsDead() {
				continue
			}
			pos := p.Position()
			hw := playerWidth / 2
			pBox := physics.AABB{
				MinX: pos.X - hw, MinY: pos.Y, MinZ: pos.Z - hw,
				MaxX: pos.X + hw, MaxY: pos.Y + playerHeight, MaxZ: pos.Z + hw,
			}
			if box.Intersects(pBox) {
				hits = append(hits, projectile.EntityHit{
					ID:       p.ID(),
					Position: pos,
					Width:    playerWidth,
					Height:   playerHeight,
				})
			}
		}
	}

	// Check mobs
	if f.mobs != nil {
		for _, mob := range f.mobs.Mobs() {
			if mob.ID() == exclude || mob.Entity.Removed() || mob.IsDead() {
				continue
			}
			pos := mob.Position()
			def := mob.TypeDef()
			hw := def.Width / 2
			mBox := physics.AABB{
				MinX: pos.X - hw, MinY: pos.Y, MinZ: pos.Z - hw,
				MaxX: pos.X + hw, MaxY: pos.Y + def.Height, MaxZ: pos.Z + hw,
			}
			if box.Intersects(mBox) {
				hits = append(hits, projectile.EntityHit{
					ID:       mob.ID(),
					Position: pos,
					Width:    def.Width,
					Height:   def.Height,
				})
			}
		}
	}

	return hits
}
