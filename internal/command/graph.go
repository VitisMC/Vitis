package command

import (
	"github.com/vitismc/vitis/internal/protocol"
)

// parserIDs maps Brigadier parser names to protocol registry IDs for 1.21.4.
// These are the parser indices used in the Declare Commands packet.
var parserIDs = map[string]int32{
	ParserBool:          0,
	ParserFloat:         1,
	ParserDouble:        2,
	ParserInteger:       3,
	ParserLong:          4,
	ParserString:        5,
	ParserEntity:        6,
	ParserGameProfile:   7,
	ParserBlockPos:      8,
	ParserColumnPos:     9,
	ParserVec3:          10,
	ParserVec2:          11,
	ParserBlockState:    12,
	ParserItemStack:     14,
	ParserComponent:     17,
	ParserMessage:       19,
	ParserObjective:     23,
	ParserScoreHolder:   30,
	ParserDimension:     40,
	ParserGameMode:      41,
	ParserTime:          42,
	ParserResourceOrTag: 43,
	ParserResource:      45,
	ParserEntitySummon:  46,
}

// graphNode is a flattened Brigadier tree node for packet encoding.
type graphNode struct {
	flags    byte
	children []int32
	redirect int32
	name     string
	parser   int32
	props    interface{}
	suggest  string
}

// EncodeCommandGraph builds the Brigadier Declare Commands packet payload from a command registry.
func EncodeCommandGraph(registry *Registry, sender Sender) []byte {
	buf := protocol.NewBuffer(512)

	commands := registry.AllVisible(sender)

	var nodes []graphNode

	// Node 0: root
	rootNode := graphNode{
		flags:    NodeTypeRoot,
		redirect: -1,
	}
	nodes = append(nodes, rootNode)
	rootChildren := make([]int32, 0, len(commands))

	for _, cmd := range commands {
		if cmd.Children != nil && len(cmd.Children) > 0 {
			// Build custom tree
			cmdIdx := int32(len(nodes))
			rootChildren = append(rootChildren, cmdIdx)
			flattenCustomTree(cmd, &nodes)
		} else {
			// Auto-build from command definition
			cmdIdx := int32(len(nodes))
			rootChildren = append(rootChildren, cmdIdx)
			autoFlattenCommand(cmd, &nodes)
		}

		// Also register aliases as literal nodes redirecting to the command
		for _, alias := range cmd.Aliases {
			rootChildren = append(rootChildren, int32(len(nodes)))

			cmdNodeIdx := findCommandNodeIndex(nodes, cmd.Name)
			aliasNode := graphNode{
				flags:    NodeTypeLiteral | 0x08, // redirect flag
				name:     alias,
				redirect: cmdNodeIdx,
			}
			nodes = append(nodes, aliasNode)
		}
	}

	nodes[0].children = rootChildren

	// Encode
	buf.WriteVarInt(int32(len(nodes)))

	for _, node := range nodes {
		flags := node.flags
		if node.flags&0x08 == 0 && node.redirect >= 0 {
			flags |= 0x08 // set redirect bit
		}
		if node.suggest != "" {
			flags |= 0x10 // custom suggestions
		}

		_ = buf.WriteByte(flags)

		// Children
		buf.WriteVarInt(int32(len(node.children)))
		for _, child := range node.children {
			buf.WriteVarInt(child)
		}

		// Redirect
		if flags&0x08 != 0 {
			buf.WriteVarInt(node.redirect)
		}

		// Name (literal and argument nodes)
		nodeType := flags & 0x03
		if nodeType == NodeTypeLiteral || nodeType == NodeTypeArgument {
			_ = buf.WriteString(node.name)
		}

		// Parser (argument nodes)
		if nodeType == NodeTypeArgument {
			buf.WriteVarInt(node.parser)
			encodeParserProperties(buf, node.parser, node.props)
		}

		// Custom suggestions
		if flags&0x10 != 0 {
			_ = buf.WriteString(node.suggest)
		}
	}

	// Root index
	buf.WriteVarInt(0)

	return buf.Bytes()
}

func findCommandNodeIndex(nodes []graphNode, name string) int32 {
	for i, n := range nodes {
		if n.name == name && (n.flags&0x03) == NodeTypeLiteral {
			return int32(i)
		}
	}
	return 0
}

