package command

import (
	"strconv"
	"strings"
)

type ScoreboardProvider interface {
	AddObjective(name, displayName string, renderType int32) bool
	RemoveObjective(name string) bool
	SetDisplaySlot(slot int32, objectiveName string)
	SetScore(objectiveName, entityName string, value int32)
	ResetScore(objectiveName, entityName string)
	ResetAllScores(entityName string)
	GetScore(objectiveName, entityName string) (int32, bool)
	ListObjectives() []string
	ListScores(objectiveName string) map[string]int32
	CreateTeam(name, displayName string) bool
	RemoveTeam(name string) bool
	TeamAddMembers(teamName string, members []string) bool
	TeamRemoveMembers(teamName string, members []string) bool
	ListTeams() []string
}

type WorldBorderProvider interface {
	SetCenter(x, z float64)
	SetSize(diameter float64)
	LerpSize(target float64, millis int64)
	Diameter() float64
	Center() (float64, float64)
	SetWarningBlocks(blocks int32)
	SetWarningTime(time int32)
	WarningBlocks() int32
	WarningTime() int32
}

type BossBarProvider interface {
	CreateBar(id, title string, color, division int32)
	RemoveBar(id string) bool
	SetBarTitle(id, title string)
	SetBarHealth(id string, health float32)
	SetBarColor(id string, color int32)
	SetBarStyle(id string, division int32)
	ListBars() []string
}

func RegisterMultiplayerCommands(registry *Registry, sb ScoreboardProvider, wb WorldBorderProvider, bb BossBarProvider) {
	registry.Register(cmdScoreboard(sb))
	registry.Register(cmdTeam(sb))
	registry.Register(cmdWorldBorder(wb))
	registry.Register(cmdBossBar(bb))
}

func cmdScoreboard(sb ScoreboardProvider) *Command {
	return &Command{
		Name:            "scoreboard",
		Description:     "Manages scoreboard objectives and scores",
		Usage:           "/scoreboard <objectives|players> ...",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("objectives",
				LiteralNode("add",
					ArgumentNode("name", ParserString, StringSingleWord,
						ArgumentNode("displayName", ParserString, StringGreedy).Executable(),
					).Executable(),
				),
				LiteralNode("remove",
					ArgumentNode("name", ParserObjective, nil).Executable(),
				),
				LiteralNode("list").Executable(),
				LiteralNode("setdisplay",
					ArgumentNode("slot", ParserString, StringSingleWord,
						ArgumentNode("objective", ParserObjective, nil).Executable(),
					).Executable(),
				),
			),
			LiteralNode("players",
				LiteralNode("set",
					ArgumentNode("entity", ParserScoreHolder, nil,
						ArgumentNode("objective", ParserObjective, nil,
							ArgumentNode("score", ParserInteger, nil).Executable(),
						),
					),
				),
				LiteralNode("add",
					ArgumentNode("entity", ParserScoreHolder, nil,
						ArgumentNode("objective", ParserObjective, nil,
							ArgumentNode("score", ParserInteger, nil).Executable(),
						),
					),
				),
				LiteralNode("remove",
					ArgumentNode("entity", ParserScoreHolder, nil,
						ArgumentNode("objective", ParserObjective, nil,
							ArgumentNode("score", ParserInteger, nil).Executable(),
						),
					),
				),
				LiteralNode("reset",
					ArgumentNode("entity", ParserScoreHolder, nil,
						ArgumentNode("objective", ParserObjective, nil).Executable(),
					).Executable(),
				),
				LiteralNode("list",
					ArgumentNode("entity", ParserScoreHolder, nil).Executable(),
				),
			),
		},
		Execute: func(ctx *Context) error {
			if sb == nil {
				ctx.ReplyError("Scoreboard system is not configured")
				return nil
			}
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /scoreboard <objectives|players> ...")
				return nil
			}
			sub := strings.ToLower(ctx.Arg(0))
			switch sub {
			case "objectives":
				return execScoreboardObjectives(ctx, sb)
			case "players":
				return execScoreboardPlayers(ctx, sb)
			default:
				ctx.ReplyError("Unknown subcommand: %s", sub)
			}
			return nil
		},
		TabComplete: func(ctx *Context) []string {
			switch ctx.ArgCount() {
			case 1:
				return filterPrefix(ctx.Arg(0), "objectives", "players")
			case 2:
				sub := strings.ToLower(ctx.Arg(0))
				if sub == "objectives" {
					return filterPrefix(ctx.Arg(1), "add", "remove", "list", "setdisplay")
				}
				if sub == "players" {
					return filterPrefix(ctx.Arg(1), "set", "add", "remove", "reset", "list")
				}
			case 3:
				sub := strings.ToLower(ctx.Arg(0))
				action := strings.ToLower(ctx.Arg(1))
				if sub == "objectives" && (action == "remove" || action == "setdisplay") && sb != nil {
					return filterPrefix(ctx.Arg(2), sb.ListObjectives()...)
				}
			}
			return nil
		},
	}
}

