package world

import "github.com/vitismc/vitis/internal/block/behavior"

// ExplodeBed destroys a complete bed before its fixed-power dimension explosion.
func (w *World) ExplodeBed(x, y, z int) bool {
	state := w.getBlockAt(x, y, z)
	if !behavior.IsBed(state) {
		return false
	}
	ox, oy, oz := behavior.BedOtherHalfPos(state, x, y, z)
	if w.getBlockAt(ox, oy, oz) < 0 {
		return false
	}
	w.setBlockAt(x, y, z, 0)
	w.broadcastBlockUpdate(x, y, z, 0)
	if behavior.IsBed(w.getBlockAt(ox, oy, oz)) {
		w.setBlockAt(ox, oy, oz, 0)
		w.broadcastBlockUpdate(ox, oy, oz, 0)
	}
	w.doExplosion(float64(x)+.5, float64(y)+.5, float64(z)+.5, 5)
	return true
}
