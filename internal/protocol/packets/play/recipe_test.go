package play

import (
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

// TestRecipeBookSettingsRoundTrip verifies all four book type settings.
func TestRecipeBookSettingsRoundTrip(t *testing.T) {
	want := RecipeBookSettings{
		CraftingOpen: true, SmeltingFilter: true,
		BlastFurnaceOpen: true, BlastFurnaceFilter: true, SmokerOpen: true,
	}
	buffer := protocol.NewBuffer(16)
	if err := want.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	var got RecipeBookSettings
	if err := got.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

// TestServerboundRecipeBookRoundTrip verifies typed settings packet encoding.
func TestServerboundRecipeBookRoundTrip(t *testing.T) {
	want := ServerboundRecipeBook{BookType: 2, Open: true, Filtering: true}
	buffer := protocol.NewBuffer(16)
	if err := want.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	var got ServerboundRecipeBook
	if err := got.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("packet = %+v, want %+v", got, want)
	}
}

// TestServerboundRecipeBookRejectsInvalidType verifies enum bounds.
func TestServerboundRecipeBookRejectsInvalidType(t *testing.T) {
	buffer := protocol.NewBuffer(16)
	buffer.WriteVarInt(4)
	buffer.WriteBool(false)
	buffer.WriteBool(false)
	var packet ServerboundRecipeBook
	if err := packet.Decode(protocol.WrapBuffer(buffer.Bytes())); err == nil {
		t.Fatal("accepted invalid recipe book type")
	}
}
