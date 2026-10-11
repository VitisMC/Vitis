package world

import (
	"errors"
	"fmt"
	"sort"

	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/projectile"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/world/chunk"
)

const maxChunkEntities = 4096

// persistentEntityChunk retains unknown entity graphs alongside implemented runtime types.
type persistentEntityChunk struct {
	root    *nbt.Compound
	opaque  []*nbt.Compound
	uuids   map[protocol.UUID]struct{}
	decoded []entity.PersistentEntity
}

// prepareEntityChunk validates the complete region payload without publishing entities.
func (w *World) prepareEntityChunk(c *chunk.Chunk) (*persistentEntityChunk, error) {
	state := &persistentEntityChunk{root: nbt.NewCompound(), uuids: make(map[protocol.UUID]struct{})}
	data, _ := c.EntityNBT()
	if len(data) == 0 {
		return state, nil
	}
	decoder := nbt.NewDecoder(data)
	_, root, err := decoder.ReadNamedRootCompound()
	if err != nil {
		return nil, fmt.Errorf("invalid entity region NBT: %w", err)
	}
	if root == nil || decoder.Pos() != len(data) {
		return nil, fmt.Errorf("invalid entity region NBT length")
	}
	version, ok := root.GetInt("DataVersion")
	if !ok || version != protocol.CurrentDataVersion {
		return nil, fmt.Errorf("unsupported entity DataVersion %d", version)
	}
	position, tag, _ := root.Get("Position")
	coords, ok := position.([]int32)
	if !ok || tag != nbt.TagIntArray || len(coords) != 2 || coords[0] != c.X() || coords[1] != c.Z() {
		return nil, fmt.Errorf("entity region position mismatch")
	}
	list, ok := root.GetList("Entities")
	if !ok || (list.ElementType() != nbt.TagCompound && !(list.ElementType() == nbt.TagEnd && list.Len() == 0)) || list.Len() > maxChunkEntities {
		return nil, fmt.Errorf("invalid entity region list")
	}
	occupied := make(map[protocol.UUID]struct{}, w.entities.Count())
	for _, e := range w.entities.Entities() {
		occupied[e.UUID()] = struct{}{}
	}
	for _, state := range w.entityChunks {
		for uuid := range state.uuids {
			occupied[uuid] = struct{}{}
		}
	}
	for _, value := range list.Elements() {
		raw, ok := value.(*nbt.Compound)
		if !ok || raw == nil {
			return nil, fmt.Errorf("invalid entity region entry")
		}
		base := entity.NewEntity(0, protocol.UUID{}, entity.EntityTypeMob, entity.Vec3{}, entity.Vec2{})
		if err := base.RestorePersistentBase(raw); err != nil {
			return nil, err
		}
		if base.ChunkX() != c.X() || base.ChunkZ() != c.Z() {
			return nil, fmt.Errorf("entity outside owning chunk")
		}
		if err := validateEntityGraph(raw, state.uuids, occupied, 0); err != nil {
			return nil, err
		}
		passengers, present := raw.GetList("Passengers")
		if present && passengers.Len() > 0 {
			state.opaque = append(state.opaque, raw.Clone())
			continue
		}
		decoded, err := entity.DecodePersistentEntity(0, raw)
		if err != nil {
			return nil, err
		}
		if decoded == nil {
			decoded, err = projectile.DecodePersistent(0, raw)
			if err != nil {
				return nil, err
			}
		}
		if decoded == nil {
			state.opaque = append(state.opaque, raw.Clone())
		} else {
			if _, err := decoded.PersistentNBT(); err != nil {
				return nil, err
			}
			state.decoded = append(state.decoded, decoded)
		}
	}
	state.root = root
	return state, nil
}

