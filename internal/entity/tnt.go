package entity

import (
	"math"

	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	EntityTypeTNT EntityType = EntityType(genentity.EntityTnt)

	tntDefaultFuse = 80
	tntPower       = 4.0
)

// TNTEntity represents a primed TNT block entity.
type TNTEntity struct {
	*Entity
	fuse    int32
	power   float64
	ownerID int32
}

// NewTNTEntity creates a primed TNT at the given position with default fuse.
func NewTNTEntity(id int32, uuid protocol.UUID, pos Vec3, ownerID int32) *TNTEntity {
	e := NewEntity(id, uuid, EntityTypeTNT, pos, Vec2{})
	e.SetProtocolInfo(genentity.EntityTnt, 0)
	e.vel = Vec3{X: 0, Y: 0.2, Z: 0}
	e.dirty |= DirtyVelocity
	return &TNTEntity{
		Entity:  e,
		fuse:    tntDefaultFuse,
		power:   tntPower,
		ownerID: ownerID,
	}
}

// NewTNTEntityWithFuse creates a primed TNT with a custom fuse time.
func NewTNTEntityWithFuse(id int32, uuid protocol.UUID, pos Vec3, ownerID int32, fuse int32) *TNTEntity {
	tnt := NewTNTEntity(id, uuid, pos, ownerID)
	tnt.fuse = fuse
	return tnt
}

// Fuse returns the remaining fuse ticks.
func (t *TNTEntity) Fuse() int32 { return t.fuse }

// Power returns the explosion power.
func (t *TNTEntity) Power() float64 { return t.power }

// SetPower sets the explosion power.
func (t *TNTEntity) SetPower(p float64) { t.power = p }

// OwnerID returns the entity that ignited this TNT.
func (t *TNTEntity) OwnerID() int32 { return t.ownerID }

// ProtocolType returns the protocol entity type ID.
func (t *TNTEntity) ProtocolType() int32 {
	return genentity.EntityTnt
}

// SpawnData returns the data field for SpawnEntity packet (fuse time).
func (t *TNTEntity) SpawnData() int32 {
	return t.fuse
}

// ShouldExplode returns true when the fuse has expired.
func (t *TNTEntity) ShouldExplode() bool {
	return t.fuse <= 0 && !t.removed
}

// Tick advances the TNT entity by one tick.
func (t *TNTEntity) Tick(world physics.BlockAccess) {
	if t == nil || t.removed {
		return
	}

	t.fuse--

	t.vel.Y -= physics.GravityTNT
	if t.vel.Y < physics.TerminalVelocity {
		t.vel.Y = physics.TerminalVelocity
	}

	dims := physics.TNTDimensions()
	box := dims.MakeBoundingBox(t.pos.X, t.pos.Y, t.pos.Z)
	result := physics.MoveWithCollision(world, box, t.vel.X, t.vel.Y, t.vel.Z)

	t.pos.X += result.Dx
	t.pos.Y += result.Dy
	t.pos.Z += result.Dz
	t.dirty |= DirtyPosition

	t.onGround = result.OnGround

	if result.CollidedX {
		t.vel.X = 0
	}
	if result.CollidedY {
		t.vel.Y *= -0.5
		if math.Abs(t.vel.Y) < 0.01 {
			t.vel.Y = 0
		}
	}
	if result.CollidedZ {
		t.vel.Z = 0
	}

	t.vel.X *= physics.DragTNT
	t.vel.Z *= physics.DragTNT

	if t.pos.Y < -128 {
		t.removed = true
	}

}

// Explosion holds the result of a TNT explosion to be applied by the world.
type Explosion struct {
	X, Y, Z          float64
	Power            float64
	AffectedBlocks   [][3]int
	AffectedEntities []ExplosionEntityEffect
}

// ExplosionEntityEffect describes the effect of an explosion on an entity.
type ExplosionEntityEffect struct {
	EntityID int32
	KnockX   float64
	KnockY   float64
	KnockZ   float64
	Damage   float64
}