func execScoreboardObjectives(ctx *Context, sb ScoreboardProvider) error {
	if ctx.ArgCount() < 2 {
		ctx.Reply("§7Usage: /scoreboard objectives <add|remove|list|setdisplay>")
		return nil
	}
	action := strings.ToLower(ctx.Arg(1))
	switch action {
	case "add":
		if ctx.ArgCount() < 3 {
			ctx.Reply("§7Usage: /scoreboard objectives add <name> [displayName]")
			return nil
		}
		name := ctx.Arg(2)
		displayName := name
		if ctx.ArgCount() > 3 {
			displayName = ctx.JoinArgs(3)
		}
		if sb.AddObjective(name, displayName, 0) {
			ctx.Reply("§aAdded objective '%s'", name)
		} else {
			ctx.ReplyError("Objective '%s' already exists", name)
		}
	case "remove":
		if ctx.ArgCount() < 3 {
			ctx.Reply("§7Usage: /scoreboard objectives remove <name>")
			return nil
		}
		name := ctx.Arg(2)
		if sb.RemoveObjective(name) {
			ctx.Reply("§aRemoved objective '%s'", name)
		} else {
			ctx.ReplyError("No objective '%s' found", name)
		}
	case "list":
		objs := sb.ListObjectives()
		if len(objs) == 0 {
			ctx.Reply("§7No objectives")
		} else {
			ctx.Reply("§6Objectives (%d): %s", len(objs), strings.Join(objs, ", "))
		}
	case "setdisplay":
		if ctx.ArgCount() < 3 {
			ctx.Reply("§7Usage: /scoreboard objectives setdisplay <slot> [objective]")
			return nil
		}
		slotName := strings.ToLower(ctx.Arg(2))
		slotMap := map[string]int32{"list": 0, "sidebar": 1, "belowname": 2, "below_name": 2}
		slot, ok := slotMap[slotName]
		if !ok {
			ctx.ReplyError("Unknown display slot: %s", slotName)
			return nil
		}
		objName := ""
		if ctx.ArgCount() > 3 {
			objName = ctx.Arg(3)
		}
		sb.SetDisplaySlot(slot, objName)
		if objName == "" {
			ctx.Reply("§aCleared display slot '%s'", slotName)
		} else {
			ctx.Reply("§aSet display slot '%s' to '%s'", slotName, objName)
		}
	default:
		ctx.ReplyError("Unknown subcommand: objectives %s", action)
	}
	return nil
}

