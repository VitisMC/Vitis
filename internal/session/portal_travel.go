package session

import (
	"fmt"
	"math"
	"strconv"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/block/behavior"
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/physics"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/portal"
)

// portalPreparation owns bounded portal lookup and creation work for one pending transfer.
type portalPreparation struct {
	kind, axis                  string
	horizontal, vertical, depth float64
	target                      portal.BlockPos
	radius                      int
	minX, minZ, maxX, maxZ      int32
	cursor                      int
	candidate                   *portal.BlockPos
	distance                    float64
	creation                    *portal.SiteSearch
	complete                    bool
}

// tickPortalContacts detects contact on the tick and respects delay and cooldown rules.
func (t *DimensionTransfers) tickPortalContacts() {
	for _, online := range t.players.Players() {
		s, ok := online.Session.(*DefaultSession)
		if !ok || s.Context().Err() != nil {
			continue
		}
		p, ok := s.Player().(*entity.Player)
		if !ok || p == nil || p.Living().IsDead() || !p.HasClientLoaded() {
			continue
		}
		if online.portalBinding != s.world.Load() {
			online.portalBinding = s.world.Load()
			online.portalTime = 0
			online.portalKind = ""
		}
		if online.PortalCooldown > 0 {
			online.PortalCooldown--
		}
		entry := t.worlds[SessionDimension(s)]
		kind, pos := portalContact(entry.Accessor, p.Position())
		if kind == "" {
			online.portalTime = max(online.portalTime-4, 0)
			continue
		}
		if online.PortalCooldown > 0 {
			online.PortalCooldown = 300
			continue
		}
		if t.pending[online.UUID] != nil || s.hasPendingTeleport() {
			continue
		}
		if online.portalKind != kind {
			online.portalTime = 0
			online.portalKind = kind
		}
		delay := int32(0)
		if kind == "minecraft:nether_portal" {
			delay = 80
			rule := "playersNetherPortalDefaultDelay"
			if online.GameMode == 1 {
				rule = "playersNetherPortalCreativeDelay"
				delay = 1
			}
			if entry.Config.GameRule != nil {
				if value, err := strconv.ParseInt(entry.Config.GameRule(rule), 10, 32); err == nil {
					delay = max(0, int32(value))
				}
			}
		}
		elapsed := online.portalTime
		if online.portalTime < math.MaxInt32 {
			online.portalTime++
		}
		if elapsed < delay {
			continue
		}
		if err := t.requestPortal(s, kind, pos); err != nil {
			online.PortalCooldown = 300
			_ = s.Send(&play.SystemChatMessage{Text: "Portal destination unavailable", Overlay: true})
		}
	}
}

// portalContact tests the player's volume against actual portal trigger shapes.
func portalContact(world WorldAccessor, position entity.Vec3) (string, portal.BlockPos) {
	if world == nil {
		return "", portal.BlockPos{}
	}
	box := physics.PlayerDimensions().MakeBoundingBox(position.X, position.Y, position.Z)
	for x := int(math.Floor(box.MinX)); x < int(math.Ceil(box.MaxX)); x++ {
		for y := int(math.Floor(box.MinY)); y < int(math.Ceil(box.MaxY)); y++ {
			for z := int(math.Floor(box.MinZ)); z < int(math.Ceil(box.MaxZ)); z++ {
				state := world.GetBlock(x, y, z)
				kind := block.NameFromState(state)
				trigger := physics.AABB{MinX: float64(x), MaxX: float64(x + 1), MinY: float64(y), MaxY: float64(y + 1), MinZ: float64(z), MaxZ: float64(z + 1)}
				switch kind {
				case "minecraft:nether_portal":
					if block.PropertiesFromState(state)["axis"] == "x" {
						trigger.MinZ += .375
						trigger.MaxZ -= .375
					} else {
						trigger.MinX += .375
						trigger.MaxX -= .375
					}
				case "minecraft:end_portal":
					trigger.MinY += .375
					trigger.MaxY -= .25
				default:
					continue
				}
				if box.MinX < trigger.MaxX && box.MaxX > trigger.MinX && box.MinY < trigger.MaxY && box.MaxY > trigger.MinY && box.MinZ < trigger.MaxZ && box.MaxZ > trigger.MinZ {
					return kind, portal.BlockPos{X: x, Y: y, Z: z}
				}
			}
		}
	}
	return "", portal.BlockPos{}
}

