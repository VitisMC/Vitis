package play

import (
	"bytes"
	"github.com/vitismc/vitis/internal/protocol"
	"testing"
)

// TestRecipeBookReplaceAndContainerID checks exact protocol 769 framing.
func TestRecipeBookReplaceAndContainerID(t *testing.T) {
	buffer := protocol.NewBuffer(16)
	if err := (&RecipeBookAdd{Replace: true}).Encode(buffer); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer.Bytes(), []byte{0, 1}) {
		t.Fatalf("recipe replacement bytes=%x", buffer.Bytes())
	}
	var add RecipeBookAdd
	if err := add.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil || !add.Replace {
		t.Fatal("replace flag not decoded")
	}
	buffer = protocol.NewBuffer(16)
	request := CraftRecipeRequest{WindowID: 300, RecipeID: 1, MakeAll: true}
	if err := request.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer.Bytes(), []byte{0xac, 0x02, 1, 1}) {
		t.Fatalf("container ID bytes=%x", buffer.Bytes())
	}
	var decoded CraftRecipeRequest
	if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil || decoded != request {
		t.Fatalf("request=%+v error=%v", decoded, err)
	}
}

// TestRecipeDisplayRoundTrip verifies populated shaped entries and unknown discriminants.
func TestRecipeDisplayRoundTrip(t *testing.T) {
	packet := RecipeBookAdd{Replace: true, Recipes: []RecipeBookEntry{{DisplayID: 5, Group: -1, Category: 3, Flags: 3, Requirements: [][]int32{{1, 2}}, Display: RecipeDisplay{Type: 1, Width: 1, Height: 1, Ingredients: []SlotDisplay{{Type: 7, Children: []SlotDisplay{{Type: 2, ItemID: 1}, {Type: 2, ItemID: 2}}}}, Result: SlotDisplay{Type: 2, ItemID: 3}, Station: SlotDisplay{Type: 2, ItemID: 4}}}}}
	buffer := protocol.NewBuffer(128)
	if err := packet.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	var decoded RecipeBookAdd
	if err := decoded.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatal(err)
	}
	again := protocol.NewBuffer(128)
	if err := decoded.Encode(again); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffer.Bytes(), again.Bytes()) {
		t.Fatal("display roundtrip changed bytes")
	}
	if _, err := decodeRecipeDisplay(protocol.WrapBuffer([]byte{99})); err == nil {
		t.Fatal("accepted unknown recipe display")
	}
	if _, err := decodeSlotDisplay(protocol.WrapBuffer([]byte{99}), 0); err == nil {
		t.Fatal("accepted unknown slot display")
	}
}
