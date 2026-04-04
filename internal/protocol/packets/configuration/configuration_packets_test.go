package configuration

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestClientInformationRoundtrip(t *testing.T) {
	original := &ClientInformation{
		Locale:              "en_US",
		ViewDistance:         12,
		ChatMode:            0,
		ChatColors:          true,
		DisplayedSkinParts:  0x7F,
		MainHand:            1,
		EnableTextFiltering: false,
		AllowServerListings: true,
		ParticleStatus:      0,
	}

	buf := protocol.NewBuffer(64)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &ClientInformation{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Locale != original.Locale {
		t.Errorf("Locale: got %q, want %q", decoded.Locale, original.Locale)
	}
	if decoded.ViewDistance != original.ViewDistance {
		t.Errorf("ViewDistance: got %d, want %d", decoded.ViewDistance, original.ViewDistance)
	}
	if decoded.ChatColors != original.ChatColors {
		t.Errorf("ChatColors: got %v, want %v", decoded.ChatColors, original.ChatColors)
	}
	if decoded.DisplayedSkinParts != original.DisplayedSkinParts {
		t.Errorf("SkinParts: got %d, want %d", decoded.DisplayedSkinParts, original.DisplayedSkinParts)
	}
}

func TestKnownPacksRoundtrip(t *testing.T) {
	original := &ClientboundKnownPacks{
		Packs: []KnownPack{
			{Namespace: "minecraft", ID: "core", Version: "1.21.4"},
		},
	}

	buf := protocol.NewBuffer(64)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &ClientboundKnownPacks{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(decoded.Packs) != 1 {
		t.Fatalf("Packs length: got %d, want 1", len(decoded.Packs))
	}
	if decoded.Packs[0].Namespace != "minecraft" {
		t.Errorf("Namespace: got %q, want %q", decoded.Packs[0].Namespace, "minecraft")
	}
	if decoded.Packs[0].ID != "core" {
		t.Errorf("ID: got %q, want %q", decoded.Packs[0].ID, "core")
	}
	if decoded.Packs[0].Version != "1.21.4" {
		t.Errorf("Version: got %q, want %q", decoded.Packs[0].Version, "1.21.4")
	}
}

func TestServerboundKnownPacksRoundtrip(t *testing.T) {
	original := &ServerboundKnownPacks{
		Packs: []KnownPack{
			{Namespace: "minecraft", ID: "core", Version: "1.21.4"},
			{Namespace: "custom", ID: "data", Version: "1.0"},
		},
	}

	buf := protocol.NewBuffer(128)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &ServerboundKnownPacks{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(decoded.Packs) != 2 {
		t.Fatalf("Packs length: got %d, want 2", len(decoded.Packs))
	}
}

func TestFinishConfigurationRoundtrip(t *testing.T) {
	original := &FinishConfiguration{}
	buf := protocol.NewBuffer(4)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("FinishConfiguration should encode to 0 bytes, got %d", buf.Len())
	}
}

func TestAcknowledgeFinishConfigurationTransition(t *testing.T) {
	pkt := &AcknowledgeFinishConfiguration{}
	state, ok := pkt.InboundStateTransition()
	if !ok {
		t.Fatal("expected state transition")
	}
	if state != protocol.StatePlay {
		t.Errorf("expected StatePlay, got %s", state.String())
	}
}

func TestPluginMessageRoundtrip(t *testing.T) {
	original := &ServerboundPluginMessage{
		Channel: "minecraft:brand",
		Data:    []byte("vanilla"),
	}

	buf := protocol.NewBuffer(64)
	if err := original.Encode(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := &ServerboundPluginMessage{}
	readBuf := protocol.WrapBuffer(buf.Bytes())
	if err := decoded.Decode(readBuf); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Channel != original.Channel {
		t.Errorf("Channel: got %q, want %q", decoded.Channel, original.Channel)
	}
	if string(decoded.Data) != string(original.Data) {
		t.Errorf("Data: got %q, want %q", decoded.Data, original.Data)
	}
}