// requestPortal records the entry geometry before asynchronous destination preparation.
func (t *DimensionTransfers) requestPortal(s *DefaultSession, kind string, entryPosition portal.BlockPos) error {
	p := s.Player().(*entity.Player)
	position, rotation := p.Position(), p.Rotation()
	if kind == "minecraft:end_portal" {
		if SessionDimension(s) == "minecraft:the_end" {
			return t.requestPersonalSpawn(s, false)
		}
		if err := t.request(s, "minecraft:the_end", entity.Vec3{X: 100.5, Y: 49, Z: .5}, entity.Vec2{X: 90, Y: rotation.Y}, false); err != nil {
			return err
		}
		t.pending[p.UUID()].portal = &portalPreparation{kind: kind}
		return nil
	}
	destination := "minecraft:the_nether"
	radius := 16
	if SessionDimension(s) == "minecraft:the_nether" {
		destination = "minecraft:overworld"
		position.X, position.Z = portal.NetherToOverworld(position.X), portal.NetherToOverworld(position.Z)
		radius = 128
	} else {
		position.X, position.Z = portal.OverworldToNether(position.X), portal.OverworldToNether(position.Z)
	}
	source := t.worlds[SessionDimension(s)]
	rect, ok := portal.RectangleAt(entryPosition, source.Accessor.GetBlock)
	if !ok {
		return fmt.Errorf("entry portal is incomplete")
	}
	h, v, d := portal.RelativePosition(rect, p.Position().X, p.Position().Y, p.Position().Z)
	if err := t.request(s, destination, position, rotation, false); err != nil {
		return err
	}
	pending := t.pending[p.UUID()]
	lookup := &portalPreparation{kind: kind, axis: rect.Axis, horizontal: h, vertical: v, depth: d, radius: radius, target: portal.BlockPos{X: int(math.Floor(position.X)), Y: int(math.Floor(position.Y)), Z: int(math.Floor(position.Z))}}
	lookup.minX, lookup.minZ = int32((lookup.target.X-radius)>>4), int32((lookup.target.Z-radius)>>4)
	lookup.maxX, lookup.maxZ = int32((lookup.target.X+radius)>>4), int32((lookup.target.Z+radius)>>4)
	pending.portal = lookup
	if err := pending.destination.SetChunkTicket(chunk.Ticket{Kind: chunk.TicketView, Owner: s.ID() | (uint64(1) << 63), X: chunkCoord(position.X), Z: chunkCoord(position.Z), Radius: int32(radius/16 + 2), ExpiresAt: pending.destination.CurrentTick() + transferTimeoutTicks}); err != nil {
		t.cancel(p.UUID(), pending)
		return err
	}
	return nil
}

