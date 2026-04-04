package command

import (
	"fmt"
	"strings"

	"github.com/vitismc/vitis/internal/protocol"
)

// Sender represents the source of a command (player, console, etc).
type Sender interface {
	// Name returns the display name of the sender.
	Name() string
	// SendMessage sends a text message to the sender.
	SendMessage(text string)
	// HasPermission checks whether the sender has the given permission level.
	HasPermission(level int) bool
	// IsPlayer returns true if the sender is a player session.
	IsPlayer() bool
}

// PlayerSender extends Sender with player-specific accessors.
type PlayerSender interface {
	Sender
	// UUID returns the player's UUID.
	UUID() protocol.UUID
	// EntityID returns the player's entity ID.
	EntityID() int32
	// Position returns the player's current x, y, z.
	Position() (x, y, z float64)
	// GameMode returns the player's current game mode.
	GameMode() int32
}

// Context carries execution state for a command invocation.
type Context struct {
	// Sender is the command source.
	Sender Sender
	// RawInput is the original command string (without leading /).
	RawInput string
	// Label is the command name that was used (may be an alias).
	Label string
	// Args is the parsed argument list (split by whitespace).
	Args []string
}

// ArgCount returns the number of arguments.
func (c *Context) ArgCount() int {
	return len(c.Args)
}

// Arg returns argument at index, or empty string.
func (c *Context) Arg(index int) string {
	if index < 0 || index >= len(c.Args) {
		return ""
	}
	return c.Args[index]
}

// JoinArgs joins arguments from the given start index.
func (c *Context) JoinArgs(from int) string {
	if from < 0 || from >= len(c.Args) {
		return ""
	}
	return strings.Join(c.Args[from:], " ")
}

// Reply sends a message back to the command sender.
func (c *Context) Reply(format string, args ...interface{}) {
	if c.Sender != nil {
		c.Sender.SendMessage(fmt.Sprintf(format, args...))
	}
}

// ReplyError sends a red error message back to the command sender.
func (c *Context) ReplyError(format string, args ...interface{}) {
	if c.Sender != nil {
		c.Sender.SendMessage("§c" + fmt.Sprintf(format, args...))
	}
}

// Executor is the function signature for command execution.
type Executor func(ctx *Context) error

// TabCompleter returns suggestions for tab completion.
type TabCompleter func(ctx *Context) []string

// Command defines a server command.
type Command struct {
	// Name is the primary command name (e.g. "gamemode").
	Name string
	// Description is a short description shown in /help.
	Description string
	// Usage is the usage string (e.g. "/gamemode <mode> [player]").
	Usage string
	// Aliases are alternative names for this command.
	Aliases []string
	// PermissionLevel is the minimum permission level required (0-4).
	PermissionLevel int
	// Execute is the command handler.
	Execute Executor
	// TabComplete provides tab completion suggestions.
	TabComplete TabCompleter
	// Children are subcommands for Brigadier tree building.
	Children []*CommandNode
}

// CommandNode represents a node in the Brigadier command tree.
type CommandNode struct {
	// Type: 0=root, 1=literal, 2=argument
	Type int
	// Name is the node name (command name for literal, arg name for argument).
	Name string
	// Parser is the Brigadier parser identifier (for argument nodes).
	Parser string
	// Properties are parser-specific properties.
	Properties interface{}
	// IsExecutable marks whether this node can be executed.
	IsExecutable bool
	// Children of this node.
	Children []*CommandNode
	// SuggestionsType is the custom suggestions type identifier.
	SuggestionsType string
}

const (
	NodeTypeRoot     = 0
	NodeTypeLiteral  = 1
	NodeTypeArgument = 2
)

// Brigadier parser identifiers for 1.21.4.
const (
	ParserBool          = "brigadier:bool"
	ParserFloat         = "brigadier:float"
	ParserDouble        = "brigadier:double"
	ParserInteger       = "brigadier:integer"
	ParserLong          = "brigadier:long"
	ParserString        = "brigadier:string"
	ParserEntity        = "minecraft:entity"
	ParserGameProfile   = "minecraft:game_profile"
	ParserBlockPos      = "minecraft:block_pos"
	ParserColumnPos     = "minecraft:column_pos"
	ParserVec3          = "minecraft:vec3"
	ParserVec2          = "minecraft:vec2"
	ParserMessage       = "minecraft:message"
	ParserComponent     = "minecraft:component"
	ParserGameMode      = "minecraft:gamemode"
	ParserTime          = "minecraft:time"
	ParserResourceOrTag = "minecraft:resource_or_tag"
	ParserResource      = "minecraft:resource"
	ParserDimension     = "minecraft:dimension"
	ParserEntitySummon  = "minecraft:entity_summon"
	ParserItemStack     = "minecraft:item_stack"
	ParserBlockState    = "minecraft:block_state"
	ParserObjective     = "minecraft:objective"
	ParserScoreHolder   = "minecraft:score_holder"
)

// StringMode for brigadier:string parser.
type StringMode int32

const (
	StringSingleWord StringMode = 0
	StringQuotable   StringMode = 1
	StringGreedy     StringMode = 2
)

// EntityFlags for minecraft:entity parser.
type EntityFlags byte

const (
	EntityFlagSingleEntity EntityFlags = 0x01
	EntityFlagOnlyPlayers  EntityFlags = 0x02
)

// LiteralNode creates a literal command node.
func LiteralNode(name string, children ...*CommandNode) *CommandNode {
	return &CommandNode{
		Type:     NodeTypeLiteral,
		Name:     name,
		Children: children,
	}
}

// ArgumentNode creates an argument command node.
func ArgumentNode(name, parser string, properties interface{}, children ...*CommandNode) *CommandNode {
	return &CommandNode{
		Type:       NodeTypeArgument,
		Name:       name,
		Parser:     parser,
		Properties: properties,
		Children:   children,
	}
}

// Executable marks a node as executable.
func (n *CommandNode) Executable() *CommandNode {
	n.IsExecutable = true
	return n
}

// WithSuggestions sets a custom suggestions type on the node.
func (n *CommandNode) WithSuggestions(suggestionsType string) *CommandNode {
	n.SuggestionsType = suggestionsType
	return n
}
