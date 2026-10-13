package mapdata

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
	"github.com/vitismc/vitis/internal/inventory"
	"github.com/vitismc/vitis/internal/item"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/level"
)

// Store owns SavedData maps and a monotonic ID counter for crafting.
type Store struct {
	mu       sync.RWMutex
	saveMu   sync.Mutex
	dir      string
	counts   *nbt.Compound
	lastID   int32
	maps     map[int32]*nbt.Compound
	revision uint64
	saved    uint64
}

// Open loads maps before the world tick starts and preserves unknown data fields.
func Open(worldDir string) (*Store, error) {
	store := &Store{dir: filepath.Join(worldDir, "data"), lastID: -1, maps: make(map[int32]*nbt.Compound)}
	counts, err := level.ReadSavedData(filepath.Join(store.dir, "idcounts.dat"))
	if err == nil {
		store.counts = counts.Clone()
		if data, ok := counts.GetCompound("data"); ok {
			if id, ok := data.GetInt("map"); ok {
				store.lastID = id
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	entries, err := os.ReadDir(store.dir)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "map_") || !strings.HasSuffix(name, ".dat") {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(name, "map_"), ".dat"), 10, 32)
		if err != nil || id < 0 {
			return nil, fmt.Errorf("invalid map data filename")
		}
		root, err := level.ReadSavedData(filepath.Join(store.dir, name))
		if err != nil {
			return nil, fmt.Errorf("map %d: %w", id, err)
		}
		data, ok := root.GetCompound("data")
		if !ok {
			return nil, fmt.Errorf("map %d missing data", id)
		}
		scale, ok := data.GetByte("scale")
		if !ok || scale < 0 || scale > 4 {
			return nil, fmt.Errorf("invalid map scale")
		}
		colors, ok := data.GetByteArray("colors")
		if !ok || len(colors) != Size*Size {
			return nil, fmt.Errorf("invalid map colors")
		}
		store.maps[int32(id)] = root
		store.lastID = max(store.lastID, int32(id))
	}
	return store, nil
}

// CanExtend validates saved scale and exploration-map restrictions without mutations.
func (s *Store) CanExtend(id int32) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	root := s.maps[id]
	if root == nil {
		return false
	}
	data, _ := root.GetCompound("data")
	scale, _ := data.GetByte("scale")
	if scale >= 4 {
		return false
	}
	if decorations, ok := data.GetList("decorations"); ok {
		for _, entry := range decorations.Elements() {
			if c, ok := entry.(*nbt.Compound); ok {
				typ, _ := c.GetByte("type")
				if typ == 8 || typ == 9 {
					return false
				}
			}
		}
	}
	return true
}

// CommitCraft applies map postprocessing only after result capacity is verified.
func (s *Store) CommitCraft(stack inventory.ItemStack) (inventory.ItemStack, error) {
	operation, ok := stack.Component(dc.ComponentMapPostProcessing)
	if !ok {
		return stack, nil
	}
	value, ok := stack.Component(dc.ComponentMapId)
	if !ok {
		return inventory.ItemStack{}, fmt.Errorf("map has no id")
	}
	sourceID := int32(value.(inventory.VarIntComponent))
	s.mu.Lock()
	defer s.mu.Unlock()
	source := s.maps[sourceID]
	if source == nil || s.lastID == math.MaxInt32 {
		return inventory.ItemStack{}, fmt.Errorf("map data unavailable")
	}
	root := source.Clone()
	data, _ := root.GetCompound("data")
	switch int32(operation.(inventory.VarIntComponent)) {
	case 0:
		scale, _ := data.GetByte("scale")
		if scale >= 4 {
			return inventory.ItemStack{}, fmt.Errorf("map cannot extend")
		}
		dimension, _ := data.GetString("dimension")
		x, _ := data.GetInt("xCenter")
		z, _ := data.GetInt("zCenter")
		aligned, err := New(s.lastID+1, dimension, x, z, byte(scale+1), true, false)
		if err != nil {
			return inventory.ItemStack{}, err
		}
		x, z = aligned.Center()
		data.PutBool("locked", false).PutByte("scale", scale+1).PutInt("xCenter", x).PutInt("zCenter", z).PutByteArray("colors", make([]byte, Size*Size))
		data.Delete("banners")
		data.Delete("frames")
	case 1:
		data.PutBool("locked", true)
	default:
		return inventory.ItemStack{}, fmt.Errorf("invalid map operation")
	}
	s.lastID++
	s.maps[s.lastID] = root
	s.revision++
	result := stack.Clone()
	result.SetComponent(dc.ComponentMapId, inventory.VarIntComponent(s.lastID))
	result.RemoveComponent(dc.ComponentMapPostProcessing)
	return result, nil
}

// Save captures map state under a short lock and writes it outside world simulation.
func (s *Store) Save() error {
	if s == nil {
		return nil
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.RLock()
	revision := s.revision
	if revision == s.saved {
		s.mu.RUnlock()
		return nil
	}
	maps := make(map[int32]*nbt.Compound, len(s.maps))
	for id, data := range s.maps {
		maps[id] = data.Clone()
	}
	lastID := s.lastID
	counts := s.counts.Clone()
	s.mu.RUnlock()
	// Publish the counter before map files so a crash cannot reuse an issued ID.
	if counts == nil {
		counts = nbt.NewCompound()
	}
	counts.PutInt("DataVersion", protocol.CurrentDataVersion)
	counter, _ := counts.GetCompound("data")
	if counter == nil {
		counter = nbt.NewCompound()
	}
	counter.PutInt("map", lastID)
	counts.PutCompound("data", counter)
	if err := level.WriteSavedData(filepath.Join(s.dir, "idcounts.dat"), counts); err != nil {
		return err
	}
	for id, data := range maps {
		if err := level.WriteSavedData(filepath.Join(s.dir, fmt.Sprintf("map_%d.dat", id)), data); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.saved = revision
	s.mu.Unlock()
	return nil
}

// Create allocates the SavedData dependency of an actually used empty map.
func (s *Store) Create(dimension string, x, z int32) (inventory.ItemStack, error) {
	if s == nil {
		return inventory.ItemStack{}, fmt.Errorf("map store unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastID == math.MaxInt32 {
		return inventory.ItemStack{}, fmt.Errorf("map id exhausted")
	}
	data, err := New(s.lastID+1, dimension, x, z, 0, true, false)
	if err != nil {
		return inventory.ItemStack{}, err
	}
	centerX, centerZ := data.Center()
	root := nbt.NewCompound().PutInt("DataVersion", protocol.CurrentDataVersion).PutCompound("data", nbt.NewCompound().PutString("dimension", dimension).PutInt("xCenter", centerX).PutInt("zCenter", centerZ).PutByte("scale", 0).PutBool("trackingPosition", true).PutBool("unlimitedTracking", false).PutBool("locked", false).PutByteArray("colors", make([]byte, Size*Size)))
	s.lastID++
	s.maps[s.lastID] = root
	s.revision++
	stack := inventory.NewSlot(item.IDByName("minecraft:filled_map"), 1)
	stack.SetComponent(dc.ComponentMapId, inventory.VarIntComponent(s.lastID))
	return stack, nil
}
