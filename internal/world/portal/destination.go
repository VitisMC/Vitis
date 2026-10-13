package portal

import (
	"fmt"
	"math"

	"github.com/vitismc/vitis/internal/block"
)

// Rectangle describes the contiguous interior of a rectangular portal.
type Rectangle struct {
	Min           BlockPos
	Axis          string
	Width, Height int
}

// RectangleAt resolves the interior of a complete rectangular portal at a loaded position.
func RectangleAt(position BlockPos, stateAt func(int, int, int) int32) (Rectangle, bool) {
	state := stateAt(position.X, position.Y, position.Z)
	if block.NameFromState(state) != "minecraft:nether_portal" {
		return Rectangle{}, false
	}
	axis := block.PropertiesFromState(state)["axis"]
	dx, dz := 1, 0
	if axis == "z" {
		dx, dz = 0, 1
	} else if axis != "x" {
		return Rectangle{}, false
	}
	min := position
	for n := 0; n < 20 && stateAt(min.X, min.Y-1, min.Z) == state; n++ {
		min.Y--
	}
	for n := 0; n < 20 && stateAt(min.X-dx, min.Y, min.Z-dz) == state; n++ {
		min.X -= dx
		min.Z -= dz
	}
	width := 0
	for width < 21 && stateAt(min.X+width*dx, min.Y, min.Z+width*dz) == state {
		width++
	}
	height := 0
	for height < 21 {
		complete := true
		for x := 0; x < width; x++ {
			if stateAt(min.X+x*dx, min.Y+height, min.Z+x*dz) != state {
				complete = false
				break
			}
		}
		if !complete {
			break
		}
		height++
	}
	if width < 2 || height < 3 {
		return Rectangle{}, false
	}
	return Rectangle{Min: min, Axis: axis, Width: width, Height: height}, true
}

// RelativePosition normalizes a standing player's position inside an entry rectangle.
func RelativePosition(rect Rectangle, x, y, z float64) (horizontal, vertical, depth float64) {
	along, origin, across, plane := x, float64(rect.Min.X), z, float64(rect.Min.Z)
	if rect.Axis == "z" {
		along, origin, across, plane = z, float64(rect.Min.Z), x, float64(rect.Min.X)
	}
	horizontal = .5
	if span := float64(rect.Width) - .6; span > 0 {
		horizontal = math.Max(0, math.Min(1, (along-origin-.3)/span))
	}
	if span := float64(rect.Height) - 1.8; span > 0 {
		vertical = math.Max(0, math.Min(1, (y-float64(rect.Min.Y))/span))
	}
	return horizontal, vertical, across - plane - .5
}

// ExitPosition maps a normalized entry position into a destination rectangle.
func ExitPosition(rect Rectangle, horizontal, vertical, depth float64) (x, y, z float64) {
	along := .3 + (float64(rect.Width)-.6)*horizontal
	x, y, z = float64(rect.Min.X)+along, float64(rect.Min.Y)+(float64(rect.Height)-1.8)*vertical, float64(rect.Min.Z)+.5+depth
	if rect.Axis == "z" {
		x, z = float64(rect.Min.X)+.5+depth, float64(rect.Min.Z)+along
	}
	return
}

// SiteSearch incrementally finds a portal location without an unbounded tick workload.
type SiteSearch struct {
	Target                              BlockPos
	Axis                                string
	MinY, MaxY                          int
	cursor                              int
	preferred, fallback                 *BlockPos
	preferredDistance, fallbackDistance float64
}

// Step checks a bounded number of columns in the sixteen-block placement radius.
func (s *SiteSearch) Step(budget int, stateAt func(int, int, int) int32) bool {
	for work := 0; work < budget && s.cursor < 33*33; work++ {
		index := s.cursor
		s.cursor++
		x, z := s.Target.X+index%33-16, s.Target.Z+index/33-16
		for y := s.MaxY; y >= s.MinY; y-- {
			if !portalReplaceable(stateAt(x, y, z)) {
				continue
			}
			top := y
			for y > s.MinY && portalReplaceable(stateAt(x, y-1, z)) {
				y--
			}
			if y+4 > s.MaxY || (top-y > 0 && top-y < 3) {
				continue
			}
			pos := BlockPos{X: x, Y: y, Z: z}
			if !s.canHost(pos, 0, stateAt) {
				continue
			}
			distance := distanceSquared(s.Target, pos)
			if s.canHost(pos, -1, stateAt) && s.canHost(pos, 1, stateAt) {
				if s.preferred == nil || distance < s.preferredDistance {
					p := pos
					s.preferred = &p
					s.preferredDistance = distance
				}
			} else if s.fallback == nil || distance < s.fallbackDistance {
				p := pos
				s.fallback = &p
				s.fallbackDistance = distance
			}
		}
	}
	return s.cursor == 33*33
}

