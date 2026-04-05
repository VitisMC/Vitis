package metadata

import (
	"encoding/binary"
	"math"

	mdt "github.com/vitismc/vitis/internal/data/generated/meta_data_type"
	td "github.com/vitismc/vitis/internal/data/generated/tracked_data"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/protocol"
)

const (
	TypeByte              = mdt.TypeByte
	TypeVarInt            = mdt.TypeInt
	TypeVarLong           = mdt.TypeLong
	TypeFloat             = mdt.TypeFloat
	TypeString            = mdt.TypeString
	TypeTextComponent     = mdt.TypeComponent
	TypeOptTextComponent  = mdt.TypeOptionalComponent
	TypeSlot              = mdt.TypeItemStack
	TypeBoolean           = mdt.TypeBoolean
	TypeRotations         = mdt.TypeRotations
	TypePosition          = mdt.TypeBlockPos
	TypeOptPosition       = mdt.TypeOptionalBlockPos
	TypeDirection         = mdt.TypeDirection
	TypeOptUUID           = mdt.TypeOptionalUuid
	TypeBlockState        = mdt.TypeBlockState
	TypeOptBlockState     = mdt.TypeOptionalBlockState
	TypeNBT               = mdt.TypeCompoundTag
	TypeParticle          = mdt.TypeParticle
	TypeParticles         = mdt.TypeParticles
	TypeVillagerData      = mdt.TypeVillagerData
	TypeOptVarInt         = mdt.TypeOptionalUnsignedInt
	TypePose              = mdt.TypePose
	TypeCatVariant        = mdt.TypeCatVariant
	TypeWolfVariant       = mdt.TypeWolfVariant
	TypeFrogVariant       = mdt.TypeFrogVariant
	TypeOptGlobalPosition = mdt.TypeOptionalGlobalPos
	TypePaintingVariant   = mdt.TypePaintingVariant
	TypeSnifferState      = mdt.TypeSnifferState
	TypeArmadilloState    = mdt.TypeArmadilloState
	TypeVector3           = mdt.TypeVector3
	TypeQuaternion        = mdt.TypeQuaternion
)

const (
	IndexBase              = td.EntitySharedFlagsId
	IndexAirTicks          = td.EntityAirSupplyId
	IndexCustomName        = td.EntityCustomName
	IndexCustomNameVisible = td.EntityCustomNameVisible
	IndexSilent            = td.EntitySilent
	IndexNoGravity         = td.EntityNoGravity
	IndexPose              = td.EntityPose
	IndexFrozenTicks       = td.EntityTicksFrozen

	IndexLivingHandStates  = td.LivingEntityLivingEntityFlags
	IndexLivingHealth      = td.LivingEntityHealthId
	IndexLivingPotionColor = td.LivingEntityEffectParticles
	IndexLivingPotionAmb   = td.LivingEntityEffectAmbienceId
	IndexLivingArrowCount  = td.LivingEntityArrowCountId
	IndexLivingBeeStingers = td.LivingEntityStingerCountId
	IndexLivingSleepingPos = td.LivingEntitySleepingPosId

	IndexPlayerAdditionalHearts = td.PlayerPlayerAbsorptionId
	IndexPlayerScore            = td.PlayerScoreId
	IndexPlayerSkinParts        = td.PlayerPlayerModeCustomisation
	IndexPlayerMainHand         = td.PlayerPlayerMainHand

	IndexItemEntityItem = td.ItemEntityItem
)

const EndMarker byte = 0xFF

// Entry is a single metadata entry with index, type, and value.
type Entry struct {
	Index byte
	Type  int32
	Value interface{}
}

// Map holds entity metadata entries keyed by index.
type Map struct {
	entries map[byte]Entry
}

// New creates an empty metadata map.
func New() *Map {
	return &Map{entries: make(map[byte]Entry, 8)}
}

// SetByte sets a byte metadata entry.
func (m *Map) SetByte(index byte, v int8) {
	m.entries[index] = Entry{Index: index, Type: TypeByte, Value: v}
}

// SetVarInt sets a VarInt metadata entry.
func (m *Map) SetVarInt(index byte, v int32) {
	m.entries[index] = Entry{Index: index, Type: TypeVarInt, Value: v}
}

// SetFloat sets a float metadata entry.
func (m *Map) SetFloat(index byte, v float32) {
	m.entries[index] = Entry{Index: index, Type: TypeFloat, Value: v}
}

