package command

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/protocol"
)

// WhitelistProvider abstracts whitelist operations for commands.
type WhitelistProvider interface {
	Enabled() bool
	SetEnabled(v bool)
	Add(uuid protocol.UUID, name string) (bool, error)
	RemoveByName(name string) (bool, error)
	IsWhitelisted(uuid protocol.UUID) bool
	Count() int
	Names() []string
	Load() error
}

// RegisterWhitelistCommands registers the /whitelist command.
func RegisterWhitelistCommands(registry *Registry, wl WhitelistProvider) {
	if wl == nil {
		return
	}
	registry.Register(cmdWhitelist(wl))
}

func cmdWhitelist(wl WhitelistProvider) *Command {
	return &Command{
		Name:            "whitelist",
		Description:     "Manages the server whitelist",
		Usage:           "/whitelist <on|off|add|remove|list|reload>",
		PermissionLevel: 3,
		Children: []*CommandNode{
			LiteralNode("on").Executable(),
			LiteralNode("off").Executable(),
			LiteralNode("list").Executable(),
			LiteralNode("reload").Executable(),
			LiteralNode("add",
				ArgumentNode("player", ParserGameProfile, nil).Executable().
					WithSuggestions("minecraft:ask_server"),
			),
			LiteralNode("remove",
				ArgumentNode("player", ParserGameProfile, nil).Executable().
					WithSuggestions("minecraft:ask_server"),
			),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.Reply("§7Usage: %s", "/whitelist <on|off|add|remove|list|reload>")
				return nil
			}

			sub := strings.ToLower(ctx.Arg(0))
			switch sub {
			case "on":
				if wl.Enabled() {
					ctx.Reply("Whitelist is already turned on")
					return nil
				}
				wl.SetEnabled(true)
				ctx.Reply("Whitelist is now turned on")

			case "off":
				if !wl.Enabled() {
					ctx.Reply("Whitelist is already turned off")
					return nil
				}
				wl.SetEnabled(false)
				ctx.Reply("Whitelist is now turned off")

			case "list":
				names := wl.Names()
				if len(names) == 0 {
					ctx.Reply("There are no whitelisted players")
					return nil
				}
				ctx.Reply("There are %d whitelisted player(s): %s",
					len(names), strings.Join(names, ", "))

			case "reload":
				if err := wl.Load(); err != nil {
					ctx.ReplyError("Failed to reload whitelist: %v", err)
					return nil
				}
				ctx.Reply("Reloaded the whitelist")

			case "add":
				if ctx.ArgCount() < 2 {
					ctx.ReplyError("Usage: /whitelist add <player>")
					return nil
				}
				name := ctx.Arg(1)
				uuid := protocol.OfflinePlayerUUID(name)
				added, err := wl.Add(uuid, name)
				if err != nil {
					ctx.ReplyError("Failed to add player: %v", err)
					return nil
				}
				if !added {
					ctx.Reply("Player is already whitelisted")
					return nil
				}
				ctx.Reply(fmt.Sprintf("Added %s to the whitelist", name))

			case "remove":
				if ctx.ArgCount() < 2 {
					ctx.ReplyError("Usage: /whitelist remove <player>")
					return nil
				}
				name := ctx.Arg(1)
				removed, err := wl.RemoveByName(name)
				if err != nil {
					ctx.ReplyError("Failed to remove player: %v", err)
					return nil
				}
				if !removed {
					ctx.Reply("Player is not whitelisted")
					return nil
				}
				ctx.Reply(fmt.Sprintf("Removed %s from the whitelist", name))

			default:
				ctx.ReplyError("Unknown subcommand: %s", sub)
			}
			return nil
		},
	}
}
