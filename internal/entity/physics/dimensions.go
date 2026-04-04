package physics

import (
	genentity "github.com/vitismc/vitis/internal/data/generated/entity"
)

// EntityDimensions holds the width and height of an entity type.
type EntityDimensions struct {
	Width  float64
	Height float64
}

// MakeBoundingBox creates an AABB centered at (x, z) with feet at y.
func (d EntityDimensions) MakeBoundingBox(x, y, z float64) AABB {
	halfW := d.Width / 2.0
	return AABB{
		MinX: x - halfW,
		MinY: y,
		MinZ: z - halfW,
		MaxX: x + halfW,
		MaxY: y + d.Height,
		MaxZ: z + halfW,
	}
}

// DimensionsForType returns the EntityDimensions for a protocol entity type ID.
func DimensionsForType(entityTypeID int32) EntityDimensions {
	info := genentity.EntityByID(entityTypeID)
	if info == nil {
		return EntityDimensions{Width: 0.6, Height: 1.8}
	}
	return EntityDimensions{
		Width:  info.Width,
		Height: info.Height,
	}
}

// DimensionsForName returns the EntityDimensions for a named entity type.
func DimensionsForName(name string) EntityDimensions {
	info := genentity.EntityByName(name)
	if info == nil {
		return EntityDimensions{Width: 0.6, Height: 1.8}
	}
	return EntityDimensions{
		Width:  info.Width,
		Height: info.Height,
	}
}

// PlayerDimensions returns dimensions for the player entity.
func PlayerDimensions() EntityDimensions {
	return EntityDimensions{Width: 0.6, Height: 1.8}
}

// ItemDimensions returns dimensions for item entities.
func ItemDimensions() EntityDimensions {
	return EntityDimensions{Width: 0.25, Height: 0.25}
}

// XPOrbDimensions returns dimensions for experience orb entities.
func XPOrbDimensions() EntityDimensions {
	return EntityDimensions{Width: 0.5, Height: 0.5}
}

// ArrowDimensions returns dimensions for arrow entities.
func ArrowDimensions() EntityDimensions {
	return EntityDimensions{Width: 0.5, Height: 0.5}
}

// TNTDimensions returns dimensions for TNT entities.
func TNTDimensions() EntityDimensions {
	return EntityDimensions{Width: 0.98, Height: 0.98}
}
