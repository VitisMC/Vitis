package command

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// PlayerLookup is used by commands to find players by name.
type PlayerLookup interface {
	// FindPlayerByName returns a PlayerSender by name, or nil.
	FindPlayerByName(name string) PlayerSender
	// OnlinePlayers returns all online player names.
	OnlinePlayers() []string
}

// ServerControl provides server-level operations for commands.
type ServerControl interface {
	// Stop gracefully shuts down the server.
	Stop()
	// Seed returns the world seed.
	Seed() int64
	// SetTime sets the world time.
	SetTime(time int64)
	// GetTime returns the current world time of day.
	GetTime() int64
	// SetWeather sets the weather state. "clear", "rain", "thunder"
	SetWeather(weather string, duration int)
	// SetGameMode sets a player's game mode by entity ID. 0=survival, 1=creative, 2=adventure, 3=spectator
	SetGameMode(entityID int32, mode int32) error
	// TeleportPlayer teleports a player to x, y, z.
	TeleportPlayer(entityID int32, x, y, z float64) error
	// GiveItem gives an item to a player.
	GiveItem(entityID int32, itemName string, count int) error
	// KillEntity kills an entity by ID.
	KillEntity(entityID int32) error
	// SetDifficulty sets the server difficulty. 0=peaceful, 1=easy, 2=normal, 3=hard
	SetDifficulty(difficulty int) error
	// SetOp sets an operator level for a player.
	SetOp(name string, level int) error
	// RemoveOp removes operator status from a player.
	RemoveOp(name string) error
	// KickPlayer kicks a player with an optional message.
	KickPlayer(name string, reason string) error
	// BroadcastMessage broadcasts a message to all players.
	BroadcastMessage(message string)
	// EnchantItem applies an enchantment to the held item of a player.
	EnchantItem(entityID int32, enchantName string, level int) error
	// SetBlockAt sets a block at coordinates. Returns the placed state ID.
	SetBlockAt(x, y, z int, blockName string) (int32, error)
	// FillBlocks fills a region with a block. Returns count of blocks changed.
	FillBlocks(x1, y1, z1, x2, y2, z2 int, blockName string) (int, error)
	// ClearInventory clears items from a player. Returns count removed.
	ClearInventory(entityID int32, itemName string, maxCount int) (int, error)
	// GetGameRule returns the current value of a game rule.
	GetGameRule(name string) (string, error)
	// SetGameRule sets a game rule value.
	SetGameRule(name, value string) error
	// SetDefaultGameMode sets the default game mode for new players.
	SetDefaultGameMode(mode int32) error
	// SetWorldSpawn sets the world spawn position.
	SetWorldSpawn(x, y, z int) error
	// SetSpawnPoint sets a player's individual spawn point.
	SetSpawnPoint(entityID int32, x, y, z int) error
	// SendTitle sends a title/subtitle to a player.
	SendTitle(entityID int32, title, subtitle string, fadeIn, stay, fadeOut int) error
	// SendActionBar sends an action bar message to a player.
	SendActionBar(entityID int32, text string) error
}

// RegisterBuiltinCommands registers all built-in commands.
func RegisterBuiltinCommands(registry *Registry, players PlayerLookup, server ServerControl) {
	registry.Register(cmdHelp(registry))
	registry.Register(cmdGamemode(players, server))
	registry.Register(cmdTp(players, server))
	registry.Register(cmdGive(players, server))
	registry.Register(cmdTime(server))
	registry.Register(cmdWeather(server))
	registry.Register(cmdSay(server))
	registry.Register(cmdMsg(players))
	registry.Register(cmdKick(players, server))
	registry.Register(cmdStop(server))
	registry.Register(cmdList(players))
	registry.Register(cmdSeed(server))
	registry.Register(cmdDifficulty(server))
	registry.Register(cmdKill(players, server))
	registry.Register(cmdOp(players, server))
	registry.Register(cmdDeop(players, server))
	registry.Register(cmdEnchant(players, server))
	registry.Register(cmdMe(server))
	registry.Register(cmdTellraw(players, server))
	registry.Register(cmdTitle(players, server))
	registry.Register(cmdSetblock(server))
	registry.Register(cmdFill(server))
	registry.Register(cmdClear(players, server))
	registry.Register(cmdGamerule(server))
	registry.Register(cmdDefaultgamemode(server))
	registry.Register(cmdSetworldspawn(server))
	registry.Register(cmdSpawnpoint(players, server))
}

