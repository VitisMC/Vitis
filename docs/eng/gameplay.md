# Gameplay systems

[Русский](../rus/gameplay.md) · [Documentation](README.md)

This guide lists game handlers and related core packages. See [project status](status.md) for supported mechanics.

| Action | Handlers and packages |
| --- | --- |
| Movement, teleport, reach | [movement action](../../internal/session/movement_action.go), [play handler](../../internal/session/play_handler.go), entity physics |
| Digging and placement | [dig action](../../internal/session/dig_action.go), [placement](../../internal/session/block_placement.go), block behaviors, mining tracker |
| Containers and inventory | [inventory action](../../internal/session/inventory_action.go), [inventory](../../internal/inventory/), window state IDs and resynchronization |
| Crafting and food | [crafting](../../internal/crafting/), [food](../../internal/food/), play inventory/action handlers |
| Combat, health and equipment | [living entities](../../internal/entity/living.go), [damage](../../internal/entity/damage.go), [equipment sync](../../internal/session/equipment_sync.go) |
| Death, respawn and beds | [player lifecycle](../../internal/session/player_lifecycle.go), bootstrap callbacks, player snapshots |
| Item/XP/TNT/projectiles/mobs | [world simulation](../../internal/world/world.go), entity managers, [AI](../../internal/entity/ai/) and [spawning](../../internal/world/spawning/) |
| Loot and datapack overrides | [loot runtime](../../internal/loot/runtime.go), world drop evaluation |
| Block entities and fluids | [block entities](../../internal/block/entity/), [fluids](../../internal/block/fluid/), world scheduled/block-entity ticks |
| Crop growth, support and portal frames | [random ticks](../../internal/world/random_ticks.go), [neighbor updates](../../internal/world/neighbor_updates.go), [block behavior](../../internal/block/behavior/neighbor.go) |
| Fishing, maps and portals | [fishing](../../internal/fishing/), [mapdata](../../internal/world/mapdata/), [portals](../../internal/world/portal/), play/world adapters |
| Player dimensions and travel | [dimension dispatch](../../internal/session/dimension_handlers.go), [transfers](../../internal/session/dimension_transfer.go), [portal travel](../../internal/session/portal_travel.go) |
| Commands | [command definitions](../../internal/command/), [session bridge](../../internal/session/command_bridge.go) |

## Adding or changing behavior

The protocol decodes client requests, the session checks player state, and world or entity changes follow state ownership rules. The server determines inventory contents, reach, damage, permissions and accepted positions.

Connect the mechanic to a handler or tick and send the required client updates.

For persisted state, identify the player/chunk NBT field and dirty/snapshot path. Test mutation, serialization and reload. For a new client-visible state, also update inventory state IDs, metadata, attributes or packets as appropriate.

## Limitations

Advancement definitions load at startup; full trigger evaluation, rewards and client progress are not connected. Recipe triggers and ingredient placement are connected. Player vehicle control is unavailable.

Scoreboard, team, worldborder and bossbar commands are not connected to the core systems. Container buttons select stonecutter recipes on tick; most other special interfaces, trading and signed chat remain unconnected.

The [event bus](../../internal/event/bus.go) calls subscribers synchronously in priority order. Handlers follow the caller's state-access rules and unsubscribe when their owner closes.

See [extension guides](extending.md) for adding mechanics. Test rules separately and through the connected handlers.
