package world

import (
	"math"
	"sync"

	"github.com/vitismc/vitis/internal/protocol"
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
)

type BorderBroadcaster interface {
	Send(pkt protocol.Packet) error
}

type WorldBorder struct {
	mu sync.RWMutex

	centerX float64
	centerZ float64

	diameter    float64
	oldDiameter float64

	lerpTarget   float64
	lerpSpeed    int64
	lerpTicksLeft int64

	warningBlocks int32
	warningTime   int32

	damagePerBlock float64
	damageBuffer   float64

	portalBound int32

	broadcast func(protocol.Packet)
}

func NewWorldBorder(broadcast func(protocol.Packet)) *WorldBorder {
	return &WorldBorder{
		centerX:        0,
		centerZ:        0,
		diameter:       60000000,
		oldDiameter:    60000000,
		lerpTarget:     60000000,
		warningBlocks:  5,
		warningTime:    15,
		damagePerBlock: 0.2,
		damageBuffer:   5.0,
		portalBound:    29999984,
		broadcast:      broadcast,
	}
}

func (wb *WorldBorder) InitPacket() *playpacket.InitializeWorldBorder {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	return &playpacket.InitializeWorldBorder{
		X:                      wb.centerX,
		Z:                      wb.centerZ,
		OldDiameter:            wb.diameter,
		NewDiameter:            wb.lerpTarget,
		Speed:                  wb.lerpSpeed,
		PortalTeleportBoundary: wb.portalBound,
		WarningBlocks:          wb.warningBlocks,
		WarningTime:            wb.warningTime,
	}
}

func (wb *WorldBorder) SetCenter(x, z float64) {
	wb.mu.Lock()
	wb.centerX = x
	wb.centerZ = z
	wb.mu.Unlock()
	if wb.broadcast != nil {
		wb.broadcast(&playpacket.SetBorderCenter{X: x, Z: z})
	}
}

func (wb *WorldBorder) Center() (float64, float64) {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	return wb.centerX, wb.centerZ
}

func (wb *WorldBorder) SetSize(diameter float64) {
	wb.mu.Lock()
	wb.diameter = diameter
	wb.oldDiameter = diameter
	wb.lerpTarget = diameter
	wb.lerpSpeed = 0
	wb.lerpTicksLeft = 0
	wb.mu.Unlock()
	if wb.broadcast != nil {
		wb.broadcast(&playpacket.SetBorderSize{Diameter: diameter})
	}
}

func (wb *WorldBorder) LerpSize(target float64, millis int64) {
	wb.mu.Lock()
	wb.oldDiameter = wb.diameter
	wb.lerpTarget = target
	wb.lerpSpeed = millis
	if millis > 0 {
		wb.lerpTicksLeft = millis / 50
	} else {
		wb.diameter = target
		wb.lerpTicksLeft = 0
	}
	wb.mu.Unlock()
	if wb.broadcast != nil {
		wb.broadcast(&playpacket.InitializeWorldBorder{
			X:                      wb.centerX,
			Z:                      wb.centerZ,
			OldDiameter:            wb.oldDiameter,
			NewDiameter:            target,
			Speed:                  millis,
			PortalTeleportBoundary: wb.portalBound,
			WarningBlocks:          wb.warningBlocks,
			WarningTime:            wb.warningTime,
		})
	}
}

func (wb *WorldBorder) Diameter() float64 {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	return wb.diameter
}

func (wb *WorldBorder) SetWarningBlocks(blocks int32) {
	wb.mu.Lock()
	wb.warningBlocks = blocks
	wb.mu.Unlock()
	if wb.broadcast != nil {
		wb.broadcast(&playpacket.SetBorderWarningDistance{WarningBlocks: blocks})
	}
}

func (wb *WorldBorder) SetWarningTime(time int32) {
	wb.mu.Lock()
	wb.warningTime = time
	wb.mu.Unlock()
	if wb.broadcast != nil {
		wb.broadcast(&playpacket.SetBorderWarningDelay{WarningTime: time})
	}
}

func (wb *WorldBorder) WarningBlocks() int32 {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	return wb.warningBlocks
}

func (wb *WorldBorder) WarningTime() int32 {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	return wb.warningTime
}

func (wb *WorldBorder) Tick() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	if wb.lerpTicksLeft <= 0 {
		return
	}
	wb.lerpTicksLeft--
	if wb.lerpTicksLeft <= 0 {
		wb.diameter = wb.lerpTarget
		wb.oldDiameter = wb.lerpTarget
		wb.lerpSpeed = 0
	} else {
		totalTicks := float64(wb.lerpSpeed) / 50.0
		progress := 1.0 - float64(wb.lerpTicksLeft)/totalTicks
		wb.diameter = wb.oldDiameter + (wb.lerpTarget-wb.oldDiameter)*progress
	}
}

func (wb *WorldBorder) IsOutside(x, z float64) bool {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	halfSize := wb.diameter / 2.0
	return x < wb.centerX-halfSize || x > wb.centerX+halfSize ||
		z < wb.centerZ-halfSize || z > wb.centerZ+halfSize
}

func (wb *WorldBorder) DistanceFromEdge(x, z float64) float64 {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	halfSize := wb.diameter / 2.0
	dists := [4]float64{
		x - (wb.centerX - halfSize),
		(wb.centerX + halfSize) - x,
		z - (wb.centerZ - halfSize),
		(wb.centerZ + halfSize) - z,
	}
	minDist := dists[0]
	for _, d := range dists[1:] {
		if d < minDist {
			minDist = d
		}
	}
	return minDist
}

func (wb *WorldBorder) ComputeDamage(x, z float64) float64 {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	halfSize := wb.diameter / 2.0
	dists := [4]float64{
		x - (wb.centerX - halfSize),
		(wb.centerX + halfSize) - x,
		z - (wb.centerZ - halfSize),
		(wb.centerZ + halfSize) - z,
	}
	minDist := dists[0]
	for _, d := range dists[1:] {
		if d < minDist {
			minDist = d
		}
	}
	if minDist >= 0 {
		return 0
	}
	penetration := math.Abs(minDist) - wb.damageBuffer
	if penetration <= 0 {
		return 0
	}
	return penetration * wb.damagePerBlock
}