func execScoreboardPlayers(ctx *Context, sb ScoreboardProvider) error {
	if ctx.ArgCount() < 2 {
		ctx.Reply("§7Usage: /scoreboard players <set|add|remove|reset|list>")
		return nil
	}
	action := strings.ToLower(ctx.Arg(1))
	switch action {
	case "set":
		if ctx.ArgCount() < 5 {
			ctx.Reply("§7Usage: /scoreboard players set <entity> <objective> <score>")
			return nil
		}
		entity := ctx.Arg(2)
		objective := ctx.Arg(3)
		value, err := strconv.Atoi(ctx.Arg(4))
		if err != nil {
			ctx.ReplyError("Invalid score: %s", ctx.Arg(4))
			return nil
		}
		sb.SetScore(objective, entity, int32(value))
		ctx.Reply("§aSet %s's score in '%s' to %d", entity, objective, value)
	case "add":
		if ctx.ArgCount() < 5 {
			ctx.Reply("§7Usage: /scoreboard players add <entity> <objective> <score>")
			return nil
		}
		entity := ctx.Arg(2)
		objective := ctx.Arg(3)
		delta, err := strconv.Atoi(ctx.Arg(4))
		if err != nil {
			ctx.ReplyError("Invalid score: %s", ctx.Arg(4))
			return nil
		}
		current, _ := sb.GetScore(objective, entity)
		sb.SetScore(objective, entity, current+int32(delta))
		ctx.Reply("§aAdded %d to %s's score in '%s' (now %d)", delta, entity, objective, current+int32(delta))
	case "remove":
		if ctx.ArgCount() < 5 {
			ctx.Reply("§7Usage: /scoreboard players remove <entity> <objective> <score>")
			return nil
		}
		entity := ctx.Arg(2)
		objective := ctx.Arg(3)
		delta, err := strconv.Atoi(ctx.Arg(4))
		if err != nil {
			ctx.ReplyError("Invalid score: %s", ctx.Arg(4))
			return nil
		}
		current, _ := sb.GetScore(objective, entity)
		sb.SetScore(objective, entity, current-int32(delta))
		ctx.Reply("§aRemoved %d from %s's score in '%s' (now %d)", delta, entity, objective, current-int32(delta))
	case "reset":
		if ctx.ArgCount() < 3 {
			ctx.Reply("§7Usage: /scoreboard players reset <entity> [objective]")
			return nil
		}
		entity := ctx.Arg(2)
		if ctx.ArgCount() > 3 {
			sb.ResetScore(ctx.Arg(3), entity)
			ctx.Reply("§aReset %s's score in '%s'", entity, ctx.Arg(3))
		} else {
			sb.ResetAllScores(entity)
			ctx.Reply("§aReset all scores for %s", entity)
		}
	case "list":
		if ctx.ArgCount() < 3 {
			ctx.Reply("§7Usage: /scoreboard players list <entity>")
			return nil
		}
		entity := ctx.Arg(2)
		found := false
		for _, obj := range sb.ListObjectives() {
			if v, ok := sb.GetScore(obj, entity); ok {
				ctx.Reply("§a%s: %d (%s)", entity, v, obj)
				found = true
			}
		}
		if !found {
			ctx.Reply("§7No scores found for %s", entity)
		}
	default:
		ctx.ReplyError("Unknown subcommand: players %s", action)
	}
	return nil
}

