package command

import (
	"fmt"
	"strconv"
	"strings"
)

// --- /me ---
func cmdMe(server ServerControl) *Command {
	return &Command{
		Name:            "me",
		Description:     "Displays an action message",
		Usage:           "/me <action>",
		PermissionLevel: 0,
		Children: []*CommandNode{
			ArgumentNode("action", ParserMessage, nil).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /me <action>")
				return nil
			}
			action := ctx.JoinArgs(0)
			server.BroadcastMessage(fmt.Sprintf("* %s %s", ctx.Sender.Name(), action))
			return nil
		},
	}
}

// --- /tellraw ---
func cmdTellraw(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "tellraw",
		Description:     "Sends a raw JSON text message to a player",
		Usage:           "/tellraw <player> <message>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
				ArgumentNode("message", ParserComponent, nil).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 2 {
				ctx.Reply("§7Usage: /tellraw <player> <message>")
				return nil
			}
			target := players.FindPlayerByName(ctx.Arg(0))
			if target == nil {
				ctx.ReplyError("Player not found: %s", ctx.Arg(0))
				return nil
			}
			message := ctx.JoinArgs(1)
			target.SendMessage(message)
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /title ---
func cmdTitle(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "title",
		Description:     "Controls title display for a player",
		Usage:           "/title <player> <title|subtitle|actionbar|clear|reset|times> ...",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
				LiteralNode("title",
					ArgumentNode("text", ParserComponent, nil).Executable(),
				),
				LiteralNode("subtitle",
					ArgumentNode("text", ParserComponent, nil).Executable(),
				),
				LiteralNode("actionbar",
					ArgumentNode("text", ParserComponent, nil).Executable(),
				),
				LiteralNode("clear").Executable(),
				LiteralNode("reset").Executable(),
				LiteralNode("times",
					ArgumentNode("fadeIn", ParserInteger, nil,
						ArgumentNode("stay", ParserInteger, nil,
							ArgumentNode("fadeOut", ParserInteger, nil).Executable(),
						),
					),
				),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 2 {
				ctx.Reply("§7Usage: /title <player> <title|subtitle|actionbar|clear|reset|times> ...")
				return nil
			}
			target := players.FindPlayerByName(ctx.Arg(0))
			if target == nil {
				ctx.ReplyError("Player not found: %s", ctx.Arg(0))
				return nil
			}
			action := strings.ToLower(ctx.Arg(1))
			switch action {
			case "title":
				if ctx.ArgCount() < 3 {
					ctx.ReplyError("Missing title text")
					return nil
				}
				text := ctx.JoinArgs(2)
				if err := server.SendTitle(target.EntityID(), text, "", 10, 70, 20); err != nil {
					ctx.ReplyError("Failed: %v", err)
				}
			case "subtitle":
				if ctx.ArgCount() < 3 {
					ctx.ReplyError("Missing subtitle text")
					return nil
				}
				text := ctx.JoinArgs(2)
				if err := server.SendTitle(target.EntityID(), "", text, 0, 0, 0); err != nil {
					ctx.ReplyError("Failed: %v", err)
				}
			case "actionbar":
				if ctx.ArgCount() < 3 {
					ctx.ReplyError("Missing actionbar text")
					return nil
				}
				text := ctx.JoinArgs(2)
				if err := server.SendActionBar(target.EntityID(), text); err != nil {
					ctx.ReplyError("Failed: %v", err)
				}
			case "clear":
				_ = server.SendTitle(target.EntityID(), "", "", 0, 0, 0)
			case "reset":
				_ = server.SendTitle(target.EntityID(), "", "", 10, 70, 20)
			case "times":
				if ctx.ArgCount() < 5 {
					ctx.ReplyError("Usage: /title <player> times <fadeIn> <stay> <fadeOut>")
					return nil
				}
				fadeIn, err1 := strconv.Atoi(ctx.Arg(2))
				stay, err2 := strconv.Atoi(ctx.Arg(3))
				fadeOut, err3 := strconv.Atoi(ctx.Arg(4))
				if err1 != nil || err2 != nil || err3 != nil {
					ctx.ReplyError("Invalid time values")
					return nil
				}
				_ = server.SendTitle(target.EntityID(), "", "", fadeIn, stay, fadeOut)
			default:
				ctx.ReplyError("Unknown action: %s", action)
			}
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /setblock ---
func cmdSetblock(server ServerControl) *Command {
	return &Command{
		Name:            "setblock",
		Description:     "Places a block at a position",
		Usage:           "/setblock <x> <y> <z> <block>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("pos", ParserBlockPos, nil,
				ArgumentNode("block", ParserBlockState, nil).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 4 {
				ctx.Reply("§7Usage: /setblock <x> <y> <z> <block>")
				return nil
			}
			ps, ok := ctx.Sender.(PlayerSender)
			if !ok {
				ctx.ReplyError("This command can only be used by a player")
				return nil
			}
			x, y, z, err := parseCoordinates(ctx.Args[:3], ps)
			if err != nil {
				ctx.ReplyError("%v", err)
				return nil
			}
			blockName := ctx.Arg(3)
			if !strings.Contains(blockName, ":") {
				blockName = "minecraft:" + blockName
			}
			_, err = server.SetBlockAt(int(x), int(y), int(z), blockName)
			if err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aSet block at %d %d %d to %s", int(x), int(y), int(z), blockName)
			return nil
		},
	}
}

// --- /fill ---
func cmdFill(server ServerControl) *Command {
	return &Command{
		Name:            "fill",
		Description:     "Fills a region with a block",
		Usage:           "/fill <x1> <y1> <z1> <x2> <y2> <z2> <block>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("from", ParserBlockPos, nil,
				ArgumentNode("to", ParserBlockPos, nil,
					ArgumentNode("block", ParserBlockState, nil).Executable(),
				),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 7 {
				ctx.Reply("§7Usage: /fill <x1> <y1> <z1> <x2> <y2> <z2> <block>")
				return nil
			}
			ps, ok := ctx.Sender.(PlayerSender)
			if !ok {
				ctx.ReplyError("This command can only be used by a player")
				return nil
			}
			x1, y1, z1, err := parseCoordinates(ctx.Args[:3], ps)
			if err != nil {
				ctx.ReplyError("From: %v", err)
				return nil
			}
			x2, y2, z2, err := parseCoordinates(ctx.Args[3:6], ps)
			if err != nil {
				ctx.ReplyError("To: %v", err)
				return nil
			}
			blockName := ctx.Arg(6)
			if !strings.Contains(blockName, ":") {
				blockName = "minecraft:" + blockName
			}
			count, err := server.FillBlocks(int(x1), int(y1), int(z1), int(x2), int(y2), int(z2), blockName)
			if err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aFilled %d blocks with %s", count, blockName)
			return nil
		},
	}
}

