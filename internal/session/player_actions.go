package session

import (
	"github.com/vitismc/vitis/internal/entity"
	"github.com/vitismc/vitis/internal/entity/metadata"
	"github.com/vitismc/vitis/internal/food"
	"github.com/vitismc/vitis/internal/mining"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

const playerActionsKey = "play_player_actions"

// playerActions retains only the current world's transient action trackers.
type playerActions struct {
	mining  *mining.Tracker
	eating  *food.EatingTracker
	players *PlayerManager
	id      int32
	binding *worldBinding
}

// rememberPlayerActions binds transient trackers when an action executes on tick.
func rememberPlayerActions(s Session, players *PlayerManager, digging *mining.Tracker, eating *food.EatingTracker) {
	concrete, ok := s.(*DefaultSession)
	if !ok || concrete == nil || s.Player() == nil {
		return
	}
	binding := concrete.world.Load()
	if stored, ok := concrete.SessionData().Load(playerActionsKey); ok {
		if actions, ok := stored.(*playerActions); ok && actions != nil && actions.mining == digging && actions.eating == eating && actions.binding == binding {
			return
		}
		CancelPlayerActions(s)
	}
	concrete.SessionData().Store(playerActionsKey, &playerActions{mining: digging, eating: eating, players: players, id: s.Player().ID(), binding: binding})
}

// CancelPlayerActions releases mining, eating and shield state on the common tick.
func CancelPlayerActions(s Session) {
	concrete, ok := s.(*DefaultSession)
	if !ok || concrete == nil {
		return
	}
	var players *PlayerManager
	if stored, found := concrete.SessionData().LoadAndDelete(playerActionsKey); found {
		if actions, ok := stored.(*playerActions); ok && actions != nil {
			players = actions.players
			if actions.mining != nil {
				if dig := actions.mining.Cancel(actions.id); dig != nil && players != nil {
					players.Broadcast(&play.BlockBreakAnimation{EntityID: actions.id, Position: play.BlockPos{X: int32(dig.X), Y: int32(dig.Y), Z: int32(dig.Z)}, DestroyStage: -1})
				}
			}
			if actions.eating != nil {
				actions.eating.Cancel(actions.id)
			}
		}
	}
	if player, ok := s.Player().(*entity.Player); ok && player != nil && player.Living().IsBlocking() {
		player.Living().SetBlocking(false)
		if players != nil {
			state := metadata.New()
			state.SetByte(metadata.IndexLivingHandStates, 0)
			players.Broadcast(&play.SetEntityMetadata{EntityID: player.ID(), Metadata: state.Encode()})
		}
	}
}