func cmdTeam(sb ScoreboardProvider) *Command {
	return &Command{
		Name:            "team",
		Description:     "Manages teams",
		Usage:           "/team <add|remove|join|leave|list> ...",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("add",
				ArgumentNode("name", ParserString, StringSingleWord,
					ArgumentNode("displayName", ParserString, StringGreedy).Executable(),
				).Executable(),
			),
			LiteralNode("remove",
				ArgumentNode("name", ParserString, StringSingleWord).Executable(),
			),
			LiteralNode("join",
				ArgumentNode("team", ParserString, StringSingleWord,
					ArgumentNode("members", ParserString, StringGreedy).Executable(),
				),
			),
			LiteralNode("leave",
				ArgumentNode("members", ParserString, StringGreedy).Executable(),
			),
			LiteralNode("list",
				ArgumentNode("team", ParserString, StringSingleWord).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if sb == nil {
				ctx.ReplyError("Team system is not configured")
				return nil
			}
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /team <add|remove|join|leave|list>")
				return nil
			}
			action := strings.ToLower(ctx.Arg(0))
			switch action {
			case "add":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /team add <name> [displayName]")
					return nil
				}
				name := ctx.Arg(1)
				displayName := name
				if ctx.ArgCount() > 2 {
					displayName = ctx.JoinArgs(2)
				}
				if sb.CreateTeam(name, displayName) {
					ctx.Reply("§aCreated team '%s'", name)
				} else {
					ctx.ReplyError("Team '%s' already exists", name)
				}
			case "remove":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /team remove <name>")
					return nil
				}
				if sb.RemoveTeam(ctx.Arg(1)) {
					ctx.Reply("§aRemoved team '%s'", ctx.Arg(1))
				} else {
					ctx.ReplyError("No team '%s' found", ctx.Arg(1))
				}
			case "join":
				if ctx.ArgCount() < 3 {
					ctx.Reply("§7Usage: /team join <team> <members...>")
					return nil
				}
				teamName := ctx.Arg(1)
				members := ctx.Args[2:]
				if sb.TeamAddMembers(teamName, members) {
					ctx.Reply("§aAdded %d members to '%s'", len(members), teamName)
				} else {
					ctx.ReplyError("Team '%s' not found", teamName)
				}
			case "leave":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /team leave <members...>")
					return nil
				}
				members := ctx.Args[1:]
				for _, team := range sb.ListTeams() {
					sb.TeamRemoveMembers(team, members)
				}
				ctx.Reply("§aRemoved %d members from all teams", len(members))
			case "list":
				if ctx.ArgCount() > 1 {
					ctx.Reply("§6Team '%s' members (use /team list to see all teams)", ctx.Arg(1))
				} else {
					teams := sb.ListTeams()
					if len(teams) == 0 {
						ctx.Reply("§7No teams")
					} else {
						ctx.Reply("§6Teams (%d): %s", len(teams), strings.Join(teams, ", "))
					}
				}
			default:
				ctx.ReplyError("Unknown subcommand: %s", action)
			}
			return nil
		},
		TabComplete: func(ctx *Context) []string {
			switch ctx.ArgCount() {
			case 1:
				return filterPrefix(ctx.Arg(0), "add", "remove", "join", "leave", "list")
			case 2:
				action := strings.ToLower(ctx.Arg(0))
				if (action == "remove" || action == "join" || action == "list") && sb != nil {
					return filterPrefix(ctx.Arg(1), sb.ListTeams()...)
				}
			}
			return nil
		},
	}
}

