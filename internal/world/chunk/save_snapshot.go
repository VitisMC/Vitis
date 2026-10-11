package chunk

import (
	"fmt"

	"github.com/vitismc/vitis/internal/nbt"
)

// SnapshotForSave captures chunk state on the world thread for background encoding.
// The returned chunk owns its palettes and NBT, including captured block entities.
func SnapshotForSave(c *Chunk) (*Chunk, error) {
	if c == nil || c.Data() == nil {
		return nil, fmt.Errorf("snapshot chunk: missing section data")
	}
	entities, err := c.EntityNBT()
	if err != nil {
		return nil, fmt.Errorf("snapshot entities: %w", err)
	}
	source := c.Data()
	data := source.CloneRaw()
	root := source.PersistentNBT()
	if root != nil || source.BlockEntities != nil || c.tickScheduler != nil {
		if root == nil {
			root = nbt.NewCompound()
		}
		encodeBlockEntitiesNBT(root, source)
		if err := encodeScheduledTicksNBT(root, c); err != nil {
			return nil, err
		}
		data.SetPersistentNBT(root)
	}
	snapshot := NewLoading(c.X(), c.Z())
	snapshot.SetData(data)
	snapshot.inhabitedTime.Store(c.InhabitedTime())
	snapshot.LoadEntityNBT(entities)
	return snapshot, nil
}