// ComputeExplosion calculates which blocks and entities are affected by an explosion.
func ComputeExplosion(world physics.BlockAccess, x, y, z, power float64, entityFinder func(box physics.AABB) []ExplosionTarget) *Explosion {
	exp := &Explosion{X: x, Y: y, Z: z, Power: power}

	affectedSet := make(map[[3]int]struct{}, 256)
	const rays = 16
	for xi := 0; xi < rays; xi++ {
		for yi := 0; yi < rays; yi++ {
			for zi := 0; zi < rays; zi++ {
				if xi != 0 && xi != rays-1 && yi != 0 && yi != rays-1 && zi != 0 && zi != rays-1 {
					continue
				}
				dx := float64(xi)/float64(rays-1)*2 - 1
				dy := float64(yi)/float64(rays-1)*2 - 1
				dz := float64(zi)/float64(rays-1)*2 - 1
				dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
				if dist == 0 {
					continue
				}
				dx /= dist
				dy /= dist
				dz /= dist

				intensity := power * (0.7 + 0.3*0.6)
				rx, ry, rz := x, y, z

				for intensity > 0 {
					bx := int(math.Floor(rx))
					by := int(math.Floor(ry))
					bz := int(math.Floor(rz))

					stateID := world.GetBlockStateAt(bx, by, bz)
					if stateID > 0 {
						resistance := 0.3
						intensity -= (resistance + 0.3) * 0.3
						if intensity > 0 {
							affectedSet[[3]int{bx, by, bz}] = struct{}{}
						}
					}
					intensity -= 0.22500001

					rx += dx * 0.3
					ry += dy * 0.3
					rz += dz * 0.3
				}
			}
		}
	}

	exp.AffectedBlocks = make([][3]int, 0, len(affectedSet))
	for pos := range affectedSet {
		exp.AffectedBlocks = append(exp.AffectedBlocks, pos)
	}

	if entityFinder != nil {
		radius := power * 2
		searchBox := physics.AABB{
			MinX: x - radius, MinY: y - radius, MinZ: z - radius,
			MaxX: x + radius, MaxY: y + radius, MaxZ: z + radius,
		}

		targets := entityFinder(searchBox)
		for _, tgt := range targets {
			dx := tgt.X - x
			dy := tgt.Y + tgt.Height/2 - y
			dz := tgt.Z - z
			dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if dist > radius || dist == 0 {
				continue
			}

			exposure := 1.0 - dist/radius
			knockback := exposure

			exp.AffectedEntities = append(exp.AffectedEntities, ExplosionEntityEffect{
				EntityID: tgt.EntityID,
				KnockX:   dx / dist * knockback,
				KnockY:   dy / dist * knockback,
				KnockZ:   dz / dist * knockback,
				Damage:   math.Floor((exposure*exposure+exposure)*3.5*power + 1),
			})
		}
	}

	return exp
}

// ExplosionTarget is an entity that can be affected by an explosion.
type ExplosionTarget struct {
	EntityID int32
	X, Y, Z  float64
	Width    float64
	Height   float64
}

// TNTManager manages all TNT entities in a world.
type TNTManager struct {
	tnts map[int32]*TNTEntity
}

// NewTNTManager creates a new TNT manager.
func NewTNTManager() *TNTManager {
	return &TNTManager{
		tnts: make(map[int32]*TNTEntity, 16),
	}
}

// Add registers a TNT entity.
func (m *TNTManager) Add(tnt *TNTEntity) {
	if m == nil || tnt == nil {
		return
	}
	m.tnts[tnt.ID()] = tnt
}

// Remove removes a TNT entity by ID.
func (m *TNTManager) Remove(id int32) {
	if m == nil {
		return
	}
	delete(m.tnts, id)
}

// Get returns a TNT entity by ID.
func (m *TNTManager) Get(id int32) *TNTEntity {
	if m == nil {
		return nil
	}
	return m.tnts[id]
}

// All returns all TNT entities.
func (m *TNTManager) All() map[int32]*TNTEntity {
	if m == nil {
		return nil
	}
	return m.tnts
}

// TickAll advances all TNT entities and returns IDs of those that should explode.
func (m *TNTManager) TickAll(world physics.BlockAccess) (exploded []int32, removed []int32) {
	if m == nil {
		return nil, nil
	}

	for _, tnt := range m.tnts {
		tnt.Tick(world)
	}

	for id, tnt := range m.tnts {
		if tnt.ShouldExplode() {
			exploded = append(exploded, id)
		} else if tnt.removed {
			removed = append(removed, id)
		}
	}

	for _, id := range exploded {
		delete(m.tnts, id)
	}
	for _, id := range removed {
		delete(m.tnts, id)
	}

	return exploded, removed
}