// --- /clear ---
func cmdClear(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "clear",
		Description:     "Clears items from a player's inventory",
		Usage:           "/clear [player] [item] [maxCount]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
				ArgumentNode("item", ParserItemStack, nil,
					ArgumentNode("maxCount", ParserInteger, [2]int32{0, 2147483647}).Executable(),
				).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			var target PlayerSender
			if ctx.ArgCount() >= 1 {
				target = players.FindPlayerByName(ctx.Arg(0))
				if target == nil {
					ctx.ReplyError("Player not found: %s", ctx.Arg(0))
					return nil
				}
			} else if ps, ok := ctx.Sender.(PlayerSender); ok {
				target = ps
			} else {
				ctx.ReplyError("Must specify a player")
				return nil
			}
			itemFilter := ""
			if ctx.ArgCount() >= 2 {
				itemFilter = ctx.Arg(1)
				if !strings.Contains(itemFilter, ":") {
					itemFilter = "minecraft:" + itemFilter
				}
			}
			maxCount := -1
			if ctx.ArgCount() >= 3 {
				c, err := strconv.Atoi(ctx.Arg(2))
				if err != nil || c < 0 {
					ctx.ReplyError("Invalid count: %s", ctx.Arg(2))
					return nil
				}
				maxCount = c
			}
			removed, err := server.ClearInventory(target.EntityID(), itemFilter, maxCount)
			if err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			if removed == 0 {
				ctx.ReplyError("No items were removed")
			} else {
				ctx.Reply("§aRemoved %d items from %s", removed, target.Name())
			}
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /gamerule ---
func cmdGamerule(server ServerControl) *Command {
	return &Command{
		Name:            "gamerule",
		Description:     "Sets or queries a game rule",
		Usage:           "/gamerule <rule> [value]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("rule", ParserString, StringSingleWord,
				ArgumentNode("value", ParserString, StringSingleWord).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /gamerule <rule> [value]")
				return nil
			}
			ruleName := ctx.Arg(0)
			if ctx.ArgCount() == 1 {
				val, err := server.GetGameRule(ruleName)
				if err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				ctx.Reply("§a%s = %s", ruleName, val)
			} else {
				val := ctx.Arg(1)
				if err := server.SetGameRule(ruleName, val); err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				ctx.Reply("§aSet %s to %s", ruleName, val)
			}
			return nil
		},
	}
}

