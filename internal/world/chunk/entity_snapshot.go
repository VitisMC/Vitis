package chunk

import "bytes"

// EntityNBT returns a detached entity-region payload on the owning world thread.
func (c *Chunk) EntityNBT() ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	return append([]byte(nil), c.entityNBT...), c.entitySaveErr
}

// LoadEntityNBT attaches an immutable worker result without making it dirty.
func (c *Chunk) LoadEntityNBT(data []byte) { c.entityNBT = append([]byte(nil), data...) }

// SetEntityNBT replaces a runtime snapshot and advances the chunk's save revision.
func (c *Chunk) SetEntityNBT(data []byte) {
	if c.entitySaveErr != nil || !bytes.Equal(c.entityNBT, data) {
		c.entityNBT = append([]byte(nil), data...)
		c.entitySaveErr = nil
		c.revision.Add(1)
		c.dirty.Store(true)
	}
}

// SetEntitySaveError prevents persistence and eviction of an incomplete snapshot.
func (c *Chunk) SetEntitySaveError(err error) {
	c.entitySaveErr = err
	if err != nil {
		c.revision.Add(1)
		c.dirty.Store(true)
	}
}
