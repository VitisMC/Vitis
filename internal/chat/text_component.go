package chat

import (
	"strings"

	"github.com/vitismc/vitis/internal/nbt"
)

// TextComponent represents a Minecraft JSON/NBT text component.
type TextComponent struct {
	Text          string
	Translate     string
	With          []TextComponent
	Extra         []TextComponent
	Color         string
	Bold          *bool
	Italic        *bool
	Underlined    *bool
	Strikethrough *bool
	Obfuscated    *bool
	Font          string
	Insertion     string
	ClickEvent    *ClickEvent
	HoverEvent    *HoverEvent
}

// ClickEvent represents a click event on a text component.
type ClickEvent struct {
	Action string
	Value  string
}

// HoverEvent represents a hover event on a text component.
type HoverEvent struct {
	Action   string
	Contents TextComponent
}

const (
	ClickOpenURL         = "open_url"
	ClickRunCommand      = "run_command"
	ClickSuggestCommand  = "suggest_command"
	ClickCopyToClipboard = "copy_to_clipboard"
)

const (
	HoverShowText   = "show_text"
	HoverShowItem   = "show_item"
	HoverShowEntity = "show_entity"
)

// Text creates a simple text component.
func Text(text string) TextComponent {
	return TextComponent{Text: text}
}

// Colored creates a text component with a color.
func Colored(text, color string) TextComponent {
	return TextComponent{Text: text, Color: color}
}

// Translatable creates a translatable text component.
func Translatable(key string, with ...TextComponent) TextComponent {
	return TextComponent{Translate: key, With: with}
}

// WithExtra returns a copy with appended extra components.
func (t TextComponent) WithExtra(extra ...TextComponent) TextComponent {
	t.Extra = append(t.Extra, extra...)
	return t
}

// WithColor returns a copy with the given color.
func (t TextComponent) WithColor(color string) TextComponent {
	t.Color = color
	return t
}

// WithBold returns a copy with bold set.
func (t TextComponent) WithBold(v bool) TextComponent {
	t.Bold = &v
	return t
}

// WithItalic returns a copy with italic set.
func (t TextComponent) WithItalic(v bool) TextComponent {
	t.Italic = &v
	return t
}

// WithClick returns a copy with a click event.
func (t TextComponent) WithClick(action, value string) TextComponent {
	t.ClickEvent = &ClickEvent{Action: action, Value: value}
	return t
}

// WithHover returns a copy with a hover text event.
func (t TextComponent) WithHover(text TextComponent) TextComponent {
	t.HoverEvent = &HoverEvent{Action: HoverShowText, Contents: text}
	return t
}

// ToNBT serializes the TextComponent to an NBT compound for 1.21.4 wire format.
func (t TextComponent) ToNBT() *nbt.Compound {
	c := nbt.NewCompound()

	if t.Translate != "" {
		c.PutString("type", "translatable")
		c.PutString("translate", t.Translate)
		if len(t.With) > 0 {
			list := nbt.NewList(nbt.TagCompound)
			for _, w := range t.With {
				list.Add(w.ToNBT())
			}
			c.PutList("with", list)
		}
	} else {
		c.PutString("text", t.Text)
	}

	if t.Color != "" {
		c.PutString("color", t.Color)
	}
	if t.Bold != nil {
		c.PutBool("bold", *t.Bold)
	}
	if t.Italic != nil {
		c.PutBool("italic", *t.Italic)
	}
	if t.Underlined != nil {
		c.PutBool("underlined", *t.Underlined)
	}
	if t.Strikethrough != nil {
		c.PutBool("strikethrough", *t.Strikethrough)
	}
	if t.Obfuscated != nil {
		c.PutBool("obfuscated", *t.Obfuscated)
	}
	if t.Font != "" {
		c.PutString("font", t.Font)
	}
	if t.Insertion != "" {
		c.PutString("insertion", t.Insertion)
	}

	if t.ClickEvent != nil {
		ce := nbt.NewCompound()
		ce.PutString("action", t.ClickEvent.Action)
		ce.PutString("value", t.ClickEvent.Value)
		c.PutCompound("click_event", ce)
	}

	if t.HoverEvent != nil {
		he := nbt.NewCompound()
		he.PutString("action", t.HoverEvent.Action)
		he.PutCompound("contents", t.HoverEvent.Contents.ToNBT())
		c.PutCompound("hover_event", he)
	}

	if len(t.Extra) > 0 {
		list := nbt.NewList(nbt.TagCompound)
		for _, e := range t.Extra {
			list.Add(e.ToNBT())
		}
		c.PutList("extra", list)
	}

	return c
}