// --- /help ---
func cmdHelp(registry *Registry) *Command {
	return &Command{
		Name:            "help",
		Description:     "Shows a list of available commands",
		Usage:           "/help [command]",
		Aliases:         []string{"?"},
		PermissionLevel: 0,
		Children: []*CommandNode{
			ArgumentNode("command", ParserString, StringSingleWord).Executable().
				WithSuggestions("minecraft:ask_server"),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() > 0 {
				cmdName := ctx.Arg(0)
				cmd, found := registry.Get(cmdName)
				if !found {
					ctx.ReplyError("Unknown command: /%s", cmdName)
					return nil
				}
				ctx.Reply("§6/%s§r - %s", cmd.Name, cmd.Description)
				if cmd.Usage != "" {
					ctx.Reply("§7Usage: %s", cmd.Usage)
				}
				if len(cmd.Aliases) > 0 {
					ctx.Reply("§7Aliases: %s", strings.Join(cmd.Aliases, ", "))
				}
				return nil
			}

			commands := registry.AllVisible(ctx.Sender)
			ctx.Reply("§6--- Available Commands (%d) ---", len(commands))
			for _, cmd := range commands {
				ctx.Reply("§a/%s§r - %s", cmd.Name, cmd.Description)
			}
			return nil
		},
		TabComplete: func(ctx *Context) []string {
			if ctx.ArgCount() <= 1 {
				prefix := strings.ToLower(ctx.Arg(0))
				var suggestions []string
				for _, cmd := range registry.AllVisible(ctx.Sender) {
					if strings.HasPrefix(cmd.Name, prefix) {
						suggestions = append(suggestions, cmd.Name)
					}
				}
				return suggestions
			}
			return nil
		},
	}
}

