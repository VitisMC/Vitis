package nbt

// Compound is an ordered collection of named NBT tags.
type Compound struct {
	entries []entry
}

type entry struct {
	name  string
	tagID byte
	value interface{}
}

// NewCompound creates an empty compound tag.
func NewCompound() *Compound {
	return &Compound{}
}

// PutByte adds a byte tag.
func (c *Compound) PutByte(name string, v int8) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagByte, value: v})
	return c
}

// PutShort adds a short tag.
func (c *Compound) PutShort(name string, v int16) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagShort, value: v})
	return c
}

// PutInt adds an int tag.
func (c *Compound) PutInt(name string, v int32) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagInt, value: v})
	return c
}

// PutLong adds a long tag.
func (c *Compound) PutLong(name string, v int64) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagLong, value: v})
	return c
}

// PutFloat adds a float tag.
func (c *Compound) PutFloat(name string, v float32) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagFloat, value: v})
	return c
}

// PutDouble adds a double tag.
func (c *Compound) PutDouble(name string, v float64) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagDouble, value: v})
	return c
}

// PutString adds a string tag.
func (c *Compound) PutString(name string, v string) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagString, value: v})
	return c
}

// PutByteArray adds a byte array tag.
func (c *Compound) PutByteArray(name string, v []byte) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagByteArray, value: v})
	return c
}

// PutIntArray adds an int array tag.
func (c *Compound) PutIntArray(name string, v []int32) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagIntArray, value: v})
	return c
}

// PutLongArray adds a long array tag.
func (c *Compound) PutLongArray(name string, v []int64) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagLongArray, value: v})
	return c
}

// PutCompound adds a nested compound tag.
func (c *Compound) PutCompound(name string, v *Compound) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagCompound, value: v})
	return c
}

// PutList adds a list tag.
func (c *Compound) PutList(name string, v *List) *Compound {
	c.entries = append(c.entries, entry{name: name, tagID: TagList, value: v})
	return c
}

// PutBool adds a boolean as a byte tag (0x00 or 0x01).
func (c *Compound) PutBool(name string, v bool) *Compound {
	var b int8
	if v {
		b = 1
	}
	return c.PutByte(name, b)
}

// Entries returns the ordered entries for encoding.
func (c *Compound) Entries() []entry {
	return c.entries
}

// Get returns the value and tag ID for a named entry, or nil if not found.
func (c *Compound) Get(name string) (interface{}, byte, bool) {
	for _, e := range c.entries {
		if e.name == name {
			return e.value, e.tagID, true
		}
	}
	return nil, 0, false
}

// GetInt returns an int32 value by name.
func (c *Compound) GetInt(name string) (int32, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagInt {
		return 0, false
	}
	val, ok := v.(int32)
	return val, ok
}

// GetLong returns an int64 value by name.
func (c *Compound) GetLong(name string) (int64, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagLong {
		return 0, false
	}
	val, ok := v.(int64)
	return val, ok
}

// GetString returns a string value by name.
func (c *Compound) GetString(name string) (string, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagString {
		return "", false
	}
	val, ok := v.(string)
	return val, ok
}

// GetFloat returns a float32 value by name.
func (c *Compound) GetFloat(name string) (float32, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagFloat {
		return 0, false
	}
	val, ok := v.(float32)
	return val, ok
}

// GetDouble returns a float64 value by name.
func (c *Compound) GetDouble(name string) (float64, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagDouble {
		return 0, false
	}
	val, ok := v.(float64)
	return val, ok
}

// GetByte returns an int8 value by name.
func (c *Compound) GetByte(name string) (int8, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagByte {
		return 0, false
	}
	val, ok := v.(int8)
	return val, ok
}

// GetBool returns a bool from a byte tag by name.
func (c *Compound) GetBool(name string) (bool, bool) {
	v, ok := c.GetByte(name)
	if !ok {
		return false, false
	}
	return v != 0, true
}

// GetList returns a list tag by name.
func (c *Compound) GetList(name string) (*List, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagList {
		return nil, false
	}
	val, ok := v.(*List)
	return val, ok
}

// GetCompound returns a nested compound by name.
func (c *Compound) GetCompound(name string) (*Compound, bool) {
	v, tid, ok := c.Get(name)
	if !ok || tid != TagCompound {
		return nil, false
	}
	val, ok := v.(*Compound)
	return val, ok
}

// GetAllStrings returns all string-typed entries as a name→value map.
func (c *Compound) GetAllStrings() map[string]string {
	result := make(map[string]string, len(c.entries))
	for _, e := range c.entries {
		if e.tagID == TagString {
			if val, ok := e.value.(string); ok {
				result[e.name] = val
			}
		}
	}
	return result
}
