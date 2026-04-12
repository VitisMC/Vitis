package blockentity

import (
	"sync"

	"github.com/vitismc/vitis/internal/nbt"
)

// BlockEntityType IDs matching the Minecraft protocol registry.
const (
	TypeSign     int32 = 7
	TypeHangSign int32 = 35
)

// SignData holds the text lines for both sides of a sign.
type SignData struct {
	FrontLines [4]string
	BackLines  [4]string
}

// key packs a block position into a single int64.
func key(x, y, z int) int64 {
	return (int64(x) & 0x3FFFFFF << 38) | (int64(z) & 0x3FFFFFF << 12) | (int64(y) & 0xFFF)
}

// Store holds block entity data keyed by position.
type Store struct {
	mu    sync.RWMutex
	signs map[int64]*SignData
}

// NewStore creates an empty block entity store.
func NewStore() *Store {
	return &Store{
		signs: make(map[int64]*SignData),
	}
}

// SetSign stores sign data at the given position.
func (s *Store) SetSign(x, y, z int, data *SignData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signs[key(x, y, z)] = data
}

// GetSign returns sign data at the given position, or nil.
func (s *Store) GetSign(x, y, z int) *SignData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.signs[key(x, y, z)]
}

// RemoveSign removes sign data at the given position.
func (s *Store) RemoveSign(x, y, z int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.signs, key(x, y, z))
}

// EncodeSignNBT encodes sign data to NBT bytes for the BlockEntityData packet.
func EncodeSignNBT(sd *SignData) []byte {
	root := nbt.NewCompound()
	root.PutString("id", "minecraft:sign")

	front := nbt.NewCompound()
	frontMsgs := nbt.NewList(nbt.TagString)
	for _, line := range sd.FrontLines {
		if line == "" {
			frontMsgs.Add("{\"text\":\"\"}")
		} else {
			frontMsgs.Add("{\"text\":\"" + escapeJSON(line) + "\"}")
		}
	}
	front.PutList("messages", frontMsgs)
	root.PutCompound("front_text", front)

	back := nbt.NewCompound()
	backMsgs := nbt.NewList(nbt.TagString)
	for _, line := range sd.BackLines {
		if line == "" {
			backMsgs.Add("{\"text\":\"\"}")
		} else {
			backMsgs.Add("{\"text\":\"" + escapeJSON(line) + "\"}")
		}
	}
	back.PutList("messages", backMsgs)
	root.PutCompound("back_text", back)

	enc := nbt.NewEncoder(256)
	_ = enc.WriteRootCompound(root)
	return enc.Bytes()
}

func escapeJSON(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