func flattenCustomTree(cmd *Command, nodes *[]graphNode) {
	// Add the command literal node
	cmdIdx := int32(len(*nodes))
	cmdNode := graphNode{
		flags:    NodeTypeLiteral | 0x04, // executable
		name:     cmd.Name,
		redirect: -1,
	}
	*nodes = append(*nodes, cmdNode)

	// Flatten children
	childIndices := make([]int32, 0, len(cmd.Children))
	for _, child := range cmd.Children {
		childIdx := flattenNode(child, nodes)
		childIndices = append(childIndices, childIdx)
	}
	(*nodes)[cmdIdx].children = childIndices
}

func flattenNode(node *CommandNode, nodes *[]graphNode) int32 {
	idx := int32(len(*nodes))

	flags := byte(node.Type & 0x03)
	if node.IsExecutable {
		flags |= 0x04
	}

	fn := graphNode{
		flags:    flags,
		name:     node.Name,
		suggest:  node.SuggestionsType,
		redirect: -1,
	}

	if node.Type == NodeTypeArgument {
		parserID, ok := parserIDs[node.Parser]
		if !ok {
			parserID = parserIDs[ParserString]
		}
		fn.parser = parserID
		fn.props = node.Properties
	}

	*nodes = append(*nodes, fn)

	childIndices := make([]int32, 0, len(node.Children))
	for _, child := range node.Children {
		childIdx := flattenNode(child, nodes)
		childIndices = append(childIndices, childIdx)
	}
	(*nodes)[idx].children = childIndices

	return idx
}

func autoFlattenCommand(cmd *Command, nodes *[]graphNode) {
	// Simple command with just the literal node, marked executable
	cmdNode := graphNode{
		flags:    NodeTypeLiteral | 0x04, // executable
		name:     cmd.Name,
		redirect: -1,
	}
	*nodes = append(*nodes, cmdNode)
}

func encodeParserProperties(buf *protocol.Buffer, parserID int32, props interface{}) {
	switch parserID {
	case 1: // brigadier:float
		if p, ok := props.([2]float32); ok {
			_ = buf.WriteByte(0x03)
			buf.WriteFloat32(p[0])
			buf.WriteFloat32(p[1])
		} else {
			_ = buf.WriteByte(0x00)
		}
	case 2: // brigadier:double
		if p, ok := props.([2]float64); ok {
			_ = buf.WriteByte(0x03)
			buf.WriteFloat64(p[0])
			buf.WriteFloat64(p[1])
		} else {
			_ = buf.WriteByte(0x00)
		}
	case 3: // brigadier:integer
		if p, ok := props.([2]int32); ok {
			_ = buf.WriteByte(0x03)
			buf.WriteInt32(p[0])
			buf.WriteInt32(p[1])
		} else {
			_ = buf.WriteByte(0x00)
		}
	case 4: // brigadier:long
		if p, ok := props.([2]int64); ok {
			_ = buf.WriteByte(0x03)
			buf.WriteInt64(p[0])
			buf.WriteInt64(p[1])
		} else {
			_ = buf.WriteByte(0x00)
		}
	case 5: // brigadier:string
		mode := StringSingleWord
		if p, ok := props.(StringMode); ok {
			mode = p
		}
		buf.WriteVarInt(int32(mode))
	case 6: // minecraft:entity
		flags := EntityFlagOnlyPlayers
		if p, ok := props.(EntityFlags); ok {
			flags = p
		}
		_ = buf.WriteByte(byte(flags))
	case 30: // minecraft:score_holder
		if p, ok := props.(bool); ok && p {
			_ = buf.WriteByte(0x01)
		} else {
			_ = buf.WriteByte(0x00)
		}
	case 42: // minecraft:time
		minVal := int32(0)
		if p, ok := props.(int32); ok {
			minVal = p
		}
		buf.WriteInt32(minVal)
	case 43, 44: // minecraft:resource_or_tag, minecraft:resource_or_tag_key
		registry := "minecraft:block"
		if p, ok := props.(string); ok {
			registry = p
		}
		_ = buf.WriteString(registry)
	case 45, 46: // minecraft:resource, minecraft:resource_key
		registry := "minecraft:block"
		if p, ok := props.(string); ok {
			registry = p
		}
		_ = buf.WriteString(registry)
	}
}