// --- /gamemode ---
func cmdGamemode(players PlayerLookup, server ServerControl) *Command {
	gameModes := map[string]int32{
		"survival": 0, "s": 0, "0": 0,
		"creative": 1, "c": 1, "1": 1,
		"adventure": 2, "a": 2, "2": 2,
		"spectator": 3, "sp": 3, "3": 3,
	}

	return &Command{
		Name:            "gamemode",
		Description:     "Sets a player's game mode",
		Usage:           "/gamemode <mode> [player]",
		Aliases:         []string{"gm"},
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("gamemode", ParserGameMode, nil,
				ArgumentNode("target", ParserEntity, EntityFlagSingleEntity|EntityFlagOnlyPlayers).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: %s", "/gamemode <mode> [player]")
				return nil
			}

			modeName := strings.ToLower(ctx.Arg(0))
			mode, ok := gameModes[modeName]
			if !ok {
				ctx.ReplyError("Unknown game mode: %s", ctx.Arg(0))
				return nil
			}

			var target PlayerSender
			if ctx.ArgCount() >= 2 {
				target = players.FindPlayerByName(ctx.Arg(1))
				if target == nil {
					ctx.ReplyError("Player not found: %s", ctx.Arg(1))
					return nil
				}
			} else if ps, ok := ctx.Sender.(PlayerSender); ok {
				target = ps
			} else {
				ctx.ReplyError("Must specify a player from console")
				return nil
			}

			modeNames := [4]string{"Survival", "Creative", "Adventure", "Spectator"}
			if err := server.SetGameMode(target.EntityID(), mode); err != nil {
				ctx.ReplyError("Failed to set game mode: %v", err)
				return nil
			}
			ctx.Reply("§aSet %s's game mode to %s", target.Name(), modeNames[mode])
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /tp ---
func cmdTp(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "tp",
		Description:     "Teleports a player",
		Usage:           "/tp <player> | /tp <x> <y> <z> | /tp <player> <x> <y> <z>",
		Aliases:         []string{"teleport"},
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("destination", ParserEntity, EntityFlagSingleEntity|EntityFlagOnlyPlayers).Executable(),
			ArgumentNode("location", ParserVec3, nil).Executable(),
			ArgumentNode("target", ParserEntity, EntityFlagSingleEntity|EntityFlagOnlyPlayers,
				ArgumentNode("destination_player", ParserEntity, EntityFlagSingleEntity|EntityFlagOnlyPlayers).Executable(),
				ArgumentNode("location", ParserVec3, nil).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: %s", ctx.Sender.Name())
				return nil
			}

			// /tp <player> - teleport to player
			if ctx.ArgCount() == 1 {
				sender, ok := ctx.Sender.(PlayerSender)
				if !ok {
					ctx.ReplyError("Must specify coordinates from console")
					return nil
				}
				target := players.FindPlayerByName(ctx.Arg(0))
				if target == nil {
					ctx.ReplyError("Player not found: %s", ctx.Arg(0))
					return nil
				}
				tx, ty, tz := target.Position()
				if err := server.TeleportPlayer(sender.EntityID(), tx, ty, tz); err != nil {
					ctx.ReplyError("Teleport failed: %v", err)
					return nil
				}
				ctx.Reply("§aTeleported to %s", target.Name())
				return nil
			}

			// /tp <x> <y> <z>
			if ctx.ArgCount() == 3 {
				sender, ok := ctx.Sender.(PlayerSender)
				if !ok {
					ctx.ReplyError("Must specify a target from console")
					return nil
				}
				x, y, z, err := parseCoordinates(ctx.Args[0:3], sender)
				if err != nil {
					ctx.ReplyError("Invalid coordinates: %v", err)
					return nil
				}
				if err := server.TeleportPlayer(sender.EntityID(), x, y, z); err != nil {
					ctx.ReplyError("Teleport failed: %v", err)
					return nil
				}
				ctx.Reply("§aTeleported to %.1f, %.1f, %.1f", x, y, z)
				return nil
			}

			// /tp <player> <x> <y> <z>
			if ctx.ArgCount() >= 4 {
				target := players.FindPlayerByName(ctx.Arg(0))
				if target == nil {
					ctx.ReplyError("Player not found: %s", ctx.Arg(0))
					return nil
				}
				x, y, z, err := parseCoordinates(ctx.Args[1:4], target)
				if err != nil {
					ctx.ReplyError("Invalid coordinates: %v", err)
					return nil
				}
				if err := server.TeleportPlayer(target.EntityID(), x, y, z); err != nil {
					ctx.ReplyError("Teleport failed: %v", err)
					return nil
				}
				ctx.Reply("§aTeleported %s to %.1f, %.1f, %.1f", target.Name(), x, y, z)
				return nil
			}

			ctx.Reply("§7Usage: /tp <player> | /tp <x> <y> <z> | /tp <player> <x> <y> <z>")
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /give ---
func cmdGive(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "give",
		Description:     "Gives an item to a player",
		Usage:           "/give <player> <item> [count]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
				ArgumentNode("item", ParserItemStack, nil,
					ArgumentNode("count", ParserInteger, [2]int32{1, 6400}).Executable(),
				).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 2 {
				ctx.Reply("§7Usage: /give <player> <item> [count]")
				return nil
			}
			target := players.FindPlayerByName(ctx.Arg(0))
			if target == nil {
				ctx.ReplyError("Player not found: %s", ctx.Arg(0))
				return nil
			}
			itemName := ctx.Arg(1)
			if !strings.Contains(itemName, ":") {
				itemName = "minecraft:" + itemName
			}
			count := 1
			if ctx.ArgCount() >= 3 {
				c, err := strconv.Atoi(ctx.Arg(2))
				if err != nil || c < 1 {
					ctx.ReplyError("Invalid count: %s", ctx.Arg(2))
					return nil
				}
				count = c
			}
			if err := server.GiveItem(target.EntityID(), itemName, count); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aGave %d × %s to %s", count, itemName, target.Name())
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /time ---
func cmdTime(server ServerControl) *Command {
	timePresets := map[string]int64{
		"day":      1000,
		"noon":     6000,
		"night":    13000,
		"midnight": 18000,
		"sunrise":  23000,
		"sunset":   12000,
	}

	return &Command{
		Name:            "time",
		Description:     "Changes or queries the world time",
		Usage:           "/time set <value> | /time query",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("set",
				ArgumentNode("time", ParserTime, nil).Executable(),
			),
			LiteralNode("query").Executable(),
			LiteralNode("add",
				ArgumentNode("time", ParserTime, nil).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /time set <value> | /time query | /time add <value>")
				return nil
			}

			sub := strings.ToLower(ctx.Arg(0))
			switch sub {
			case "set":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /time set <value|day|noon|night|midnight>")
					return nil
				}
				timeStr := strings.ToLower(ctx.Arg(1))
				if preset, ok := timePresets[timeStr]; ok {
					server.SetTime(preset)
					ctx.Reply("§aSet time to %s (%d)", timeStr, preset)
					return nil
				}
				ticks, err := strconv.ParseInt(timeStr, 10, 64)
				if err != nil {
					ctx.ReplyError("Invalid time: %s", ctx.Arg(1))
					return nil
				}
				server.SetTime(ticks)
				ctx.Reply("§aSet time to %d", ticks)
			case "query":
				ctx.Reply("§aThe time is %d", server.GetTime())
			case "add":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /time add <value>")
					return nil
				}
				ticks, err := strconv.ParseInt(ctx.Arg(1), 10, 64)
				if err != nil {
					ctx.ReplyError("Invalid time: %s", ctx.Arg(1))
					return nil
				}
				current := server.GetTime()
				server.SetTime(current + ticks)
				ctx.Reply("§aAdded %d ticks (now %d)", ticks, current+ticks)
			default:
				ctx.Reply("§7Usage: /time set <value> | /time query | /time add <value>")
			}
			return nil
		},
	}
}

// --- /weather ---
func cmdWeather(server ServerControl) *Command {
	return &Command{
		Name:            "weather",
		Description:     "Sets the weather",
		Usage:           "/weather <clear|rain|thunder> [duration]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("clear",
				ArgumentNode("duration", ParserInteger, [2]int32{0, 1000000}).Executable(),
			).Executable(),
			LiteralNode("rain",
				ArgumentNode("duration", ParserInteger, [2]int32{0, 1000000}).Executable(),
			).Executable(),
			LiteralNode("thunder",
				ArgumentNode("duration", ParserInteger, [2]int32{0, 1000000}).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /weather <clear|rain|thunder> [duration]")
				return nil
			}
			weather := strings.ToLower(ctx.Arg(0))
			if weather != "clear" && weather != "rain" && weather != "thunder" {
				ctx.ReplyError("Unknown weather type: %s", ctx.Arg(0))
				return nil
			}
			duration := 6000
			if ctx.ArgCount() >= 2 {
				d, err := strconv.Atoi(ctx.Arg(1))
				if err != nil || d < 0 {
					ctx.ReplyError("Invalid duration: %s", ctx.Arg(1))
					return nil
				}
				duration = d
			}
			server.SetWeather(weather, duration)
			ctx.Reply("§aSet weather to %s for %d ticks", weather, duration)
			return nil
		},
	}
}