// validateEntityGraph reserves every passenger UUID and rejects malformed or duplicate graphs.
func validateEntityGraph(root *nbt.Compound, seen, occupied map[protocol.UUID]struct{}, depth int) error {
	if depth > 16 || len(seen) >= maxChunkEntities {
		return fmt.Errorf("entity passenger graph exceeds limit")
	}
	name, ok := root.GetString("id")
	if !ok || name == "" || name == "minecraft:player" {
		return fmt.Errorf("invalid persistent entity type")
	}
	base := entity.NewEntity(0, protocol.UUID{}, entity.EntityTypeMob, entity.Vec3{}, entity.Vec2{})
	if err := base.RestorePersistentBase(root); err != nil {
		return err
	}
	uuid := base.UUID()
	if _, ok := seen[uuid]; ok {
		return fmt.Errorf("duplicate entity UUID")
	}
	if _, ok := occupied[uuid]; ok {
		return fmt.Errorf("entity UUID already loaded")
	}
	seen[uuid] = struct{}{}
	if err := entity.ValidatePersistentTags(root, map[string]byte{"Passengers": nbt.TagList}); err != nil {
		return err
	}
	if passengers, ok := root.GetList("Passengers"); ok {
		if passengers.ElementType() != nbt.TagCompound && !(passengers.ElementType() == nbt.TagEnd && passengers.Len() == 0) {
			return fmt.Errorf("invalid passenger list")
		}
		for _, value := range passengers.Elements() {
			child, ok := value.(*nbt.Compound)
			if !ok || child == nil {
				return fmt.Errorf("invalid passenger")
			}
			if err := validateEntityGraph(child, seen, occupied, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// bindChunkEntities restores validated runtime state once on the world tick.
func (w *World) bindChunkEntities(c *chunk.Chunk) error {
	if w.chunkStore == nil || c.IsPregenerationOnly() {
		return nil
	}
	if w.entityChunks == nil {
		w.entityChunks = make(map[*chunk.Chunk]*persistentEntityChunk)
	}
	if _, ok := w.entityChunks[c]; ok {
		return nil
	}
	state, err := w.prepareEntityChunk(c)
	if err != nil {
		c.SetEntitySaveError(err)
		return err
	}
	for _, restored := range state.decoded {
		if !restored.BaseEntity().BindRuntimeID(w.entities.AllocateID()) {
			return fmt.Errorf("entity ID allocation failed")
		}
	}
	for _, restored := range state.decoded {
		switch e := restored.(type) {
		case *entity.ItemEntity:
			w.SpawnItemEntity(e)
		case *entity.XPOrb:
			w.SpawnXPOrb(e)
		case *entity.TNTEntity:
			w.SpawnTNT(e)
		case *entity.FallingBlock:
			w.entities.Add(e.Entity)
			w.fallingBlocks.Add(e)
		case *entity.MobEntity:
			w.mobs.Add(e)
		case *projectile.Arrow:
			w.SpawnArrow(e)
		case *projectile.ThrownEntity:
			w.SpawnThrown(e)
		case *projectile.Trident:
			w.SpawnTrident(e)
		default:
			return fmt.Errorf("missing runtime entity registration")
		}
	}
	// UUID reservations apply only to opaque graphs; live entities are checked in the manager.
	state.uuids = make(map[protocol.UUID]struct{})
	var reserve func(*nbt.Compound)
	reserve = func(raw *nbt.Compound) {
		uuid, _ := entity.UUIDFromNBT(raw, "UUID")
		state.uuids[uuid] = struct{}{}
		if list, ok := raw.GetList("Passengers"); ok {
			for _, value := range list.Elements() {
				reserve(value.(*nbt.Compound))
			}
		}
	}
	for _, raw := range state.opaque {
		reserve(raw)
	}
	state.decoded = nil
	w.entityChunks[c] = state
	return nil
}

// runtimePersistentEntity resolves a base entity to its authoritative specialized manager.
func (w *World) runtimePersistentEntity(id int32) entity.PersistentEntity {
	if e := w.items.Get(id); e != nil {
		return e
	}
	if e := w.xpOrbs.Get(id); e != nil {
		return e
	}
	if e := w.tnts.Get(id); e != nil {
		return e
	}
	if e := w.fallingBlocks.Get(id); e != nil {
		return e
	}
	if e := w.mobs.Get(id); e != nil {
		return e
	}
	if e := w.projectiles.GetArrow(id); e != nil {
		return e
	}
	if e := w.projectiles.GetThrown(id); e != nil {
		return e
	}
	if e := w.projectiles.GetTrident(id); e != nil {
		return e
	}
	return nil
}

// refreshEntitySnapshots freezes all entity changes before autosave or final flushing.
func (w *World) refreshEntitySnapshots() error {
	if w.chunkStore == nil {
		return nil
	}
	var failures []error
	resident := make(map[int64]*chunk.Chunk)
	w.chunks.VisitResident(func(c *chunk.Chunk) {
		resident[c.Key()] = c
		if err := w.bindChunkEntities(c); err != nil {
			failures = append(failures, err)
		}
	})
	grouped := make(map[int64][]*nbt.Compound)
	seen := make(map[protocol.UUID]struct{})
	for _, state := range w.entityChunks {
		for uuid := range state.uuids {
			seen[uuid] = struct{}{}
		}
	}
	for _, base := range w.entities.Entities() {
		if base.Removed() {
			continue
		}
		if _, duplicate := seen[base.UUID()]; duplicate {
			failures = append(failures, fmt.Errorf("duplicate runtime entity UUID"))
			continue
		}
		seen[base.UUID()] = struct{}{}
		if base.Type() == entity.EntityTypePlayer {
			continue
		}
		position := base.Position()
		x, z := entity.BlockToChunk(position.X), entity.BlockToChunk(position.Z)
		c, ok := resident[chunk.ChunkKey(x, z)]
		if !ok {
			failures = append(failures, fmt.Errorf("entity %d is outside resident chunks", base.ID()))
			w.chunks.RequestLoad(x, z)
			continue
		}
		persistent := w.runtimePersistentEntity(base.ID())
		if persistent == nil {
			failures = append(failures, fmt.Errorf("entity %d has no persistence codec", base.ID()))
			continue
		}
		if tnt, ok := persistent.(*entity.TNTEntity); ok {
			tnt.ResolvePersistentOwner(w.entities)
		}
		raw, err := persistent.PersistentNBT()
		if err != nil {
			failures = append(failures, fmt.Errorf("snapshot entity %d: %w", base.ID(), err))
			continue
		}
		grouped[c.Key()] = append(grouped[c.Key()], raw)
	}
	if err := errors.Join(failures...); err != nil {
		// Retain every previous snapshot if an entity could otherwise disappear from its source.
		for _, c := range resident {
			if !c.IsPregenerationOnly() {
				c.SetEntitySaveError(err)
			}
		}
		return err
	}
	for c, state := range w.entityChunks {
		entries := append(grouped[c.Key()], state.opaque...)
		if len(entries) > maxChunkEntities {
			err := fmt.Errorf("chunk entity count exceeds limit")
			c.SetEntitySaveError(err)
			failures = append(failures, err)
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			a, _ := entity.UUIDFromNBT(entries[i], "UUID")
			b, _ := entity.UUIDFromNBT(entries[j], "UUID")
			if a[0] == b[0] {
				return a[1] < b[1]
			}
			return a[0] < b[0]
		})
		list := nbt.NewList(nbt.TagCompound)
		for _, raw := range entries {
			list.Add(raw)
		}
		root := state.root.Clone()
		root.PutInt("DataVersion", protocol.CurrentDataVersion).PutIntArray("Position", []int32{c.X(), c.Z()}).PutList("Entities", list)
		encoder := nbt.NewEncoder(1024)
		if err := encoder.WriteNamedRootCompound("", root); err != nil {
			c.SetEntitySaveError(err)
			failures = append(failures, err)
			continue
		}
		if len(encoder.Bytes()) > 16<<20 {
			err := fmt.Errorf("entity region payload exceeds limit")
			c.SetEntitySaveError(err)
			failures = append(failures, err)
			continue
		}
		c.SetEntityNBT(encoder.Bytes())
	}
	owners := make(map[protocol.UUID]int32, w.entities.Count())
	for id, base := range w.entities.Entities() {
		if !base.Removed() {
			owners[base.UUID()] = id
		}
	}
	w.projectiles.ResolvePersistentOwners(owners)
	return errors.Join(failures...)
}

// evictChunkEntities releases entity state only after both storage namespaces are durable.
func (w *World) evictChunkEntities(c *chunk.Chunk) {
	for id, base := range w.entities.Entities() {
		if base.Type() == entity.EntityTypePlayer || entity.ChunkKey(entity.BlockToChunk(base.Position().X), entity.BlockToChunk(base.Position().Z)) != c.Key() {
			continue
		}
		w.items.Remove(id)
		w.xpOrbs.Remove(id)
		w.tnts.Remove(id)
		w.fallingBlocks.Remove(id)
		w.mobs.Evict(id)
		w.mobBrains.Remove(id)
		w.projectiles.FinalizeRemovals([]int32{id})
		w.entities.Evict(id)
	}
	delete(w.entityChunks, c)
}

// logEntitySnapshotError reports runtime persistence failures with world context.
func (w *World) logEntitySnapshotError(err error) {
	if err != nil {
		logger.Error("snapshot world entities failed", "world", w.name, "error", err)
	}
}
