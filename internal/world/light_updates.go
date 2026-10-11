package world

import (
	playpacket "github.com/vitismc/vitis/internal/protocol/packets/play"
	"github.com/vitismc/vitis/internal/world/chunk"
	"github.com/vitismc/vitis/internal/world/streaming"
)

// bindChunkLight applies dimension policy and queues loaded or modified light sources.
func (w *World) bindChunkLight(c *chunk.Chunk) {
	if c.IsPregenerationOnly() {
		return
	}
	if w.lightChanges == nil {
		w.lightChanges = make(map[int64]struct{})
	}
	c.BindLightPolicy(w.dimension != "minecraft:the_nether" && w.dimension != "minecraft:the_end", func(changed *chunk.Chunk) {
		w.lightChanges[changed.Key()] = struct{}{}
	})
}

// tickLighting rebuilds a bounded set of source neighborhoods and notifies their observers.
func (w *World) tickLighting() {
	for work := 0; work < 2 && len(w.lightChanges) != 0; work++ {
		var source int64
		for source = range w.lightChanges {
			break
		}
		delete(w.lightChanges, source)
		x, z := int32(uint64(source)>>32), int32(source)
		if _, ok := w.chunks.PeekLoaded(x, z); !ok {
			continue
		}
		var columns [9]*chunk.Chunk
		count := 0
		for dx := int32(-1); dx <= 1; dx++ {
			for dz := int32(-1); dz <= 1; dz++ {
				c, ok := w.chunks.PeekLoaded(x+dx, z+dz)
				if ok && !c.IsPregenerationOnly() {
					c.RefreshLight()
					columns[count] = c
					count++
				}
			}
		}
		// Every seam crossing attenuates light; fifteen passes cover its maximum reach.
		for pass := 0; pass < 15; pass++ {
			changed := false
			for _, c := range columns[:count] {
				data := c.Data()
				for _, sky := range []bool{false, true} {
					if sky && (w.dimension == "minecraft:the_nether" || w.dimension == "minecraft:the_end") {
						continue
					}
					incoming := func(lx, y, lz int) uint8 {
						ax, az := int(c.X())*16+lx, int(c.Z())*16+lz
						neighbor, ok := w.chunks.PeekLoaded(int32(ax>>4), int32(az>>4))
						if !ok || neighbor.IsPregenerationOnly() {
							return 0
						}
						neighbor.EnsureLight()
						if sky {
							return neighbor.Data().LightEngine.SkyLight(ax&15, y, az&15)
						}
						return neighbor.Data().LightEngine.BlockLight(ax&15, y, az&15)
					}
					changed = data.LightEngine.ImportBorders(data, sky, incoming) || changed
				}
			}
			if !changed {
				break
			}
		}
		for _, c := range columns[:count] {
			c.MarkDirty()
			w.sendLightUpdate(c)
		}
	}
}

// sendLightUpdate sends immutable light payloads only to clients observing the chunk.
func (w *World) sendLightUpdate(c *chunk.Chunk) {
	var packet *playpacket.UpdateLight
	for _, player := range w.tracker.Players() {
		streamer, ok := w.streaming.GetStreamer(player.ID())
		if player.Removed() || !ok || !streamer.View().IsLoaded(streaming.ChunkPos{X: c.X(), Z: c.Z()}) || player.Session() == nil {
			continue
		}
		if packet == nil {
			packet = &playpacket.UpdateLight{ChunkX: c.X(), ChunkZ: c.Z(), Data: c.Data().EncodeLightPayload()}
		}
		_ = player.Session().Send(packet)
	}
}
