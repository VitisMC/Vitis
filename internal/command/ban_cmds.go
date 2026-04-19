package command

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/protocol"
)

// BanProvider abstracts player ban operations for commands.
type BanProvider interface {
	Ban(uuid protocol.UUID, name, reason, source, expires string) error
	Pardon(name string) (bool, error)
	IsBanned(uuid protocol.UUID) bool
	GetBanReason(uuid protocol.UUID) string
	All() []BanEntryInfo
}

// BanEntryInfo is a minimal view of a ban entry for command display.
type BanEntryInfo struct {
	Name   string
	Reason string
}

// IPBanProvider abstracts IP ban operations for commands.
type IPBanProvider interface {
	Ban(ip, reason, source, expires string) error
	Pardon(ip string) (bool, error)
	IsBanned(ip string) bool
}

// BanKicker can kick a player by name.
type BanKicker interface {
	KickPlayer(name string, reason string) error
	FindPlayerByName(name string) PlayerSender
	OnlinePlayers() []string
}

// RegisterBanCommands registers /ban, /ban-ip, /pardon, /pardon-ip, /banlist commands.
func RegisterBanCommands(registry *Registry, bans BanProvider, ipBans IPBanProvider, kicker BanKicker) {
	if bans == nil {
		return
	}
	registry.Register(cmdBan(bans, kicker))
	registry.Register(cmdPardon(bans))
	registry.Register(cmdBanlist(bans))
	if ipBans != nil {
		registry.Register(cmdBanIP(ipBans, kicker))
		registry.Register(cmdPardonIP(ipBans))
	}
}

func cmdBan(bans BanProvider, kicker BanKicker) *Command {
	return &Command{
		Name:            "ban",
		Description:     "Bans a player from the server",
		Usage:           "/ban <player> [reason]",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("player", ParserGameProfile, nil,
				ArgumentNode("reason", ParserString, StringGreedy).Executable(),
			).Executable().WithSuggestions("minecraft:ask_server"),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.ReplyError("Usage: /ban <player> [reason]")
				return nil
			}
			name := ctx.Arg(0)
			reason := "Banned by an operator"
			if ctx.ArgCount() > 1 {
				reason = ctx.JoinArgs(1)
			}

			uuid := protocol.OfflinePlayerUUID(name)
			if kicker != nil {
				if p := kicker.FindPlayerByName(name); p != nil {
					uuid = p.UUID()
				}
			}

			if bans.IsBanned(uuid) {
				ctx.Reply("Player is already banned")
				return nil
			}

			source := ctx.Sender.Name()
			if err := bans.Ban(uuid, name, reason, source, "forever"); err != nil {
				ctx.ReplyError("Failed to ban player: %v", err)
				return nil
			}

			ctx.Reply(fmt.Sprintf("Banned %s: %s", name, reason))

			if kicker != nil {
				_ = kicker.KickPlayer(name, "You are banned from this server!\nReason: "+reason)
			}
			return nil
		},
	}
}

func cmdPardon(bans BanProvider) *Command {
	return &Command{
		Name:            "pardon",
		Description:     "Pardons (unbans) a player",
		Usage:           "/pardon <player>",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("player", ParserGameProfile, nil).Executable().
				WithSuggestions("minecraft:ask_server"),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.ReplyError("Usage: /pardon <player>")
				return nil
			}
			name := ctx.Arg(0)
			removed, err := bans.Pardon(name)
			if err != nil {
				ctx.ReplyError("Failed to pardon player: %v", err)
				return nil
			}
			if !removed {
				ctx.Reply("Player is not banned")
				return nil
			}
			ctx.Reply(fmt.Sprintf("Pardoned %s", name))
			return nil
		},
	}
}

func cmdBanlist(bans BanProvider) *Command {
	return &Command{
		Name:            "banlist",
		Description:     "Shows the ban list",
		Usage:           "/banlist",
		PermissionLevel: 3,
		Children: []*CommandNode{
			LiteralNode("players").Executable(),
			LiteralNode("ips").Executable(),
		},
		Execute: func(ctx *Context) error {
			entries := bans.All()
			if len(entries) == 0 {
				ctx.Reply("There are no bans")
				return nil
			}
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name
			}
			ctx.Reply("There are %d ban(s): %s", len(entries), strings.Join(names, ", "))
			return nil
		},
	}
}

func cmdBanIP(ipBans IPBanProvider, kicker BanKicker) *Command {
	return &Command{
		Name:            "ban-ip",
		Description:     "Bans an IP address",
		Usage:           "/ban-ip <ip> [reason]",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("ip", ParserString, StringSingleWord,
				ArgumentNode("reason", ParserString, StringGreedy).Executable(),
			).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.ReplyError("Usage: /ban-ip <ip> [reason]")
				return nil
			}
			ip := ctx.Arg(0)
			reason := "Banned by an operator"
			if ctx.ArgCount() > 1 {
				reason = ctx.JoinArgs(1)
			}
			source := ctx.Sender.Name()
			if err := ipBans.Ban(ip, reason, source, "forever"); err != nil {
				ctx.ReplyError("Failed to ban IP: %v", err)
				return nil
			}
			ctx.Reply(fmt.Sprintf("Banned IP %s: %s", ip, reason))
			return nil
		},
	}
}

func cmdPardonIP(ipBans IPBanProvider) *Command {
	return &Command{
		Name:            "pardon-ip",
		Description:     "Pardons (unbans) an IP address",
		Usage:           "/pardon-ip <ip>",
		PermissionLevel: 3,
		Children: []*CommandNode{
			ArgumentNode("ip", ParserString, StringSingleWord).Executable(),
		},
		Execute: func(ctx *Context) error {
			if ctx.ArgCount() < 1 {
				ctx.ReplyError("Usage: /pardon-ip <ip>")
				return nil
			}
			ip := ctx.Arg(0)
			removed, err := ipBans.Pardon(ip)
			if err != nil {
				ctx.ReplyError("Failed to pardon IP: %v", err)
				return nil
			}
			if !removed {
				ctx.Reply("IP is not banned")
				return nil
			}
			ctx.Reply(fmt.Sprintf("Pardoned IP %s", ip))
			return nil
		},
	}
}
