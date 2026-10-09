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

// Keys returns the names of all tags in insertion order.
func (c *Compound) Keys() []string {
	if c == nil {
		return nil
	}
	keys := make([]string, len(c.entries))
	for i, e := range c.entries {
		keys[i] = e.name
	}
	return keys
}

// PutTag inserts a typed tag and owns a deep copy of its value.
func (c *Compound) PutTag(name string, tagID byte, value any) *Compound {
	return c.put(name, tagID, cloneValue(value))
}

// NewCompound creates an empty compound tag.
func NewCompound() *Compound {
	return &Compound{}
}

// Clone returns a deep copy of the compound.
func (c *Compound) Clone() *Compound {
	if c == nil {
		return nil
	}
	clone := NewCompound()
	clone.entries = make([]entry, 0, len(c.entries))
	for _, e := range c.entries {
		clone.entries = append(clone.entries, entry{name: e.name, tagID: e.tagID, value: cloneValue(e.value)})
	}
	return clone
}

func cloneValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case *Compound:
		return typed.Clone()
	case *List:
		return typed.Clone()
	case []byte:
		return append([]byte(nil), typed...)
	case []int32:
		return append([]int32(nil), typed...)
	case []int64:
		return append([]int64(nil), typed...)
	default:
		return typed
	}
}

func (c *Compound) put(name string, tagID byte, value interface{}) *Compound {
	for i := range c.entries {
		if c.entries[i].name == name {
			c.entries[i] = entry{name: name, tagID: tagID, value: value}
			return c
		}
	}
	c.entries = append(c.entries, entry{name: name, tagID: tagID, value: value})
	return c
}

// Delete removes a named tag while preserving the order of all other tags.
func (c *Compound) Delete(name string) {
	if c == nil {
		return
	}
	for i := range c.entries {
		if c.entries[i].name == name {
			copy(c.entries[i:], c.entries[i+1:])
			c.entries[len(c.entries)-1] = entry{}
			c.entries = c.entries[:len(c.entries)-1]
			return
		}
	}
}

// PutByte adds a byte tag.
func (c *Compound) PutByte(name string, v int8) *Compound {
	return c.put(name, TagByte, v)
}

// PutShort adds a short tag.
func (c *Compound) PutShort(name string, v int16) *Compound {
	return c.put(name, TagShort, v)
}

// PutInt adds an int tag.
func (c *Compound) PutInt(name string, v int32) *Compound {
	return c.put(name, TagInt, v)
}

// PutLong adds a long tag.
func (c *Compound) PutLong(name string, v int64) *Compound {
	return c.put(name, TagLong, v)
}

// PutFloat adds a float tag.
func (c *Compound) PutFloat(name string, v float32) *Compound {
	return c.put(name, TagFloat, v)
}

// PutDouble adds a double tag.
func (c *Compound) PutDouble(name string, v float64) *Compound {
	return c.put(name, TagDouble, v)
}

// PutString adds a string tag.
func (c *Compound) PutString(name string, v string) *Compound {
	return c.put(name, TagString, v)
}

// PutByteArray adds a byte array tag.
func (c *Compound) PutByteArray(name string, v []byte) *Compound {
	return c.put(name, TagByteArray, v)
}

// PutIntArray adds an int array tag.
func (c *Compound) PutIntArray(name string, v []int32) *Compound {
	return c.put(name, TagIntArray, v)
}

// PutLongArray adds a long array tag.
func (c *Compound) PutLongArray(name string, v []int64) *Compound {
	return c.put(name, TagLongArray, v)
}

// PutCompound adds a nested compound tag.
func (c *Compound) PutCompound(name string, v *Compound) *Compound {
	return c.put(name, TagCompound, v)
}

// PutList adds a list tag.
func (c *Compound) PutList(name string, v *List) *Compound {
	return c.put(name, TagList, v)
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

// GetShort returns a short tag by name.
func (c *Compound) GetShort(name string) (int16, bool) {
	value, tag, ok := c.Get(name)
	if !ok || tag != TagShort {
		return 0, false
	}
	result, ok := value.(int16)
	return result, ok
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

// GetAllInts returns a copy of all int-typed entries by name.
func (c *Compound) GetAllInts() map[string]int32 {
	result := make(map[string]int32)
	if c == nil {
		return result
	}
	if c == nil {
		return result
	}
	for _, e := range c.entries {
		if e.tagID == TagInt {
			if value, ok := e.value.(int32); ok {
				result[e.name] = value
			}
		}
	}
	return result
}

// GetByteArray returns a byte array tag without exposing its backing storage.
func (c *Compound) GetByteArray(name string) ([]byte, bool) {
	value, tag, ok := c.Get(name)
	if !ok || tag != TagByteArray {
		return nil, false
	}
	data, ok := value.([]byte)
	return append([]byte(nil), data...), ok
}
