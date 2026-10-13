# Commands and permissions

[Русский](../rus/commands.md) · [Documentation](README.md)

Use `/` in game and omit it in console/RCON. `<argument>` is required; `[argument]` is optional. Help and completions follow sender permissions. To grant the first operator, connect the player and run `op PlayerName` in the console.

The number is the minimum operator level. Console/RCON have administrative permissions; player-only commands cannot acquire a console position or inventory.

## Command catalogue

| Syntax | Level | Aliases | Note |
| --- | --- | --- | --- |
| `/ban <player> [reason]` | 3 | — | Connected target uses its UUID; otherwise offline UUID |
| `/ban-ip <ip> [reason]` | 3 | — | — |
| `/banlist` | 3 | — | — |
| `/bossbar <add\|remove\|set\|list\|get> ...` | 2 | — | Not connected to the game system |
| `/chunky <world\|shape\|center\|radius\|selection\|start\|progress\|pause\|continue\|cancel>` | 4 | — | Selection and job controls; see pregeneration |
| `/clear [player] [item] [maxCount]` | 2 | — | — |
| `/defaultgamemode <mode>` | 2 | — | Changes the command adapter value; does not persist it or change the initial join mode |
| `/deop <player>` | 3 | — | — |
| `/difficulty <peaceful\|easy\|normal\|hard>` | 2 | — | — |
| `/effect <give\|clear> <player> [effect] [seconds] [amplifier]` | 2 | — | — |
| `/enchant <player> <enchantment> [level]` | 2 | — | Held-item command; enchanting GUI is unavailable |
| `/fill <x1> <y1> <z1> <x2> <y2> <z2> <block>` | 2 | — | — |
| `/gamemode <mode> [player]` | 2 | gm | — |
| `/gamerule <rule> [value]` | 2 | — | — |
| `/give <player> <item> [count]` | 2 | — | — |
| `/help [command]` | 0 | ? | — |
| `/kick <player> [reason]` | 3 | — | — |
| `/kill [player]` | 2 | — | — |
| `/list` | 0 | — | — |
| `/me <action>` | 0 | — | — |
| `/msg <player> <message>` | 0 | tell, w | — |
| `/op <player>` | 3 | — | — |
| `/pardon <player>` | 3 | — | — |
| `/pardon-ip <ip>` | 3 | — | — |
| `/reload` | 2 | — | Datapacks only |
| `/say <message>` | 2 | — | — |
| `/scoreboard <objectives\|players> ...` | 2 | — | Not connected to the game system |
| `/seed` | 2 | — | — |
| `/setblock <x> <y> <z> <block>` | 2 | — | — |
| `/setworldspawn [x] [y] [z]` | 2 | — | Updates the adapter and clients; does not save the world spawn position |
| `/spawnpoint [player] [x] [y] [z]` | 2 | — | Sends the position to the client; does not save the player respawn position |
| `/speed <blocks/second\|reset>` | 2 | — | Player only; positive ≤200 blocks/s, or reset |
| `/stop` | 4 | — | — |
| `/summon <entity> [x y z]` | 2 | — | — |
| `/team <add\|remove\|join\|leave\|list> ...` | 2 | — | Not connected to the game system |
| `/tellraw <player> <message>` | 2 | — | — |
| `/time set <value> \| /time add <value> \| /time query` | 2 | — | — |
| `/title <player> <title\|subtitle\|actionbar\|clear\|reset\|times> ...` | 2 | — | — |
| `/tp <player> \| /tp <x> <y> <z> \| /tp <player> <x> <y> <z>` | 2 | teleport | — |
| `/weather <clear\|rain\|thunder> [duration]` | 2 | — | — |
| `/whitelist <on\|off\|add\|remove\|list\|reload>` | 3 | — | Enabled flag is not persisted; add uses offline UUID |
| `/worldborder <center\|set\|add\|warning\|get> ...` | 2 | — | Not connected to the game system |
| `/xp <add\|set> <player> <amount> [points\|levels] \| /xp query <player>` | 2 | experience | — |

## Practical limits

- Use explicit names and coordinates. Full support for vanilla selectors, SNBT and the command-block API is unavailable.
- `speed` uses blocks/s, not a Bukkit-style scale. `reset` restores ordinary movement speed.
- See [pregeneration](pregeneration.md) for `chunky`.
- Repeat `whitelist on` after restarting. Name-based whitelist add does not authorize Mojang online UUIDs. Banning a connected player uses the actual UUID; absent players use offline UUIDs.
- Changes to game rules, default mode and spawn positions through commands are not fully persisted. Bed respawn is handled separately in the player lifecycle.

Handlers define exact argument validation and supported subcommands: [builtin](../../internal/command/builtin.go), [gameplay](../../internal/command/gameplay_cmds.go), [multiplayer](../../internal/command/multiplayer_cmds.go), [ban](../../internal/command/ban_cmds.go), [whitelist](../../internal/command/whitelist_cmds.go). The [command bridge](../../internal/session/command_bridge.go) applies mutations. See also the [features](status.md).