// preparePortal scans a bounded number of loaded chunks and prepares an exit geometry.
func (t *DimensionTransfers) preparePortal(transfer *dimensionTransfer, entry PlayWorld) (bool, error) {
	job := transfer.portal
	if job.complete {
		return true, nil
	}
	update := func(x, y, z int, state int32) {
		oldState := entry.Accessor.GetBlock(x, y, z)
		if oldState == state {
			return
		}
		players := t.players.ForDimension(entry.Dimension)
		if store := entry.Accessor.BlockEntities(); store != nil {
			for _, stack := range store.RemoveContainer(x, y, z) {
				spawnInventoryItem(players, entry.Accessor, float64(x)+.5, float64(y)+.5, float64(z)+.5, stack)
			}
		}
		if job.kind == "minecraft:end_portal" && oldState > 0 {
			for _, drop := range behavior.GetByState(oldState).OnBreak(&behavior.Context{X: x, Y: y, Z: z, StateID: oldState}) {
				spawnCommandItem(players, entry.Accessor, float64(x)+.5, float64(y)+.5, float64(z)+.5, drop.ItemID, drop.Count)
			}
		}
		entry.Accessor.SetBlock(x, y, z, state)
		entry.Accessor.NotifyNeighbors(x, y, z)
		t.players.ForDimension(entry.Dimension).Broadcast(&play.BlockUpdate{Position: play.BlockPos{X: int32(x), Y: int32(y), Z: int32(z)}, BlockID: state})
	}
	if job.kind == "minecraft:end_portal" {
		if err := portal.CreateEndPlatform(entry.Accessor.GetBlock, update); err != nil {
			return false, err
		}
		job.complete = true
		return true, nil
	}
	width := job.maxX - job.minX + 1
	count := int(width * (job.maxZ - job.minZ + 1))
	for work := 0; work < 8 && t.portalWork > 0 && job.cursor < count; work++ {
		x, z := job.minX+int32(job.cursor)%width, job.minZ+int32(job.cursor)/width
		if x < -1875000 || x >= 1875000 || z < -1875000 || z >= 1875000 {
			job.cursor++
			continue
		}
		c, loaded := entry.Accessor.GetChunk(x, z)
		if !loaded {
			transfer.destination.ChunkManager().RequestLoad(x, z)
			return false, nil
		}
		job.scan(c)
		job.cursor++
		t.portalWork--
	}
	if job.cursor < count {
		return false, nil
	}
	var rect portal.Rectangle
	if job.candidate != nil {
		if err := prepareRespawnChunks(entry.Accessor, float64(job.candidate.X), float64(job.candidate.Z)); err != nil {
			return false, nil
		}
		var ok bool
		rect, ok = portal.RectangleAt(*job.candidate, entry.Accessor.GetBlock)
		if !ok {
			return false, fmt.Errorf("destination portal changed during preparation")
		}
	} else {
		if job.creation == nil {
			c, ok := entry.Accessor.GetChunk(chunkCoord(transfer.position.X), chunkCoord(transfer.position.Z))
			if !ok {
				return false, nil
			}
			maxY := c.Data().MaxY()
			if entry.Dimension == "minecraft:the_nether" {
				maxY = min(maxY, c.Data().MinY()+127)
			}
			job.creation = &portal.SiteSearch{Target: job.target, Axis: job.axis, MinY: c.Data().MinY(), MaxY: maxY}
		}
		// Placement only examines sixteen blocks; require its complete dependency area.
		cx, cz := chunkCoord(transfer.position.X), chunkCoord(transfer.position.Z)
		for dx := int32(-2); dx <= 2; dx++ {
			for dz := int32(-2); dz <= 2; dz++ {
				if cx+dx < -1875000 || cx+dx >= 1875000 || cz+dz < -1875000 || cz+dz >= 1875000 {
					continue
				}
				if _, ok := entry.Accessor.GetChunk(cx+dx, cz+dz); !ok {
					transfer.destination.ChunkManager().RequestLoad(cx+dx, cz+dz)
					return false, nil
				}
			}
		}
		budget := min(8, t.portalWork)
		if budget == 0 {
			return false, nil
		}
		t.portalWork -= budget
		if !job.creation.Step(budget, entry.Accessor.GetBlock) {
			return false, nil
		}
		var err error
		rect, err = job.creation.Build(entry.Accessor.GetBlock, update)
		if err != nil {
			return false, err
		}
	}
	transfer.position.X, transfer.position.Y, transfer.position.Z = portal.ExitPosition(rect, job.horizontal, job.vertical, job.depth)
	if rect.Axis != job.axis {
		transfer.rotation.X += 90
	}
	job.complete = true
	return true, nil
}

// scan records the nearest portal block in one loaded chunk, skipping irrelevant palettes.
func (job *portalPreparation) scan(c *chunk.Chunk) {
	data := c.Data()
	for index := 0; index < data.NumSections(); index++ {
		palette := data.BlockPalette(index)
		relevant := palette == nil
		for _, state := range palette {
			if block.NameFromState(state) == "minecraft:nether_portal" {
				relevant = true
				break
			}
		}
		if !relevant {
			continue
		}
		for y := data.MinY() + index*16; y < data.MinY()+(index+1)*16; y++ {
			for x := int(c.X()) * 16; x < int(c.X())*16+16; x++ {
				for z := int(c.Z()) * 16; z < int(c.Z())*16+16; z++ {
					if absInt(x-job.target.X) > job.radius || absInt(z-job.target.Z) > job.radius || block.NameFromState(data.GetBlock(x, y, z)) != "minecraft:nether_portal" {
						continue
					}
					dx, dy, dz := float64(x-job.target.X), float64(y-job.target.Y), float64(z-job.target.Z)
					distance := dx*dx + dy*dy + dz*dz
					if job.candidate == nil || distance < job.distance || distance == job.distance && (y < job.candidate.Y || y == job.candidate.Y && (x < job.candidate.X || x == job.candidate.X && z < job.candidate.Z)) {
						p := portal.BlockPos{X: x, Y: y, Z: z}
						job.candidate = &p
						job.distance = distance
					}
				}
			}
		}
	}
}