// --- /defaultgamemode ---
func cmdDefaultgamemode(server ServerControl) *Command {
	modeMap := map[string]int32{
		"survival":  0,
		"creative":  1,
		"adventure": 2,
		"spectator": 3,
		"0":         0,
		"1":         1,
		"2":         2,
		"3":         3,
	}

	return &Command{
		Name:            "defaultgamemode",
		Description:     "Sets the default game mode",
		Usage:           "/defaultgamemode <mode>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("mode", ParserGameMode, nil).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /defaultgamemode <mode>")
				return nil
			}
			mode, ok := modeMap[strings.ToLower(ctx.Arg(0))]
			if !ok {
				ctx.ReplyError("Unknown game mode: %s", ctx.Arg(0))
				return nil
			}
			if err := server.SetDefaultGameMode(mode); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			modeNames := []string{"Survival", "Creative", "Adventure", "Spectator"}
			ctx.Reply("§aDefault game mode set to %s", modeNames[mode])
			return nil
		},
	}
}

// --- /setworldspawn ---
func cmdSetworldspawn(server ServerControl) *Command {
	return &Command{
		Name:            "setworldspawn",
		Description:     "Sets the world spawn point",
		Usage:           "/setworldspawn [x] [y] [z]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("pos", ParserBlockPos, nil).Executable(),
		},
		Execute: func(ctx *Context) error {
			ps, ok := ctx.Sender.(PlayerSender)
			if !ok {
				ctx.ReplyError("This command can only be used by a player")
				return nil
			}
			var x, y, z float64
			if ctx.ArgCount() >= 3 {
				var err error
				x, y, z, err = parseCoordinates(ctx.Args[:3], ps)
				if err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
			} else {
				x, y, z = ps.Position()
			}
			if err := server.SetWorldSpawn(int(x), int(y), int(z)); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aSet world spawn to %d %d %d", int(x), int(y), int(z))
			return nil
		},
	}
}

// --- /spawnpoint ---
func cmdSpawnpoint(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "spawnpoint",
		Description:     "Sets a player's spawn point",
		Usage:           "/spawnpoint [player] [x] [y] [z]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
				ArgumentNode("pos", ParserBlockPos, nil).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			var target PlayerSender
			if ctx.ArgCount() >= 1 {
				target = players.FindPlayerByName(ctx.Arg(0))
				if target == nil {
					ctx.ReplyError("Player not found: %s", ctx.Arg(0))
					return nil
				}
			} else if ps, ok := ctx.Sender.(PlayerSender); ok {
				target = ps
			} else {
				ctx.ReplyError("Must specify a player")
				return nil
			}
			ps, ok := ctx.Sender.(PlayerSender)
			if !ok {
				ps = target
			}
			var x, y, z float64
			argOffset := 0
			if ctx.ArgCount() >= 1 && players.FindPlayerByName(ctx.Arg(0)) != nil {
				argOffset = 1
			}
			if ctx.ArgCount() >= argOffset+3 {
				var err error
				x, y, z, err = parseCoordinates(ctx.Args[argOffset:argOffset+3], ps)
				if err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
			} else {
				x, y, z = target.Position()
			}
			if err := server.SetSpawnPoint(target.EntityID(), int(x), int(y), int(z)); err != nil {
				ctx.ReplyError("Failed: %v", err)
				return nil
			}
			ctx.Reply("§aSet spawn point of %s to %d %d %d", target.Name(), int(x), int(y), int(z))
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}

