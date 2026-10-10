package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

const maxMapDecorations = 1024

// MapDecoration describes one icon rendered on a map.
type MapDecoration struct {
	TypeID   int32
	X        int8
	Y        int8
	Rotation byte
	NameNBT  []byte
}

// MapPatch describes a rectangular color update.
type MapPatch struct {
	StartX byte
	StartY byte
	Width  byte
	Height byte
	Colors []byte
}

// MapData sends map item data to the client.
type MapData struct {
	MapID          int32
	Scale          byte
	Locked         bool
	HasDecorations bool
	Decorations    []MapDecoration
	Patch          *MapPatch
}

func NewMapData() protocol.Packet { return &MapData{} }

func (p *MapData) ID() int32 {
	return int32(packetid.ClientboundMap)
}

func (p *MapData) Decode(buf *protocol.Buffer) error {
	var err error
	if p.MapID, err = buf.ReadVarInt(); err != nil || p.MapID < 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("map ID out of range: %d", p.MapID)
	}
	if p.Scale, err = buf.ReadByte(); err != nil || p.Scale > 4 {
		if err != nil {
			return err
		}
		return fmt.Errorf("map scale out of range: %d", p.Scale)
	}
	if p.Locked, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasDecorations, err = buf.ReadBool(); err != nil {
		return err
	}
	p.Decorations = nil
	if p.HasDecorations {
		count, err := buf.ReadVarInt()
		if err != nil {
			return err
		}
		if count < 0 || count > maxMapDecorations {
			return fmt.Errorf("map decoration count out of range: %d", count)
		}
		p.Decorations = make([]MapDecoration, count)
		for index := range p.Decorations {
			decoration := &p.Decorations[index]
			if decoration.TypeID, err = buf.ReadVarInt(); err != nil || decoration.TypeID < 0 {
				if err != nil {
					return err
				}
				return fmt.Errorf("map decoration type out of range: %d", decoration.TypeID)
			}
			x, err := buf.ReadByte()
			if err != nil {
				return err
			}
			y, err := buf.ReadByte()
			if err != nil {
				return err
			}
			decoration.X, decoration.Y = int8(x), int8(y)
			if decoration.Rotation, err = buf.ReadByte(); err != nil {
				return err
			}
			decoration.Rotation &= 15
			hasName, err := buf.ReadBool()
			if err != nil {
				return err
			}
			if hasName {
				decoder := nbt.NewDecoder(buf.RemainingBytes())
				if _, _, err := decoder.ReadRootTag(); err != nil {
					return fmt.Errorf("map decoration name: %w", err)
				}
				decoration.NameNBT, err = buf.ReadBytes(decoder.Pos())
				if err != nil {
					return err
				}
			}
		}
	}
	width, err := buf.ReadByte()
	if err != nil {
		return err
	}
	p.Patch = nil
	if width == 0 {
		return nil
	}
	height, err := buf.ReadByte()
	if err != nil {
		return err
	}
	startX, err := buf.ReadByte()
	if err != nil {
		return err
	}
	startY, err := buf.ReadByte()
	if err != nil {
		return err
	}
	length, err := buf.ReadVarInt()
	if err != nil {
		return err
	}
	if err := validateMapPatch(startX, startY, width, height, int(length)); err != nil {
		return err
	}
	colors, err := buf.ReadBytes(int(length))
	if err != nil {
		return err
	}
	p.Patch = &MapPatch{StartX: startX, StartY: startY, Width: width, Height: height, Colors: append([]byte(nil), colors...)}
	return nil
}

func (p *MapData) Encode(buf *protocol.Buffer) error {
	if p.MapID < 0 || p.Scale > 4 || len(p.Decorations) > maxMapDecorations {
		return fmt.Errorf("invalid map packet")
	}
	buf.WriteVarInt(p.MapID)
	buf.WriteByte(p.Scale)
	buf.WriteBool(p.Locked)
	buf.WriteBool(p.HasDecorations)
	if p.HasDecorations {
		buf.WriteVarInt(int32(len(p.Decorations)))
		for _, decoration := range p.Decorations {
			if decoration.TypeID < 0 || decoration.Rotation > 15 {
				return fmt.Errorf("invalid map decoration")
			}
			buf.WriteVarInt(decoration.TypeID)
			buf.WriteByte(byte(decoration.X))
			buf.WriteByte(byte(decoration.Y))
			buf.WriteByte(decoration.Rotation)
			buf.WriteBool(len(decoration.NameNBT) > 0)
			if len(decoration.NameNBT) > 0 {
				decoder := nbt.NewDecoder(decoration.NameNBT)
				if _, _, err := decoder.ReadRootTag(); err != nil || decoder.Pos() != len(decoration.NameNBT) {
					return fmt.Errorf("invalid map decoration name NBT")
				}
				buf.WriteBytes(decoration.NameNBT)
			}
		}
	}
	if p.Patch == nil {
		buf.WriteByte(0)
		return nil
	}
	if err := validateMapPatch(p.Patch.StartX, p.Patch.StartY, p.Patch.Width, p.Patch.Height, len(p.Patch.Colors)); err != nil {
		return err
	}
	buf.WriteByte(p.Patch.Width)
	buf.WriteByte(p.Patch.Height)
	buf.WriteByte(p.Patch.StartX)
	buf.WriteByte(p.Patch.StartY)
	buf.WriteVarInt(int32(len(p.Patch.Colors)))
	buf.WriteBytes(p.Patch.Colors)
	return nil
}

// validateMapPatch verifies the 128 by 128 map bounds and payload length.
func validateMapPatch(startX, startY, width, height byte, colorCount int) error {
	if width == 0 || height == 0 || int(startX)+int(width) > 128 || int(startY)+int(height) > 128 {
		return fmt.Errorf("map patch out of bounds")
	}
	if colorCount != int(width)*int(height) {
		return fmt.Errorf("map patch color count %d does not match %dx%d", colorCount, width, height)
	}
	return nil
}
