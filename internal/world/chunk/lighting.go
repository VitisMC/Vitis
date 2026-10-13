package chunk

import "github.com/vitismc/vitis/internal/world/lighting"

// BindLightPolicy attaches dimension lighting and a tick-owned mutation observer once.
func (c *Chunk) BindLightPolicy(hasSky bool, changed func(*Chunk)) {
	if c == nil || c.lightObserver != nil {
		return
	}
	c.noSkyLight = !hasSky
	c.lightObserver = changed
	c.lightDirty.Store(true)
	if changed != nil {
		changed(c)
	}
}

// RefreshLight resets local sources before a region rebuild, including light removal.
func (c *Chunk) RefreshLight() {
	if c == nil || c.Data() == nil {
		return
	}
	data := c.Data()
	engine := lighting.NewSparseEngine(len(data.Sections), data.MinY(), data.MaxY())
	engine.ComputeBlockLight(data)
	if !c.noSkyLight {
		engine.ComputeSkyLight(data)
	}
	data.LightEngine = engine
	c.lightDirty.Store(false)
}