// Build installs the selected frame after all affected columns have been prepared.
func (s *SiteSearch) Build(stateAt func(int, int, int) int32, setBlock func(int, int, int, int32)) (Rectangle, error) {
	if s.cursor < 33*33 {
		return Rectangle{}, fmt.Errorf("portal site search is incomplete")
	}
	origin := s.preferred
	if origin == nil {
		origin = s.fallback
	}
	artificial := origin == nil
	dx, dz := 1, 0
	if s.Axis == "z" {
		dx, dz = 0, 1
	}
	if artificial {
		low, high := max(s.MinY+1, 70), s.MaxY-9
		if high < low {
			return Rectangle{}, fmt.Errorf("dimension cannot host a portal")
		}
		origin = &BlockPos{X: s.Target.X - dx, Y: max(low, min(s.Target.Y, high)), Z: s.Target.Z - dz}
	}
	rect := Rectangle{Min: *origin, Axis: s.Axis, Width: 2, Height: 3}
	// Validate every write first, including artificial support and clearance.
	positions := make(map[BlockPos]int32, 32)
	obsidian := block.DefaultStateID("minecraft:obsidian")
	if artificial {
		for across := -1; across <= 1; across++ {
			for along := 0; along < 2; along++ {
				for y := -1; y < 3; y++ {
					state := int32(0)
					if y < 0 {
						state = obsidian
					}
					positions[BlockPos{X: origin.X + along*dx + across*dz, Y: origin.Y + y, Z: origin.Z + along*dz - across*dx}] = state
				}
			}
		}
	}
	portalState := block.StateID("minecraft:nether_portal", map[string]string{"axis": s.Axis})
	for along := -1; along <= 2; along++ {
		for y := -1; y <= 3; y++ {
			state := obsidian
			if along >= 0 && along < 2 && y >= 0 && y < 3 {
				state = portalState
			}
			positions[BlockPos{X: origin.X + along*dx, Y: origin.Y + y, Z: origin.Z + along*dz}] = state
		}
	}
	for p, state := range positions {
		if state < 0 || stateAt(p.X, p.Y, p.Z) < 0 {
			return Rectangle{}, fmt.Errorf("portal destination contains unavailable blocks")
		}
	}
	// Fixed coordinate order keeps neighbor notifications reproducible.
	for y := origin.Y - 1; y <= origin.Y+3; y++ {
		for x := origin.X - 2; x <= origin.X+3; x++ {
			for z := origin.Z - 2; z <= origin.Z+3; z++ {
				p := BlockPos{X: x, Y: y, Z: z}
				if state, ok := positions[p]; ok {
					setBlock(x, y, z, state)
				}
			}
		}
	}
	return rect, nil
}

// canHost checks solid support and fluid-free clearance for one portal plane.
func (s *SiteSearch) canHost(position BlockPos, across int, stateAt func(int, int, int) int32) bool {
	dx, dz := 1, 0
	if s.Axis == "z" {
		dx, dz = 0, 1
	}
	for along := -1; along <= 2; along++ {
		for y := -1; y <= 3; y++ {
			state := stateAt(position.X+along*dx+across*dz, position.Y+y, position.Z+along*dz-across*dx)
			if y < 0 {
				if state < 0 || !block.IsSolid(state) {
					return false
				}
			} else if !portalReplaceable(state) {
				return false
			}
		}
	}
	return true
}

// portalReplaceable excludes unavailable cells and all fluids from frame placement.
func portalReplaceable(state int32) bool {
	if state < 0 || block.IsWater(state) || block.IsLava(state) || block.PropertiesFromState(state)["waterlogged"] == "true" {
		return false
	}
	switch block.NameFromState(state) {
	case "minecraft:air", "minecraft:cave_air", "minecraft:void_air", "minecraft:fire":
		return true
	}
	return false
}

// distanceSquared ranks portal blocks and creation sites without square roots.
func distanceSquared(a, b BlockPos) float64 {
	dx, dy, dz := float64(a.X-b.X), float64(a.Y-b.Y), float64(a.Z-b.Z)
	return dx*dx + dy*dy + dz*dz
}

// CreateEndPlatform prepares the five-by-five entry platform and standing clearance.
func CreateEndPlatform(stateAt func(int, int, int) int32, setBlock func(int, int, int, int32)) error {
	for x := 98; x <= 102; x++ {
		for z := -2; z <= 2; z++ {
			for y := 48; y <= 51; y++ {
				if stateAt(x, y, z) < 0 {
					return fmt.Errorf("End platform chunks are unavailable")
				}
			}
		}
	}
	obsidian := block.DefaultStateID("minecraft:obsidian")
	for x := 98; x <= 102; x++ {
		for z := -2; z <= 2; z++ {
			for y := 48; y <= 51; y++ {
				state := int32(0)
				if y == 48 {
					state = obsidian
				}
				setBlock(x, y, z, state)
			}
		}
	}
	return nil
}