// --- /summon ---
func cmdSummon(server ServerControl) *Command {
	return &Command{
		Name:            "summon",
		Description:     "Summons an entity",
		Usage:           "/summon <entity> [x y z]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			ArgumentNode("entity", ParserResource, nil,
				ArgumentNode("pos", ParserVec3, nil).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /summon <entity> [x y z]")
				return nil
			}
			entityType := ctx.Arg(0)
			if !strings.Contains(entityType, ":") {
				entityType = "minecraft:" + entityType
			}

			var x, y, z float64
			if ctx.ArgCount() >= 4 {
				ps, ok := ctx.Sender.(PlayerSender)
				if !ok {
					ctx.ReplyError("must specify coordinates from console")
					return nil
				}
				var err error
				x, y, z, err = parseCoordinates(ctx.Args[1:4], ps)
				if err != nil {
					ctx.ReplyError("invalid coordinates: %v", err)
					return nil
				}
			} else if ps, ok := ctx.Sender.(PlayerSender); ok {
				x, y, z = ps.Position()
			} else {
				ctx.ReplyError("must specify coordinates from console")
				return nil
			}

			if err := server.SummonMob(entityType, x, y, z); err != nil {
				ctx.ReplyError("%v", err)
				return nil
			}
			ctx.Reply("§aSummoned %s at %.1f %.1f %.1f", entityType, x, y, z)
			return nil
		},
	}
}

// --- /effect ---
func cmdEffect(players PlayerLookup, server ServerControl) *Command {
	return &Command{
		Name:            "effect",
		Description:     "Manages status effects",
		Usage:           "/effect <give|clear> <player> [effect] [seconds] [amplifier]",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("give",
				ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
					ArgumentNode("effect", ParserResource, nil,
						ArgumentNode("seconds", ParserInteger, [2]int32{1, 1000000},
							ArgumentNode("amplifier", ParserInteger, [2]int32{0, 255}).Executable(),
						).Executable(),
					).Executable(),
				),
			),
			LiteralNode("clear",
				ArgumentNode("targets", ParserEntity, EntityFlagOnlyPlayers,
					ArgumentNode("effect", ParserResource, nil).Executable(),
				).Executable(),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /effect <give|clear> <player> [effect] [seconds] [amplifier]")
				return nil
			}

			sub := strings.ToLower(ctx.Arg(0))
			switch sub {
			case "give":
				if ctx.ArgCount() < 3 {
					ctx.ReplyError("Usage: /effect give <player> <effect> [seconds] [amplifier]")
					return nil
				}
				target := players.FindPlayerByName(ctx.Arg(1))
				if target == nil {
					ctx.ReplyError("player not found: %s", ctx.Arg(1))
					return nil
				}
				effectName := ctx.Arg(2)
				if !strings.Contains(effectName, ":") {
					effectName = "minecraft:" + effectName
				}
				seconds := 30
				if ctx.ArgCount() >= 4 {
					d, err := strconv.Atoi(ctx.Arg(3))
					if err != nil || d < 1 {
						ctx.ReplyError("invalid duration: %s", ctx.Arg(3))
						return nil
					}
					seconds = d
				}
				amplifier := 0
				if ctx.ArgCount() >= 5 {
					a, err := strconv.Atoi(ctx.Arg(4))
					if err != nil || a < 0 {
						ctx.ReplyError("invalid amplifier: %s", ctx.Arg(4))
						return nil
					}
					amplifier = a
				}
				durationTicks := int32(seconds * 20)
				if err := server.ApplyEffect(target.EntityID(), effectName, durationTicks, int32(amplifier)); err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				ctx.Reply("§aApplied %s (amplifier %d) for %ds to %s", effectName, amplifier, seconds, target.Name())

			case "clear":
				if ctx.ArgCount() < 2 {
					ctx.ReplyError("Usage: /effect clear <player> [effect]")
					return nil
				}
				target := players.FindPlayerByName(ctx.Arg(1))
				if target == nil {
					ctx.ReplyError("player not found: %s", ctx.Arg(1))
					return nil
				}
				effectFilter := ""
				if ctx.ArgCount() >= 3 {
					effectFilter = ctx.Arg(2)
					if !strings.Contains(effectFilter, ":") {
						effectFilter = "minecraft:" + effectFilter
					}
				}
				if err := server.ClearEffects(target.EntityID(), effectFilter); err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				if effectFilter == "" {
					ctx.Reply("§aCleared all effects from %s", target.Name())
				} else {
					ctx.Reply("§aCleared %s from %s", effectFilter, target.Name())
				}

			default:
				ctx.ReplyError("unknown subcommand: %s (use 'give' or 'clear')", sub)
			}
			return nil
		},
		TabComplete: tabCompletePlayers(players),
	}
}