// SetBool sets a boolean metadata entry.
func (m *Map) SetBool(index byte, v bool) {
	m.entries[index] = Entry{Index: index, Type: TypeBoolean, Value: v}
}

// SetString sets a string metadata entry.
func (m *Map) SetString(index byte, v string) {
	m.entries[index] = Entry{Index: index, Type: TypeString, Value: v}
}

// SetOptPosition sets an optional position (nil = absent).
func (m *Map) SetOptPosition(index byte, v *[3]int32) {
	m.entries[index] = Entry{Index: index, Type: TypeOptPosition, Value: v}
}

// SetPose sets a Pose metadata entry (VarInt on wire but with TypePose type ID).
func (m *Map) SetPose(index byte, v int32) {
	m.entries[index] = Entry{Index: index, Type: TypePose, Value: v}
}

// SetOptTextComponent sets an optional text component (nil = absent).
func (m *Map) SetOptTextComponent(index byte, v *string) {
	m.entries[index] = Entry{Index: index, Type: TypeOptTextComponent, Value: v}
}

// SetSlot sets an item stack metadata entry.
func (m *Map) SetSlot(index byte, s inventory.Slot) {
	m.entries[index] = Entry{Index: index, Type: TypeSlot, Value: s}
}

// Encode serializes all metadata entries to the wire format.
func (m *Map) Encode() []byte {
	buf := make([]byte, 0, 128)
	for _, e := range m.entries {
		buf = append(buf, e.Index)
		buf = appendVarInt(buf, e.Type)
		buf = encodeValue(buf, e.Type, e.Value)
	}
	buf = append(buf, EndMarker)
	return buf
}

// DefaultPlayer returns default player metadata.
func DefaultPlayer() *Map {
	m := New()
	m.SetByte(IndexBase, 0)
	m.SetVarInt(IndexAirTicks, 300)
	m.SetOptTextComponent(IndexCustomName, nil)
	m.SetBool(IndexCustomNameVisible, false)
	m.SetBool(IndexSilent, false)
	m.SetBool(IndexNoGravity, false)
	m.SetPose(IndexPose, 0)
	m.SetVarInt(IndexFrozenTicks, 0)
	m.SetByte(IndexLivingHandStates, 0)
	m.SetFloat(IndexLivingHealth, 20.0)
	m.SetBool(IndexLivingPotionAmb, false)
	m.SetVarInt(IndexLivingArrowCount, 0)
	m.SetVarInt(IndexLivingBeeStingers, 0)
	m.SetOptPosition(IndexLivingSleepingPos, nil)
	m.SetFloat(IndexPlayerAdditionalHearts, 0)
	m.SetVarInt(IndexPlayerScore, 0)
	m.SetByte(IndexPlayerSkinParts, 0x7F)
	m.SetByte(IndexPlayerMainHand, 1)
	return m
}

func encodeValue(buf []byte, typeID int32, value interface{}) []byte {
	switch typeID {
	case TypeByte:
		v := value.(int8)
		return append(buf, byte(v))
	case TypeVarInt, TypeDirection, TypeBlockState, TypeOptBlockState,
		TypePose, TypeCatVariant, TypeFrogVariant, TypePaintingVariant,
		TypeSnifferState, TypeWolfVariant, TypeArmadilloState:
		v := value.(int32)
		return appendVarInt(buf, v)
	case TypeFloat:
		v := value.(float32)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], math.Float32bits(v))
		return append(buf, b[:]...)
	case TypeString:
		v := value.(string)
		buf = appendVarInt(buf, int32(len(v)))
		return append(buf, v...)
	case TypeBoolean:
		v := value.(bool)
		if v {
			return append(buf, 1)
		}
		return append(buf, 0)
	case TypeOptTextComponent:
		if value == nil {
			return append(buf, 0)
		}
		return append(buf, 0)
	case TypeOptPosition:
		if value == nil {
			return append(buf, 0)
		}
		return append(buf, 0)
	case TypeOptVarInt:
		v := value.(int32)
		return appendVarInt(buf, v)
	case TypeSlot:
		s := value.(inventory.Slot)
		pb := &protocol.Buffer{}
		inventory.EncodeSlot(pb, s)
		return append(buf, pb.Bytes()...)
	}
	return buf
}

func appendVarInt(dst []byte, value int32) []byte {
	uv := uint32(value)
	for uv >= 0x80 {
		dst = append(dst, byte(uv&0x7F)|0x80)
		uv >>= 7
	}
	return append(dst, byte(uv))
}