// --- /say ---
func cmdSay(server ServerControl) *Command {
	return &Command{
		Name:            "say",
		Description:     "Broadcasts a message to all players",
		Usage:           "/say <message>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("message", ParserMessage, nil).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /say <message>")
				return nil
			}
			msg := ctx.JoinArgs(0)
			server.BroadcastMessage(fmt.Sprintf("§d[%s] %s", ctx.Sender.Name(), msg))
			return nil
		},
	}
}

// --- /msg ---
func cmdMsg(players PlayerLookup) *Command {
	return &Command{
		Name:            "msg",
		Description:     "Sends a private message to a player",
		Usage:           "/msg <player> <message>",
		Aliases:         []string{"tell", "w"},
		PermissionLevel: 0,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagSingleEntity|EntityFlagOnlyPlayers,
				ArgumentNode("message", ParserMessage, nil).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 2 {
				ctx.Reply("§7Usage: /msg <player> <message>")
				return nil
			}
			target := players.FindPlayerByName(ctx.Arg(0))
			if target == nil {
				ctx.ReplyError("Player not found: %s", ctx.Arg(0))
				return nil
			}
			msg := ctx.JoinArgs(1)
			target.SendMessage(fmt.Sprintf("§7%s whispers to you: %s", ctx.Sender.Name(), msg))
			ctx.Reply("§7You whisper to %s: %s", target.Name(), msg)
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /kick ---
func cmdKick(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "kick",
		Description:     "Kicks a player from the server",
		Usage:           "/kick <player> [reason]",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagSingleEntity|EntityFlagOnlyPlayers,
				ArgumentNode("reason", ParserMessage, nil).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /kick <player> [reason]")
				return nil
			}
			targetName := ctx.Arg(0)
			reason := "Kicked by an operator"
			if ctx.ArgCount() >= 2 {
				reason = ctx.JoinArgs(1)
			}
			if err := server.KickPlayer(targetName, reason); err != nil {
				ctx.ReplyError("Failed to kick %s: %v", targetName, err)
				return nil
			}
			ctx.Reply("§aKicked %s: %s", targetName, reason)
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /stop ---
func cmdStop(server ServerControl) *Command {
	return &Command{
		Name:            "stop",
		Description:     "Stops the server",
		Usage:           "/stop",
		PermissionLevel: 4,
		Execute: func(ctx *Context) error {
			ctx.Reply("§6Stopping the server...")
			server.Stop()
			return nil
		},
	}
}

// --- /list ---
func cmdList(players PlayerLookup) *Command {
	return &Command{
		Name:            "list",
		Description:     "Lists online players",
		Usage:           "/list",
		PermissionLevel: 0,
		Execute: func(ctx *Context) error {
			names := players.OnlinePlayers()
			if len(names) == 0 {
				ctx.Reply("§7No players online")
				return nil
			}
			ctx.Reply("§6Online players (%d): §r%s", len(names), strings.Join(names, ", "))
			return nil
		},
	}
}

// --- /seed ---
func cmdSeed(server ServerControl) *Command {
	return &Command{
		Name:            "seed",
		Description:     "Displays the world seed",
		Usage:           "/seed",
		PermissionLevel: 2,
		Execute: func(ctx *Context) error {
			ctx.Reply("§aSeed: [§6%d§a]", server.Seed())
			return nil
		},
	}
}

// --- /difficulty ---
func cmdDifficulty(server ServerControl) *Command {
	difficulties := map[string]int{
		"peaceful": 0, "p": 0, "0": 0,
		"easy": 1, "e": 1, "1": 1,
		"normal": 2, "n": 2, "2": 2,
		"hard": 3, "h": 3, "3": 3,
	}
	diffNames := [4]string{"Peaceful", "Easy", "Normal", "Hard"}

	return &Command{
		Name:            "difficulty",
		Description:     "Sets the game difficulty",
		Usage:           "/difficulty <peaceful|easy|normal|hard>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("peaceful").Executable(),
			LiteralNode("easy").Executable(),
			LiteralNode("normal").Executable(),
			LiteralNode("hard").Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /difficulty <peaceful|easy|normal|hard>")
				return nil
			}
			diffStr := strings.ToLower(ctx.Arg(0))
			diff, ok := difficulties[diffStr]
			if !ok {
				ctx.ReplyError("Unknown difficulty: %s", ctx.Arg(0))
				return nil
			}
			if err := server.SetDifficulty(diff); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aDifficulty set to %s", diffNames[diff])
			return nil
		},
	}
}

