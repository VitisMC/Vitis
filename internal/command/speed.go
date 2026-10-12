package command

import (
	"math"
	"strconv"
	"strings"
)

const maxCommandSpeed = 200.0

type playerSpeedController interface {
	SetPlayerSpeed(entityID int32, blocksPerSecond float64) error
}

// cmdSpeed changes the invoking player's walking and flying speeds.
func cmdSpeed(server ServerControl) *Command {
	return &Command{
		Name:            "speed",
		Description:     "Sets movement speed in blocks per second",
		Usage:           "/speed <blocks/second|reset>",
		PermissionLevel: 2,
		Children: []*CommandNode{
			LiteralNode("reset").Executable(),
			ArgumentNode("blocks_per_second", ParserDouble, nil).Executable(),
		},
		Execute: func(ctx *Context) error {
			if !ctx.Sender.IsPlayer() {
				ctx.ReplyError("This command is only available to players")
				return nil
			}
			if ctx.ArgCount() != 1 {
				ctx.ReplyError("Usage: /speed <blocks/second|reset>")
				return nil
			}
			player, ok := ctx.Sender.(PlayerSender)
			if !ok {
				ctx.ReplyError("Player is unavailable")
				return nil
			}
			controller, ok := server.(playerSpeedController)
			if !ok {
				ctx.ReplyError("Speed control is unavailable")
				return nil
			}
			if strings.EqualFold(ctx.Arg(0), "reset") {
				if err := controller.SetPlayerSpeed(player.EntityID(), 0); err != nil {
					ctx.ReplyError("Failed to reset speed: %v", err)
					return nil
				}
				ctx.Reply("§aMovement speed reset")
				return nil
			}
			speed, err := strconv.ParseFloat(ctx.Arg(0), 64)
			if err != nil || math.IsNaN(speed) || math.IsInf(speed, 0) || speed <= 0 || speed > maxCommandSpeed {
				ctx.ReplyError("Speed must be greater than 0 and at most %g blocks/s", maxCommandSpeed)
				return nil
			}
			if err := controller.SetPlayerSpeed(player.EntityID(), speed); err != nil {
				ctx.ReplyError("Failed to set speed: %v", err)
				return nil
			}
			ctx.Reply("§aMovement speed set to %g blocks/s", speed)
			return nil
		},
	}
}
