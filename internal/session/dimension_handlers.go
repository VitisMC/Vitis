package session

import (
	"fmt"
	"sort"

	"github.com/vitismc/vitis/internal/protocol"
	play "github.com/vitismc/vitis/internal/protocol/packets/play"
)

// PlayWorld binds an immutable play configuration to one dimension's runtime.
type PlayWorld struct {
	Dimension string
	Accessor  WorldAccessor
	Config    PlayBootstrapConfig
}

// RegisterDimensionPlayHandlers dispatches gameplay through the player's world.
// The legacy RegisterPlayHandlers adapter remains available for single-world users.
func RegisterDimensionPlayHandlers(router PacketRouter, players *PlayerManager, worlds []PlayWorld) error {
	if router == nil || players == nil || len(worlds) == 0 {
		return fmt.Errorf("register dimension handlers: missing runtime")
	}
	tables := make(map[string]map[handlerKey]PacketHandler, len(worlds))
	accessors := make(map[string]WorldAccessor, len(worlds))
	var keys []handlerKey
	for _, world := range worlds {
		if world.Dimension == "" || world.Accessor == nil || tables[world.Dimension] != nil {
			return fmt.Errorf("register dimension handlers: invalid or duplicate dimension %q", world.Dimension)
		}
		local := NewPacketRouter()
		config := world.Config
		config.DimensionName = world.Dimension
		// These private adapters are only called inside dimensionPacketCommand.
		// Nested actions execute immediately to retain the original TCP ordering.
		var tickAccessor WorldAccessor = &tickWorldAccessor{WorldAccessor: world.Accessor}
		if accessor, ok := world.Accessor.(*DefaultWorldAccessor); ok {
			inline := *accessor
			inline.SubmitTask = func(task func()) bool { task(); return true }
			tickAccessor = &inline
		}
		if err := RegisterPlayHandlers(local, config, players.ForDimension(world.Dimension), tickAccessor); err != nil {
			return err
		}
		table := local.handlers.Load().(map[handlerKey]PacketHandler)
		if keys == nil {
			for key := range table {
				keys = append(keys, key)
			}
		} else {
			if len(table) != len(keys) {
				return fmt.Errorf("register dimension handlers: inconsistent packet set")
			}
			for _, key := range keys {
				if table[key] == nil {
					return fmt.Errorf("register dimension handlers: missing packet %d", key.id)
				}
			}
		}
		tables[world.Dimension], accessors[world.Dimension] = table, world.Accessor
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].id < keys[j].id })
	for _, key := range keys {
		if err := router.Register(key.state, key.id, func(s Session, packet protocol.Packet) error {
			if s == nil || packet == nil {
				return protocol.ErrNilPacket
			}
			dimension := SessionDimension(s)
			handler := tables[dimension][key]
			if handler == nil {
				return fmt.Errorf("play dimension %q is unavailable", dimension)
			}
			// Transport acknowledgements only mutate atomic session counters.
			switch packet.(type) {
			case *play.KeepAliveServerbound, *play.ChunkBatchReceived:
				return handler(s, packet)
			}
			if !accessors[dimension].Submit(&dimensionPacketCommand{session: s, packet: packet, handler: handler}) {
				return fmt.Errorf("world command queue is unavailable or full")
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// tickWorldAccessor keeps nested actions in the current packet's tick position.
type tickWorldAccessor struct{ WorldAccessor }

// Submit executes an action within an already queued dimension packet.
func (a *tickWorldAccessor) Submit(command WorldCommand) bool {
	if command == nil {
		return false
	}
	command.Execute()
	return true
}

// dimensionPacketCommand runs stateful handler validation on the common tick.
type dimensionPacketCommand struct {
	session Session
	packet  protocol.Packet
	handler PacketHandler
}

// OwnerSession identifies the connection that submitted this packet.
func (c *dimensionPacketCommand) OwnerSession() Session { return c.session }

// Execute dispatches a decoded packet without mutating state on the network loop.
func (c *dimensionPacketCommand) Execute() {
	if s, ok := c.session.(*DefaultSession); ok && s.hasPendingTeleport() {
		switch packet := c.packet.(type) {
		case *play.ConfirmTeleportation, *play.PlayerLoaded, *play.TickEnd,
			*play.ServerboundChatMessage, *play.ChatCommand, *play.ChatCommandSigned:
		case *play.BlockDig:
			_ = s.Send(&play.AcknowledgeBlockChange{Sequence: packet.Sequence})
			return
		case *play.UseItemOn:
			_ = s.Send(&play.AcknowledgeBlockChange{Sequence: packet.Sequence})
			return
		case *play.UseItem:
			_ = s.Send(&play.AcknowledgeBlockChange{Sequence: packet.Sequence})
			return
		default:
			return
		}
	}
	if err := c.handler(c.session, c.packet); err != nil {
		_ = c.session.ForceClose(err)
	}
}
