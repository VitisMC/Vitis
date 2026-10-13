package play

import (
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestServerboundSettingsRejectsInvalidFields(t *testing.T) {
	settings := &ServerboundSettings{
		Locale:       "en_us",
		ViewDistance: 1,
		ChatMode:     0,
		MainHand:     0,
		AllowListing: true,
	}
	buffer := protocol.NewBuffer(64)
	if err := settings.Encode(buffer); err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded := NewServerboundSettings().(*ServerboundSettings)
	if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err == nil || !strings.Contains(err.Error(), "view distance") {
		t.Fatalf("expected view distance error, got %v", err)
	}
}

func TestServerboundSettingsDecodesValidFields(t *testing.T) {
	settings := &ServerboundSettings{
		Locale:       "en_us",
		ViewDistance: 12,
		ChatMode:     1,
		ChatColors:   true,
		SkinParts:    0x7F,
		MainHand:     1,
		AllowListing: true,
	}
	buffer := protocol.NewBuffer(64)
	if err := settings.Encode(buffer); err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded := NewServerboundSettings().(*ServerboundSettings)
	if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Locale != settings.Locale || decoded.ViewDistance != settings.ViewDistance || decoded.MainHand != settings.MainHand {
		t.Fatalf("unexpected settings: %#v", decoded)
	}
}

func TestServerboundChatMessageRejectsOversizedText(t *testing.T) {
	buf := protocol.NewBuffer(maxChatMessageLength + 16)
	if err := buf.WriteString(strings.Repeat("a", maxChatMessageLength+1)); err != nil {
		t.Fatalf("write chat message: %v", err)
	}
	if err := (&ServerboundChatMessage{}).Decode(protocol.WrapBuffer(buf.Bytes())); err == nil {
		t.Fatal("oversized chat message was accepted")
	}
}