// EncodeNBT encodes the TextComponent as raw NBT bytes for packet use.
func (t TextComponent) EncodeNBT() []byte {
	enc := nbt.NewEncoder(256)
	_ = enc.WriteRootCompound(t.ToNBT())
	return enc.Bytes()
}

// --- Section sign (§) formatting code support ---

var colorCodes = map[rune]string{
	'0': "black",
	'1': "dark_blue",
	'2': "dark_green",
	'3': "dark_aqua",
	'4': "dark_red",
	'5': "dark_purple",
	'6': "gold",
	'7': "gray",
	'8': "dark_gray",
	'9': "blue",
	'a': "green",
	'b': "aqua",
	'c': "red",
	'd': "light_purple",
	'e': "yellow",
	'f': "white",
}

// FromLegacy parses a string with § formatting codes into a TextComponent.
func FromLegacy(text string) TextComponent {
	runes := []rune(text)
	var root TextComponent
	var current TextComponent
	var hasStyle bool

	for i := 0; i < len(runes); i++ {
		if runes[i] == '§' && i+1 < len(runes) {
			code := runes[i+1]

			if current.Text != "" || hasStyle {
				root.Extra = append(root.Extra, current)
				current = TextComponent{}
				hasStyle = false
			}

			switch {
			case code >= '0' && code <= '9' || code >= 'a' && code <= 'f':
				current.Color = colorCodes[code]
				hasStyle = true
			case code == 'k':
				v := true
				current.Obfuscated = &v
				hasStyle = true
			case code == 'l':
				v := true
				current.Bold = &v
				hasStyle = true
			case code == 'm':
				v := true
				current.Strikethrough = &v
				hasStyle = true
			case code == 'n':
				v := true
				current.Underlined = &v
				hasStyle = true
			case code == 'o':
				v := true
				current.Italic = &v
				hasStyle = true
			case code == 'r':
				current = TextComponent{}
				hasStyle = false
			default:
				current.Text += string(runes[i])
				continue
			}
			i++
			continue
		}
		current.Text += string(runes[i])
	}

	if current.Text != "" || hasStyle {
		root.Extra = append(root.Extra, current)
	}

	if len(root.Extra) == 1 && root.Text == "" {
		return root.Extra[0]
	}

	return root
}

// ToLegacy converts a TextComponent to a legacy §-formatted string.
func ToLegacy(t TextComponent) string {
	var sb strings.Builder
	writeLegacy(&sb, t)
	for _, e := range t.Extra {
		writeLegacy(&sb, e)
	}
	return sb.String()
}

func writeLegacy(sb *strings.Builder, t TextComponent) {
	if t.Color != "" {
		for code, name := range colorCodes {
			if name == t.Color {
				sb.WriteRune('§')
				sb.WriteRune(code)
				break
			}
		}
	}
	if t.Bold != nil && *t.Bold {
		sb.WriteString("§l")
	}
	if t.Italic != nil && *t.Italic {
		sb.WriteString("§o")
	}
	if t.Underlined != nil && *t.Underlined {
		sb.WriteString("§n")
	}
	if t.Strikethrough != nil && *t.Strikethrough {
		sb.WriteString("§m")
	}
	if t.Obfuscated != nil && *t.Obfuscated {
		sb.WriteString("§k")
	}
	sb.WriteString(t.Text)
}

// Plain extracts plain text without formatting.
func Plain(t TextComponent) string {
	var sb strings.Builder
	sb.WriteString(t.Text)
	for _, e := range t.Extra {
		sb.WriteString(Plain(e))
	}
	return sb.String()
}
