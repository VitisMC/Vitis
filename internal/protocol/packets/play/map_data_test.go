package play

import (
	"testing"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

// TestMapDataRoundTrip verifies typed decoration and color patch encoding.
func TestMapDataRoundTrip(t *testing.T) {
	name := nbt.NewCompound()
	name.PutString("text", "Home")
	encoder := nbt.NewEncoder(32)
	if err := encoder.WriteRootCompound(name); err != nil {
		t.Fatal(err)
	}
	want := MapData{MapID: 7, Scale: 2, Locked: true, HasDecorations: true,
		Decorations: []MapDecoration{{TypeID: 1, X: -4, Y: 12, Rotation: 7, NameNBT: encoder.Bytes()}},
		Patch:       &MapPatch{StartX: 3, StartY: 4, Width: 2, Height: 2, Colors: []byte{1, 2, 3, 4}}}
	buffer := protocol.NewBuffer(128)
	if err := want.Encode(buffer); err != nil {
		t.Fatal(err)
	}
	var got MapData
	if err := got.Decode(protocol.WrapBuffer(buffer.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got.MapID != want.MapID || got.Scale != want.Scale || !got.Locked || len(got.Decorations) != 1 || got.Decorations[0].X != -4 || got.Patch == nil || len(got.Patch.Colors) != 4 {
		t.Fatalf("decoded map = %+v", got)
	}
}

// TestMapDataRejectsInvalidPatch verifies payload and bounds validation.
func TestMapDataRejectsInvalidPatch(t *testing.T) {
	packet := MapData{MapID: 1, Patch: &MapPatch{StartX: 127, Width: 2, Height: 1, Colors: []byte{1, 2}}}
	if err := packet.Encode(protocol.NewBuffer(32)); err == nil {
		t.Fatal("accepted out-of-bounds map patch")
	}
}
