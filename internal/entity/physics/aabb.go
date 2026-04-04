package physics

import "math"

// AABB represents an axis-aligned bounding box in 3D space.
type AABB struct {
	MinX, MinY, MinZ float64
	MaxX, MaxY, MaxZ float64
}

// NewAABB creates an AABB from two corner points, normalizing min/max.
func NewAABB(x1, y1, z1, x2, y2, z2 float64) AABB {
	return AABB{
		MinX: math.Min(x1, x2), MinY: math.Min(y1, y2), MinZ: math.Min(z1, z2),
		MaxX: math.Max(x1, x2), MaxY: math.Max(y1, y2), MaxZ: math.Max(z1, z2),
	}
}

// Intersects returns true if this AABB overlaps with other (strict inequality).
func (a AABB) Intersects(b AABB) bool {
	return a.MinX < b.MaxX && a.MaxX > b.MinX &&
		a.MinY < b.MaxY && a.MaxY > b.MinY &&
		a.MinZ < b.MaxZ && a.MaxZ > b.MinZ
}

// Contains returns true if the point (x, y, z) is inside the AABB (inclusive).
func (a AABB) Contains(x, y, z float64) bool {
	return x >= a.MinX && x <= a.MaxX &&
		y >= a.MinY && y <= a.MaxY &&
		z >= a.MinZ && z <= a.MaxZ
}

// Offset returns a new AABB shifted by (dx, dy, dz).
func (a AABB) Offset(dx, dy, dz float64) AABB {
	return AABB{
		MinX: a.MinX + dx, MinY: a.MinY + dy, MinZ: a.MinZ + dz,
		MaxX: a.MaxX + dx, MaxY: a.MaxY + dy, MaxZ: a.MaxZ + dz,
	}
}

// Expand returns a new AABB expanded in the direction of (dx, dy, dz).
// Positive values expand the max side, negative expand the min side.
func (a AABB) Expand(dx, dy, dz float64) AABB {
	minX, maxX := a.MinX, a.MaxX
	minY, maxY := a.MinY, a.MaxY
	minZ, maxZ := a.MinZ, a.MaxZ

	if dx < 0 {
		minX += dx
	} else {
		maxX += dx
	}
	if dy < 0 {
		minY += dy
	} else {
		maxY += dy
	}
	if dz < 0 {
		minZ += dz
	} else {
		maxZ += dz
	}

	return AABB{MinX: minX, MinY: minY, MinZ: minZ, MaxX: maxX, MaxY: maxY, MaxZ: maxZ}
}

// Grow returns a new AABB expanded by amount in all directions.
func (a AABB) Grow(amount float64) AABB {
	return AABB{
		MinX: a.MinX - amount, MinY: a.MinY - amount, MinZ: a.MinZ - amount,
		MaxX: a.MaxX + amount, MaxY: a.MaxY + amount, MaxZ: a.MaxZ + amount,
	}
}

// Contract returns a new AABB shrunk by amount in all directions.
func (a AABB) Contract(amount float64) AABB {
	return a.Grow(-amount)
}

// SizeX returns the width along the X axis.
func (a AABB) SizeX() float64 { return a.MaxX - a.MinX }

// SizeY returns the height along the Y axis.
func (a AABB) SizeY() float64 { return a.MaxY - a.MinY }

// SizeZ returns the depth along the Z axis.
func (a AABB) SizeZ() float64 { return a.MaxZ - a.MinZ }

// CenterX returns the center X coordinate.
func (a AABB) CenterX() float64 { return (a.MinX + a.MaxX) * 0.5 }

// CenterY returns the center Y coordinate.
func (a AABB) CenterY() float64 { return (a.MinY + a.MaxY) * 0.5 }

// CenterZ returns the center Z coordinate.
func (a AABB) CenterZ() float64 { return (a.MinZ + a.MaxZ) * 0.5 }

// ClipXCollide computes the clipped X movement of this AABB against other.
// Returns the adjusted dx such that this box does not penetrate other.
func (a AABB) ClipXCollide(other AABB, dx float64) float64 {
	if other.MaxY <= a.MinY || other.MinY >= a.MaxY ||
		other.MaxZ <= a.MinZ || other.MinZ >= a.MaxZ {
		return dx
	}

	if dx > 0 && a.MaxX <= other.MinX {
		clip := other.MinX - a.MaxX
		if clip < dx {
			return clip
		}
	}
	if dx < 0 && a.MinX >= other.MaxX {
		clip := other.MaxX - a.MinX
		if clip > dx {
			return clip
		}
	}
	return dx
}

// ClipYCollide computes the clipped Y movement of this AABB against other.
// Returns the adjusted dy such that this box does not penetrate other.
func (a AABB) ClipYCollide(other AABB, dy float64) float64 {
	if other.MaxX <= a.MinX || other.MinX >= a.MaxX ||
		other.MaxZ <= a.MinZ || other.MinZ >= a.MaxZ {
		return dy
	}

	if dy > 0 && a.MaxY <= other.MinY {
		clip := other.MinY - a.MaxY
		if clip < dy {
			return clip
		}
	}
	if dy < 0 && a.MinY >= other.MaxY {
		clip := other.MaxY - a.MinY
		if clip > dy {
			return clip
		}
	}
	return dy
}

// ClipZCollide computes the clipped Z movement of this AABB against other.
// Returns the adjusted dz such that this box does not penetrate other.
func (a AABB) ClipZCollide(other AABB, dz float64) float64 {
	if other.MaxX <= a.MinX || other.MinX >= a.MaxX ||
		other.MaxY <= a.MinY || other.MinY >= a.MaxY {
		return dz
	}

	if dz > 0 && a.MaxZ <= other.MinZ {
		clip := other.MinZ - a.MaxZ
		if clip < dz {
			return clip
		}
	}
	if dz < 0 && a.MinZ >= other.MaxZ {
		clip := other.MaxZ - a.MinZ
		if clip > dz {
			return clip
		}
	}
	return dz
}

// IsZero returns true if the AABB has zero volume.
func (a AABB) IsZero() bool {
	return a.MinX == 0 && a.MinY == 0 && a.MinZ == 0 &&
		a.MaxX == 0 && a.MaxY == 0 && a.MaxZ == 0
}
