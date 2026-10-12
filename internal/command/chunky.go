package command

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/vitismc/vitis/internal/world"
)

// ChunkyControl connects commands to the server's world pregeneration service.
type ChunkyControl interface {
	DefaultChunkyWorld() string
	StartChunky(world, shape string, centerX, centerZ, radius int) error
	PauseChunky(world string) error
	ContinueChunky(world string) error
	CancelChunky(world string) error
	ChunkyStatus(world string) (world.PregenerationStatus, error)
}

type chunkySelection struct {
	mu      sync.Mutex
	world   string
	shape   string
	centerX int
	centerZ int
	radius  int
}

func cmdChunky(control ChunkyControl) *Command {
	selection := &chunkySelection{world: control.DefaultChunkyWorld(), shape: "square", radius: 500}
	subcommands := []string{"world", "shape", "center", "radius", "selection", "start", "progress", "pause", "continue", "cancel"}
	children := make([]*CommandNode, 0, len(subcommands))
	for _, name := range subcommands {
		children = append(children, LiteralNode(name, ArgumentNode("args", ParserString, StringGreedy).Executable()).Executable())
	}
	return &Command{
		Name: "chunky", Description: "Pre-generates world chunks", Usage: "/chunky <world|shape|center|radius|selection|start|progress|pause|continue|cancel>", PermissionLevel: 4,
		Children: children,
		TabComplete: func(ctx *Context) []string {
			if ctx.ArgCount() <= 1 {
				return subcommands
			}
			if ctx.Arg(0) == "shape" {
				return []string{"square", "circle"}
			}
			return nil
		},
		Execute: func(ctx *Context) error {
			selection.mu.Lock()
			defer selection.mu.Unlock()
			if ctx.ArgCount() == 0 {
				ctx.Reply("§e%s", "/chunky <world|shape|center|radius|selection|start|progress|pause|continue|cancel>")
				return nil
			}
			action := strings.ToLower(ctx.Arg(0))
			switch action {
			case "world":
				if ctx.ArgCount() != 2 {
					ctx.ReplyError("Usage: /chunky world <name>")
					return nil
				}
				if _, err := control.ChunkyStatus(ctx.Arg(1)); err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				selection.world = ctx.Arg(1)
				ctx.Reply("§aSelected world: %s", selection.world)
			case "shape":
				if ctx.ArgCount() != 2 || (ctx.Arg(1) != "square" && ctx.Arg(1) != "circle") {
					ctx.ReplyError("Usage: /chunky shape <square|circle>")
					return nil
				}
				selection.shape = ctx.Arg(1)
				ctx.Reply("§aSelected shape: %s", selection.shape)
			case "center":
				if ctx.ArgCount() == 1 {
					player, ok := ctx.Sender.(PlayerSender)
					if !ok {
						ctx.ReplyError("Console must specify x and z")
						return nil
					}
					x, _, z := player.Position()
					if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(z) || math.IsInf(z, 0) {
						ctx.ReplyError("Invalid position")
						return nil
					}
					selection.centerX, selection.centerZ = int(math.Floor(x)), int(math.Floor(z))
				} else if ctx.ArgCount() == 3 {
					x, errX := strconv.Atoi(ctx.Arg(1))
					z, errZ := strconv.Atoi(ctx.Arg(2))
					if errX != nil || errZ != nil {
						ctx.ReplyError("Usage: /chunky center <x> <z>")
						return nil
					}
					selection.centerX, selection.centerZ = x, z
				} else {
					ctx.ReplyError("Usage: /chunky center [x z]")
					return nil
				}
				ctx.Reply("§aSelected center: %d, %d", selection.centerX, selection.centerZ)
			case "radius":
				if ctx.ArgCount() != 2 {
					ctx.ReplyError("Usage: /chunky radius <blocks>")
					return nil
				}
				radius, err := strconv.Atoi(ctx.Arg(1))
				if err != nil || radius <= 0 || radius > 30000 {
					ctx.ReplyError("Radius must be 1..30000 blocks")
					return nil
				}
				selection.radius = radius
				ctx.Reply("§aSelected radius: %d blocks", radius)
			case "selection":
				ctx.Reply("§eWorld %s, shape %s, center %d %d, radius %d blocks", selection.world, selection.shape, selection.centerX, selection.centerZ, selection.radius)
			case "start":
				if ctx.ArgCount() != 1 {
					ctx.ReplyError("Usage: /chunky start")
					return nil
				}
				if err := control.StartChunky(selection.world, selection.shape, selection.centerX, selection.centerZ, selection.radius); err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				ctx.Reply("§aPregeneration started in %s", selection.world)
			case "progress", "pause", "continue", "cancel":
				if ctx.ArgCount() > 2 {
					ctx.ReplyError("Usage: /chunky %s [world]", ctx.Arg(0))
					return nil
				}
				name := selection.world
				if ctx.ArgCount() == 2 {
					name = ctx.Arg(1)
				}
				var err error
				switch action {
				case "pause":
					err = control.PauseChunky(name)
				case "continue":
					err = control.ContinueChunky(name)
				case "cancel":
					err = control.CancelChunky(name)
				}
				if err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				status, err := control.ChunkyStatus(name)
				if err != nil {
					ctx.ReplyError("%v", err)
					return nil
				}
				if status.Total == 0 {
					ctx.Reply("§eNo pregeneration job in %s", name)
					return nil
				}
				percent := 100 * float64(status.Completed) / float64(status.Total)
				ctx.Reply("§e%s: %s %.1f%% (%d/%d chunks, %d new), average %.1f chunks/s, maximum %.1f chunks/s, active %s", status.World, status.State, percent, status.Completed, status.Total, status.Generated, status.Average, status.Maximum, status.Elapsed.Round(1e9))
				if status.Error != "" {
					ctx.ReplyError("Last error: %s", status.Error)
				}
			default:
				ctx.ReplyError("Unknown subcommand: %s", ctx.Arg(0))
			}
			return nil
		},
	}
}
