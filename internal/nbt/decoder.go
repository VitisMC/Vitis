package nbt

import (
	"fmt"
	"math"
)

// Decoder reads NBT data from a byte buffer.
type Decoder struct {
	data []byte
	pos  int
}

// NewDecoder creates a new NBT decoder over the given data.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{data: data}
}

// Pos returns the current read position.
func (d *Decoder) Pos() int {
	return d.pos
}

// ReadRootCompound reads a root compound tag with no name (network NBT format).
func (d *Decoder) ReadRootCompound() (*Compound, error) {
	tagType, err := d.readByte()
	if err != nil {
		return nil, fmt.Errorf("read root tag type: %w", err)
	}
	if tagType == TagEnd {
		return nil, nil
	}
	if tagType != TagCompound {
		return nil, fmt.Errorf("expected root compound tag (10), got %d", tagType)
	}
	return d.readCompoundPayload()
}

// ReadNamedRootCompound reads a root compound tag with a name (standard NBT format).
func (d *Decoder) ReadNamedRootCompound() (string, *Compound, error) {
	tagType, err := d.readByte()
	if err != nil {
		return "", nil, fmt.Errorf("read root tag type: %w", err)
	}
	if tagType != TagCompound {
		return "", nil, fmt.Errorf("expected root compound tag (10), got %d", tagType)
	}
	name, err := d.readString()
	if err != nil {
		return "", nil, fmt.Errorf("read root name: %w", err)
	}
	c, err := d.readCompoundPayload()
	if err != nil {
		return "", nil, err
	}
	return name, c, nil
}

func (d *Decoder) readCompoundPayload() (*Compound, error) {
	c := NewCompound()
	for {
		tagType, err := d.readByte()
		if err != nil {
			return nil, fmt.Errorf("read compound tag type: %w", err)
		}
		if tagType == TagEnd {
			return c, nil
		}
		name, err := d.readString()
		if err != nil {
			return nil, fmt.Errorf("read compound entry name: %w", err)
		}
		value, err := d.readPayload(tagType)
		if err != nil {
			return nil, fmt.Errorf("read compound entry %q: %w", name, err)
		}
		c.entries = append(c.entries, entry{name: name, tagID: tagType, value: value})
	}
}

func (d *Decoder) readPayload(tagID byte) (interface{}, error) {
	switch tagID {
	case TagByte:
		b, err := d.readByte()
		return int8(b), err
	case TagShort:
		return d.readInt16()
	case TagInt:
		return d.readInt32()
	case TagLong:
		return d.readInt64()
	case TagFloat:
		return d.readFloat32()
	case TagDouble:
		return d.readFloat64()
	case TagByteArray:
		length, err := d.readInt32()
		if err != nil {
			return nil, err
		}
		return d.readBytes(int(length))
	case TagString:
		return d.readString()
	case TagList:
		return d.readListPayload()
	case TagCompound:
		return d.readCompoundPayload()
	case TagIntArray:
		length, err := d.readInt32()
		if err != nil {
			return nil, err
		}
		arr := make([]int32, length)
		for i := range arr {
			arr[i], err = d.readInt32()
			if err != nil {
				return nil, err
			}
		}
		return arr, nil
	case TagLongArray:
		length, err := d.readInt32()
		if err != nil {
			return nil, err
		}
		arr := make([]int64, length)
		for i := range arr {
			arr[i], err = d.readInt64()
			if err != nil {
				return nil, err
			}
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unknown tag type %d", tagID)
	}
}

func (d *Decoder) readListPayload() (*List, error) {
	elemType, err := d.readByte()
	if err != nil {
		return nil, err
	}
	length, err := d.readInt32()
	if err != nil {
		return nil, err
	}
	l := NewList(elemType)
	for i := int32(0); i < length; i++ {
		value, err := d.readPayload(elemType)
		if err != nil {
			return nil, fmt.Errorf("read list element %d: %w", i, err)
		}
		l.Add(value)
	}
	return l, nil
}

func (d *Decoder) readByte() (byte, error) {
	if d.pos >= len(d.data) {
		return 0, fmt.Errorf("nbt: unexpected end of data at pos %d", d.pos)
	}
	v := d.data[d.pos]
	d.pos++
	return v, nil
}

func (d *Decoder) readBytes(n int) ([]byte, error) {
	if d.pos+n > len(d.data) {
		return nil, fmt.Errorf("nbt: need %d bytes at pos %d, have %d", n, d.pos, len(d.data)-d.pos)
	}
	out := make([]byte, n)
	copy(out, d.data[d.pos:d.pos+n])
	d.pos += n
	return out, nil
}

func (d *Decoder) readString() (string, error) {
	if d.pos+2 > len(d.data) {
		return "", fmt.Errorf("nbt: unexpected end of data reading string length at pos %d", d.pos)
	}
	length := int(uint16(d.data[d.pos])<<8 | uint16(d.data[d.pos+1]))
	d.pos += 2
	if d.pos+length > len(d.data) {
		return "", fmt.Errorf("nbt: string length %d exceeds data at pos %d", length, d.pos)
	}
	s := string(d.data[d.pos : d.pos+length])
	d.pos += length
	return s, nil
}

func (d *Decoder) readInt16() (int16, error) {
	if d.pos+2 > len(d.data) {
		return 0, fmt.Errorf("nbt: unexpected end of data at pos %d", d.pos)
	}
	v := int16(uint16(d.data[d.pos])<<8 | uint16(d.data[d.pos+1]))
	d.pos += 2
	return v, nil
}

func (d *Decoder) readInt32() (int32, error) {
	if d.pos+4 > len(d.data) {
		return 0, fmt.Errorf("nbt: unexpected end of data at pos %d", d.pos)
	}
	v := int32(uint32(d.data[d.pos])<<24 | uint32(d.data[d.pos+1])<<16 | uint32(d.data[d.pos+2])<<8 | uint32(d.data[d.pos+3]))
	d.pos += 4
	return v, nil
}

func (d *Decoder) readInt64() (int64, error) {
	if d.pos+8 > len(d.data) {
		return 0, fmt.Errorf("nbt: unexpected end of data at pos %d", d.pos)
	}
	v := int64(uint64(d.data[d.pos])<<56 | uint64(d.data[d.pos+1])<<48 | uint64(d.data[d.pos+2])<<40 | uint64(d.data[d.pos+3])<<32 |
		uint64(d.data[d.pos+4])<<24 | uint64(d.data[d.pos+5])<<16 | uint64(d.data[d.pos+6])<<8 | uint64(d.data[d.pos+7]))
	d.pos += 8
	return v, nil
}

func (d *Decoder) readFloat32() (float32, error) {
	v, err := d.readInt32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(uint32(v)), nil
}

func (d *Decoder) readFloat64() (float64, error) {
	v, err := d.readInt64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(v)), nil
}
