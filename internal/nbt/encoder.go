package nbt

import (
	"fmt"
	"math"
)

// Encoder writes NBT data into a byte buffer.
type Encoder struct {
	buf []byte
}

// NewEncoder creates a new NBT encoder with the given initial capacity.
func NewEncoder(capacity int) *Encoder {
	if capacity < 0 {
		capacity = 256
	}
	return &Encoder{buf: make([]byte, 0, capacity)}
}

// Bytes returns the encoded NBT data.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// WriteRootCompound writes a root compound tag with no name (network NBT format).
func (e *Encoder) WriteRootCompound(c *Compound) error {
	e.writeByte(TagCompound)
	return e.writeCompoundPayload(c)
}

// WriteNamedRootCompound writes a root compound tag with a name (standard NBT format).
func (e *Encoder) WriteNamedRootCompound(name string, c *Compound) error {
	e.writeByte(TagCompound)
	e.writeString(name)
	return e.writeCompoundPayload(c)
}

// WriteNetworkCompound writes a compound tag without tag type prefix or name (inline network NBT).
func (e *Encoder) WriteNetworkCompound(c *Compound) error {
	return e.writeCompoundPayload(c)
}

func (e *Encoder) writeCompoundPayload(c *Compound) error {
	if c == nil {
		e.writeByte(TagEnd)
		return nil
	}
	for _, ent := range c.Entries() {
		e.writeByte(ent.tagID)
		e.writeString(ent.name)
		if err := e.writePayload(ent.tagID, ent.value); err != nil {
			return fmt.Errorf("encode compound entry %q: %w", ent.name, err)
		}
	}
	e.writeByte(TagEnd)
	return nil
}

func (e *Encoder) writePayload(tagID byte, value interface{}) error {
	switch tagID {
	case TagByte:
		v, ok := value.(int8)
		if !ok {
			return fmt.Errorf("expected int8 for TagByte, got %T", value)
		}
		e.writeByte(byte(v))
	case TagShort:
		v, ok := value.(int16)
		if !ok {
			return fmt.Errorf("expected int16 for TagShort, got %T", value)
		}
		e.writeInt16(v)
	case TagInt:
		v, ok := value.(int32)
		if !ok {
			return fmt.Errorf("expected int32 for TagInt, got %T", value)
		}
		e.writeInt32(v)
	case TagLong:
		v, ok := value.(int64)
		if !ok {
			return fmt.Errorf("expected int64 for TagLong, got %T", value)
		}
		e.writeInt64(v)
	case TagFloat:
		v, ok := value.(float32)
		if !ok {
			return fmt.Errorf("expected float32 for TagFloat, got %T", value)
		}
		e.writeFloat32(v)
	case TagDouble:
		v, ok := value.(float64)
		if !ok {
			return fmt.Errorf("expected float64 for TagDouble, got %T", value)
		}
		e.writeFloat64(v)
	case TagByteArray:
		v, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte for TagByteArray, got %T", value)
		}
		e.writeInt32(int32(len(v)))
		e.buf = append(e.buf, v...)
	case TagString:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string for TagString, got %T", value)
		}
		e.writeString(v)
	case TagList:
		v, ok := value.(*List)
		if !ok {
			return fmt.Errorf("expected *List for TagList, got %T", value)
		}
		return e.writeListPayload(v)
	case TagCompound:
		v, ok := value.(*Compound)
		if !ok {
			return fmt.Errorf("expected *Compound for TagCompound, got %T", value)
		}
		return e.writeCompoundPayload(v)
	case TagIntArray:
		v, ok := value.([]int32)
		if !ok {
			return fmt.Errorf("expected []int32 for TagIntArray, got %T", value)
		}
		e.writeInt32(int32(len(v)))
		for _, elem := range v {
			e.writeInt32(elem)
		}
	case TagLongArray:
		v, ok := value.([]int64)
		if !ok {
			return fmt.Errorf("expected []int64 for TagLongArray, got %T", value)
		}
		e.writeInt32(int32(len(v)))
		for _, elem := range v {
			e.writeInt64(elem)
		}
	default:
		return fmt.Errorf("unknown tag type %d", tagID)
	}
	return nil
}

func (e *Encoder) writeListPayload(l *List) error {
	if l == nil || l.Len() == 0 {
		e.writeByte(TagEnd)
		e.writeInt32(0)
		return nil
	}
	e.writeByte(l.ElementType())
	e.writeInt32(int32(l.Len()))
	for _, elem := range l.Elements() {
		if err := e.writePayload(l.ElementType(), elem); err != nil {
			return fmt.Errorf("encode list element: %w", err)
		}
	}
	return nil
}

func (e *Encoder) writeByte(v byte) {
	e.buf = append(e.buf, v)
}

func (e *Encoder) writeString(v string) {
	length := uint16(len(v))
	e.buf = append(e.buf, byte(length>>8), byte(length))
	e.buf = append(e.buf, v...)
}

func (e *Encoder) writeInt16(v int16) {
	e.buf = append(e.buf, byte(uint16(v)>>8), byte(v))
}

func (e *Encoder) writeInt32(v int32) {
	e.buf = append(e.buf, byte(uint32(v)>>24), byte(uint32(v)>>16), byte(uint32(v)>>8), byte(v))
}

func (e *Encoder) writeInt64(v int64) {
	e.buf = append(e.buf,
		byte(uint64(v)>>56), byte(uint64(v)>>48), byte(uint64(v)>>40), byte(uint64(v)>>32),
		byte(uint64(v)>>24), byte(uint64(v)>>16), byte(uint64(v)>>8), byte(v),
	)
}

func (e *Encoder) writeFloat32(v float32) {
	e.writeInt32(int32(math.Float32bits(v)))
}

func (e *Encoder) writeFloat64(v float64) {
	e.writeInt64(int64(math.Float64bits(v)))
}