// --- /kill ---
func cmdKill(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "kill",
		Description:     "Kills an entity or the sender",
		Usage:           "/kill [player]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() >= 1 {
				target := players.FindPlayerByName(ctx.Arg(0))
				if target == nil {
					ctx.ReplyError("Player not found: %s", ctx.Arg(0))
					return nil
				}
				if err := server.KillEntity(target.EntityID()); err != nil {
					ctx.ReplyError("Failed: %v", err)
					return nil
				}
				ctx.Reply("§aKilled %s", target.Name())
				return nil
			}

			sender, ok := ctx.Sender.(PlayerSender)
			if !ok {
				ctx.ReplyError("Must specify a target from console")
				return nil
			}
			if err := server.KillEntity(sender.EntityID()); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aKilled %s", sender.Name())
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /op ---
func cmdOp(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "op",
		Description:     "Grants operator status to a player",
		Usage:           "/op <player>",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserGameProfile, nil).Executable().
				WithSuggestions("minecraft:ask_server"),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /op <player>")
				return nil
			}
			if err := server.SetOp(ctx.Arg(0), 4); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aMade %s a server operator", ctx.Arg(0))
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /deop ---
func cmdDeop(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "deop",
		Description:     "Revokes operator status from a player",
		Usage:           "/deop <player>",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserGameProfile, nil).Executable().
				WithSuggestions("minecraft:ask_server"),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /deop <player>")
				return nil
			}
			if err := server.RemoveOp(ctx.Arg(0)); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aRemoved %s as a server operator", ctx.Arg(0))
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- Helpers ---

