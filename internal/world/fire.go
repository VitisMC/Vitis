package world

import (
	"strconv"
	"strings"

	"github.com/vitismc/vitis/internal/block"
	"github.com/vitismc/vitis/internal/world/tick"
)

// tickFire applies bounded support, aging and spread updates on the world tick.
func (w *World) tickFire(x, y, z int, stateID int32) {
	if !w.fireTick.Load() {
		w.tickScheduler.Schedule(x, y, z, w.tick.Load(), 30, tick.PriorityNormal, tick.TickTypeBlock, stateID)
		return
	}
	name := block.NameFromState(stateID)
	below := w.getBlockAt(x, y-1, z)
	if name == "minecraft:soul_fire" {
		support := block.NameFromState(below)
		if support != "minecraft:soul_sand" && support != "minecraft:soul_soil" {
			w.setBlockAt(x, y, z, 0)
			w.broadcastBlockUpdate(x, y, z, 0)
		}
		return
	}
	neighbors := [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}, {0, 1, 0}}
	for _, offset := range [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}, {0, 1, 0}, {0, -1, 0}} {
		nx, ny, nz := x+offset[0], y+offset[1], z+offset[2]
		if block.NameFromState(w.getBlockAt(nx, ny, nz)) == "minecraft:tnt" {
			w.IgniteTNT(nx, ny, nz, 0)
		}
	}
	fuel := isFlammable(below)
	for _, offset := range neighbors {
		fuel = fuel || isFlammable(w.getBlockAt(x+offset[0], y+offset[1], z+offset[2]))
	}
	if !block.IsSolid(below) && !fuel {
		w.setBlockAt(x, y, z, 0)
		w.broadcastBlockUpdate(x, y, z, 0)
		return
	}
	props := block.PropertiesFromState(stateID)
	age, _ := strconv.Atoi(props["age"])
	if age >= 15 && !fuel {
		w.setBlockAt(x, y, z, 0)
		w.broadcastBlockUpdate(x, y, z, 0)
		return
	}
	if age >= 15 && fuel && (uint64(x*31+z*17)+w.tick.Load())%7 == 0 {
		for _, offset := range neighbors {
			nx, ny, nz := x+offset[0], y+offset[1], z+offset[2]
			if isFlammable(w.getBlockAt(nx, ny, nz)) {
				w.setBlockAt(nx, ny, nz, 0)
				w.broadcastBlockUpdate(nx, ny, nz, 0)
				break
			}
		}
	}
	if age < 15 {
		props["age"] = strconv.Itoa(age + 1)
		if next := block.StateID(name, props); next >= 0 {
			stateID = next
			w.setBlockAt(x, y, z, stateID)
			w.broadcastBlockUpdate(x, y, z, stateID)
		}
	}
	if fuel {
		for _, offset := range neighbors[:4] {
			nx, nz := x+offset[0], z+offset[2]
			if !block.IsAir(w.getBlockAt(nx, y, nz)) || !isFlammable(w.getBlockAt(nx, y-1, nz)) {
				continue
			}
			if (uint64(nx*31+nz*17+age)+w.tick.Load())%7 != 0 {
				continue
			}
			next := block.DefaultStateID("minecraft:fire")
			if next < 0 {
				break
			}
			w.setBlockAt(nx, y, nz, next)
			w.broadcastBlockUpdate(nx, y, nz, next)
			w.tickScheduler.Schedule(nx, y, nz, w.tick.Load(), 30, tick.PriorityNormal, tick.TickTypeBlock, next)
			break
		}
	}
	w.tickScheduler.Schedule(x, y, z, w.tick.Load(), 30, tick.PriorityNormal, tick.TickTypeBlock, stateID)
}

func isFlammable(stateID int32) bool {
	name := block.NameFromState(stateID)
	for _, suffix := range []string{"_planks", "_log", "_wood", "_leaves", "_wool", "_carpet", "_fence", "_fence_gate", "_stairs", "_slab"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	switch name {
	case "minecraft:bookshelf", "minecraft:hay_block", "minecraft:bamboo_block", "minecraft:dried_kelp_block":
		return true
	}
	return false
}
