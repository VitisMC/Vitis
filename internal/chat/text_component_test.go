package chat

import (
	"testing"
)

func TestTextPlain(t *testing.T) {
	tc := Text("Hello, World!")
	if tc.Text != "Hello, World!" {
		t.Fatalf("expected 'Hello, World!', got %q", tc.Text)
	}
}

func TestColored(t *testing.T) {
	tc := Colored("Red text", "red")
	if tc.Color != "red" || tc.Text != "Red text" {
		t.Fatalf("unexpected: %+v", tc)
	}
}

func TestWithExtraChaining(t *testing.T) {
	tc := Text("Hello ").WithExtra(Colored("World", "gold"))
	if len(tc.Extra) != 1 {
		t.Fatalf("expected 1 extra, got %d", len(tc.Extra))
	}
	if tc.Extra[0].Color != "gold" {
		t.Fatalf("expected gold, got %q", tc.Extra[0].Color)
	}
}

func TestFromLegacySimple(t *testing.T) {
	tc := FromLegacy("§aGreen §cRed")
	plain := Plain(tc)
	if plain != "Green Red" {
		t.Fatalf("expected 'Green Red', got %q", plain)
	}
}

func TestFromLegacyColors(t *testing.T) {
	tc := FromLegacy("§aHello")
	if tc.Color != "green" || tc.Text != "Hello" {
		t.Fatalf("expected green Hello, got %+v", tc)
	}
}

func TestFromLegacyBold(t *testing.T) {
	tc := FromLegacy("§l§aBold Green")
	if tc.Extra == nil || len(tc.Extra) < 1 {
		allPlain := Plain(tc)
		if allPlain != "Bold Green" {
			t.Fatalf("plain text mismatch: %q", allPlain)
		}
	}
}

func TestFromLegacyReset(t *testing.T) {
	tc := FromLegacy("§aGreen§rNormal")
	plain := Plain(tc)
	if plain != "GreenNormal" {
		t.Fatalf("expected 'GreenNormal', got %q", plain)
	}
}

func TestToLegacy(t *testing.T) {
	tc := Colored("Hello", "green")
	legacy := ToLegacy(tc)
	if legacy != "§aHello" {
		t.Fatalf("expected '§aHello', got %q", legacy)
	}
}

func TestToNBTEncodes(t *testing.T) {
	tc := Text("Hello")
	nbtComp := tc.ToNBT()
	if nbtComp == nil {
		t.Fatalf("ToNBT returned nil")
	}
	s, ok := nbtComp.GetString("text")
	if !ok || s != "Hello" {
		t.Fatalf("expected text=Hello, got %q (ok=%v)", s, ok)
	}
}

func TestToNBTWithColor(t *testing.T) {
	tc := Colored("Red", "red")
	nbtComp := tc.ToNBT()
	s, ok := nbtComp.GetString("color")
	if !ok || s != "red" {
		t.Fatalf("expected color=red, got %q (ok=%v)", s, ok)
	}
}

func TestEncodeNBT(t *testing.T) {
	tc := Text("test")
	data := tc.EncodeNBT()
	if len(data) == 0 {
		t.Fatalf("EncodeNBT returned empty")
	}
}

func TestTranslatable(t *testing.T) {
	tc := Translatable("chat.type.text", Text("Player"), Text("Hello"))
	if tc.Translate != "chat.type.text" {
		t.Fatalf("expected translate key, got %q", tc.Translate)
	}
	if len(tc.With) != 2 {
		t.Fatalf("expected 2 with args, got %d", len(tc.With))
	}
}

func TestPlainExtraction(t *testing.T) {
	tc := Text("Hello ").WithExtra(Text("World"))
	plain := Plain(tc)
	if plain != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", plain)
	}
}

func TestClickEvent(t *testing.T) {
	tc := Text("Click me").WithClick(ClickRunCommand, "/help")
	if tc.ClickEvent == nil {
		t.Fatalf("click event is nil")
	}
	if tc.ClickEvent.Action != ClickRunCommand {
		t.Fatalf("expected run_command, got %q", tc.ClickEvent.Action)
	}
}

func TestFromLegacyNoFormatting(t *testing.T) {
	tc := FromLegacy("plain text")
	if Plain(tc) != "plain text" {
		t.Fatalf("expected 'plain text', got %q", Plain(tc))
	}
}
