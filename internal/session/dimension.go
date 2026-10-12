package session

import "strings"

// worldBinding identifies one uninterrupted membership in a dimension.
type worldBinding struct{ dimension string }

// BindWorldDimension publishes prepared world membership on the owning tick.
// Rebinding the same dimension also invalidates actions queued before a respawn.
func (s *DefaultSession) BindWorldDimension(dimension string) bool {
	if s == nil || !strings.Contains(dimension, ":") {
		return false
	}
	s.world.Store(&worldBinding{dimension: dimension})
	return true
}

// WorldDimension returns the dimension attached to this session.
func (s *DefaultSession) WorldDimension() string {
	if s == nil {
		return "minecraft:overworld"
	}
	if binding := s.world.Load(); binding != nil {
		return binding.dimension
	}
	return "minecraft:overworld"
}

// SessionDimension resolves world membership while preserving legacy sessions.
func SessionDimension(s Session) string {
	if bound, ok := s.(interface{ WorldDimension() string }); ok {
		return bound.WorldDimension()
	}
	return "minecraft:overworld"
}

// Global returns the admission and tab-list manager backing a dimension view.
func (pm *PlayerManager) Global() *PlayerManager {
	if pm != nil && pm.parent != nil {
		return pm.parent
	}
	return pm
}

// ForDimension returns a view with dimension-local gameplay broadcasts and lookups.
func (pm *PlayerManager) ForDimension(dimension string) *PlayerManager {
	if pm == nil {
		return nil
	}
	return &PlayerManager{parent: pm.Global(), dimension: dimension}
}

// contains checks whether a player belongs to this gameplay view.
func (pm *PlayerManager) contains(player *OnlinePlayer) bool {
	return player != nil && (pm.dimension == "" || SessionDimension(player.Session) == pm.dimension)
}