func tabCompletePlayers(players PlayerLookup) TabCompleter {
	return func(ctx *Context) []string {
		if players == nil {
			return nil
		}
		if ctx.ArgCount() == 0 {
			return players.OnlinePlayers()
		}
		prefix := strings.ToLower(ctx.Arg(ctx.ArgCount() - 1))
		var suggestions []string
		for _, name := range players.OnlinePlayers() {
			if strings.HasPrefix(strings.ToLower(name), prefix) {
				suggestions = append(suggestions, name)
			}
		}
		return suggestions
	}
}

// --- /enchant ---
func cmdEnchant(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "enchant",
		Description:     "Enchants the held item of a player",
		Usage:           "/enchant <player> <enchantment> [level]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
				ArgumentNode("enchantment", ParserResource, nil,
					ArgumentNode("level", ParserInteger, [2]int32{1, 255}).Executable(),
				).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 2 {
				ctx.Reply("§7Usage: /enchant <player> <enchantment> [level]")
				return nil
			}
			target := players.FindPlayerByName(ctx.Arg(0))
			if target == nil {
				ctx.ReplyError("Player not found: %s", ctx.Arg(0))
				return nil
			}
			enchName := ctx.Arg(1)
			if !strings.Contains(enchName, ":") {
				enchName = "minecraft:" + enchName
			}
			level := 1
			if ctx.ArgCount() >= 3 {
				l, err := strconv.Atoi(ctx.Arg(2))
				if err != nil || l < 1 {
					ctx.ReplyError("Invalid level: %s", ctx.Arg(2))
					return nil
				}
				level = l
			}
			if err := server.EnchantItem(target.EntityID(), enchName, level); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aApplied %s level %d to %s's held item", enchName, level, target.Name())
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

func parseCoordinates(args []string, sender PlayerSender) (x, y, z float64, err error) {
	if len(args) < 3 {
		return 0, 0, 0, fmt.Errorf("expected 3 coordinates, got %d", len(args))
	}

	var sx, sy, sz float64
	if sender != nil {
		sx, sy, sz = sender.Position()
	}

	x, err = parseCoord(args[0], sx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid x: %w", err)
	}
	y, err = parseCoord(args[1], sy)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid y: %w", err)
	}
	z, err = parseCoord(args[2], sz)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid z: %w", err)
	}
	return x, y, z, nil
}

func parseCoord(s string, base float64) (float64, error) {
	if s == "~" {
		return base, nil
	}
	if strings.HasPrefix(s, "~") {
		offset, err := strconv.ParseFloat(s[1:], 64)
		if err != nil {
			return 0, err
		}
		return base + offset, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, fmt.Errorf("coordinate out of range")
	}
	return v, nil
}