func cmdWorldBorder(wb WorldBorderProvider) *Command {
	return &Command{
		Name:            "worldborder",
		Description:     "Manages the world border",
		Usage:           "/worldborder <center|set|add|warning|get> ...",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("center",
				ArgumentNode("x", ParserDouble, nil,
					ArgumentNode("z", ParserDouble, nil).Executable(),
				),
			),
			LiteralNode("set",
				ArgumentNode("diameter", ParserDouble, nil,
					ArgumentNode("time", ParserInteger, nil).Executable(),
				).Executable(),
			),
			LiteralNode("add",
				ArgumentNode("diameter", ParserDouble, nil,
					ArgumentNode("time", ParserInteger, nil).Executable(),
				).Executable(),
			),
			LiteralNode("warning",
				LiteralNode("time",
					ArgumentNode("seconds", ParserInteger, nil).Executable(),
				),
				LiteralNode("distance",
					ArgumentNode("blocks", ParserInteger, nil).Executable(),
				),
			),
			LiteralNode("get").Executable(),
		},
		Execute: func(ctx *Context) error {
			if wb == nil {
				ctx.ReplyError("World border is not configured")
				return nil
			}
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /worldborder <center|set|add|warning|get>")
				return nil
			}
			action := strings.ToLower(ctx.Arg(0))
			switch action {
			case "center":
				if ctx.ArgCount() < 3 {
					ctx.Reply("§7Usage: /worldborder center <x> <z>")
					return nil
				}
				x, err := strconv.ParseFloat(ctx.Arg(1), 64)
				if err != nil {
					ctx.ReplyError("Invalid x: %s", ctx.Arg(1))
					return nil
				}
				z, err := strconv.ParseFloat(ctx.Arg(2), 64)
				if err != nil {
					ctx.ReplyError("Invalid z: %s", ctx.Arg(2))
					return nil
				}
				wb.SetCenter(x, z)
				ctx.Reply("§aSet world border center to %.1f, %.1f", x, z)
			case "set":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /worldborder set <diameter> [time]")
					return nil
				}
				diameter, err := strconv.ParseFloat(ctx.Arg(1), 64)
				if err != nil || diameter <= 0 {
					ctx.ReplyError("Invalid diameter: %s", ctx.Arg(1))
					return nil
				}
				if ctx.ArgCount() > 2 {
					seconds, err := strconv.Atoi(ctx.Arg(2))
					if err != nil || seconds < 0 {
						ctx.ReplyError("Invalid time: %s", ctx.Arg(2))
						return nil
					}
					wb.LerpSize(diameter, int64(seconds)*1000)
					ctx.Reply("§aSet world border to %.1f over %d seconds", diameter, seconds)
				} else {
					wb.SetSize(diameter)
					ctx.Reply("§aSet world border to %.1f", diameter)
				}
			case "add":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /worldborder add <diameter> [time]")
					return nil
				}
				delta, err := strconv.ParseFloat(ctx.Arg(1), 64)
				if err != nil {
					ctx.ReplyError("Invalid diameter: %s", ctx.Arg(1))
					return nil
				}
				target := wb.Diameter() + delta
				if target < 1 {
					target = 1
				}
				if ctx.ArgCount() > 2 {
					seconds, err := strconv.Atoi(ctx.Arg(2))
					if err != nil || seconds < 0 {
						ctx.ReplyError("Invalid time: %s", ctx.Arg(2))
						return nil
					}
					wb.LerpSize(target, int64(seconds)*1000)
					ctx.Reply("§aGrowing world border to %.1f over %d seconds", target, seconds)
				} else {
					wb.SetSize(target)
					ctx.Reply("§aSet world border to %.1f", target)
				}
			case "warning":
				if ctx.ArgCount() < 3 {
					ctx.Reply("§7Usage: /worldborder warning <time|distance> <value>")
					return nil
				}
				warnType := strings.ToLower(ctx.Arg(1))
				value, err := strconv.Atoi(ctx.Arg(2))
				if err != nil || value < 0 {
					ctx.ReplyError("Invalid value: %s", ctx.Arg(2))
					return nil
				}
				switch warnType {
				case "time":
					wb.SetWarningTime(int32(value))
					ctx.Reply("§aSet warning time to %d seconds", value)
				case "distance":
					wb.SetWarningBlocks(int32(value))
					ctx.Reply("§aSet warning distance to %d blocks", value)
				default:
					ctx.ReplyError("Unknown warning type: %s (expected time or distance)", warnType)
				}
			case "get":
				cx, cz := wb.Center()
				ctx.Reply("§6World border: diameter=%.1f center=(%.1f, %.1f) warning=%d blocks, %d seconds",
					wb.Diameter(), cx, cz, wb.WarningBlocks(), wb.WarningTime())
			default:
				ctx.ReplyError("Unknown subcommand: %s", action)
			}
			return nil
		},
		TabComplete: func(ctx *Context) []string {
			switch ctx.ArgCount() {
			case 1:
				return filterPrefix(ctx.Arg(0), "center", "set", "add", "warning", "get")
			case 2:
				if strings.ToLower(ctx.Arg(0)) == "warning" {
					return filterPrefix(ctx.Arg(1), "time", "distance")
				}
			}
			return nil
		},
	}
}

func cmdBossBar(bb BossBarProvider) *Command {
	return &Command{
		Name:            "bossbar",
		Description:     "Manages boss bars",
		Usage:           "/bossbar <add|remove|set|list|get> ...",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("add",
				ArgumentNode("id", ParserString, StringSingleWord,
					ArgumentNode("name", ParserString, StringGreedy).Executable(),
				),
			),
			LiteralNode("remove",
				ArgumentNode("id", ParserString, StringSingleWord).Executable(),
			),
			LiteralNode("set",
				ArgumentNode("id", ParserString, StringSingleWord,
					LiteralNode("name",
						ArgumentNode("name", ParserString, StringGreedy).Executable(),
					),
					LiteralNode("color",
						ArgumentNode("color", ParserString, StringSingleWord).Executable(),
					),
					LiteralNode("style",
						ArgumentNode("style", ParserString, StringSingleWord).Executable(),
					),
					LiteralNode("value",
						ArgumentNode("value", ParserInteger, nil).Executable(),
					),
				),
			),
			LiteralNode("list").Executable(),
		},
		Execute: func(ctx *Context) error {
			if bb == nil {
				ctx.ReplyError("Boss bar system is not configured")
				return nil
			}
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: /bossbar <add|remove|set|list>")
				return nil
			}
			action := strings.ToLower(ctx.Arg(0))
			switch action {
			case "add":
				if ctx.ArgCount() < 3 {
					ctx.Reply("§7Usage: /bossbar add <id> <name>")
					return nil
				}
				id := ctx.Arg(1)
				name := ctx.JoinArgs(2)
				bb.CreateBar(id, name, 0, 0)
				ctx.Reply("§aCreated boss bar '%s'", id)
			case "remove":
				if ctx.ArgCount() < 2 {
					ctx.Reply("§7Usage: /bossbar remove <id>")
					return nil
				}
				if bb.RemoveBar(ctx.Arg(1)) {
					ctx.Reply("§aRemoved boss bar '%s'", ctx.Arg(1))
				} else {
					ctx.ReplyError("No boss bar '%s' found", ctx.Arg(1))
				}
			case "set":
				if ctx.ArgCount() < 4 {
					ctx.Reply("§7Usage: /bossbar set <id> <name|color|style|value> <value>")
					return nil
				}
				id := ctx.Arg(1)
				prop := strings.ToLower(ctx.Arg(2))
				val := ctx.JoinArgs(3)
				switch prop {
				case "name":
					bb.SetBarTitle(id, val)
					ctx.Reply("§aSet boss bar '%s' name to '%s'", id, val)
				case "color":
					colorMap := map[string]int32{"pink": 0, "blue": 1, "red": 2, "green": 3, "yellow": 4, "purple": 5, "white": 6}
					c, ok := colorMap[strings.ToLower(val)]
					if !ok {
						ctx.ReplyError("Unknown color: %s", val)
						return nil
					}
					bb.SetBarColor(id, c)
					ctx.Reply("§aSet boss bar '%s' color to %s", id, val)
				case "style":
					styleMap := map[string]int32{"progress": 0, "notched_6": 1, "notched_10": 2, "notched_12": 3, "notched_20": 4}
					st, ok := styleMap[strings.ToLower(val)]
					if !ok {
						ctx.ReplyError("Unknown style: %s", val)
						return nil
					}
					bb.SetBarStyle(id, st)
					ctx.Reply("§aSet boss bar '%s' style to %s", id, val)
				case "value":
					v, err := strconv.Atoi(val)
					if err != nil || v < 0 || v > 100 {
						ctx.ReplyError("Invalid value (0-100): %s", val)
						return nil
					}
					bb.SetBarHealth(id, float32(v)/100.0)
					ctx.Reply("§aSet boss bar '%s' value to %d%%", id, v)
				default:
					ctx.ReplyError("Unknown property: %s", prop)
				}
			case "list":
				bars := bb.ListBars()
				if len(bars) == 0 {
					ctx.Reply("§7No boss bars")
				} else {
					ctx.Reply("§6Boss bars (%d): %s", len(bars), strings.Join(bars, ", "))
				}
			default:
				ctx.ReplyError("Unknown subcommand: %s", action)
			}
			return nil
		},
		TabComplete: func(ctx *Context) []string {
			switch ctx.ArgCount() {
			case 1:
				return filterPrefix(ctx.Arg(0), "add", "remove", "set", "list")
			case 2:
				action := strings.ToLower(ctx.Arg(0))
				if (action == "remove" || action == "set" || action == "get") && bb != nil {
					return filterPrefix(ctx.Arg(1), bb.ListBars()...)
				}
			case 3:
				if strings.ToLower(ctx.Arg(0)) == "set" {
					return filterPrefix(ctx.Arg(2), "name", "color", "style", "value")
				}
			}
			return nil
		},
	}
}

func filterPrefix(input string, options ...string) []string {
	prefix := strings.ToLower(input)
	var result []string
	for _, o := range options {
		if strings.HasPrefix(strings.ToLower(o), prefix) {
			result = append(result, o)
		}
	}
	return result
}
