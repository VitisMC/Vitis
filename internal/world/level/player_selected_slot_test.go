package level

import (
	"testing"

	"github.com/vitismc/vitis/internal/nbt"
)

// TestSelectedItemSlotRoundTrip preserves the active hotbar slot across restarts.
func TestSelectedItemSlotRoundTrip(t *testing.T) {
	const uuid = "00000000-0000-0000-0000-000000000001"
	data := DefaultPlayerData(uuid, 8, 64, 8, 0)
	data.SelectedItemSlot = 7
	dir := t.TempDir()
	if err := data.Save(dir); err != nil {
		t.Fatal(err)
	}
	restored, err := LoadPlayerData(dir, uuid)
	if err != nil || restored.SelectedItemSlot != 7 {
		t.Fatalf("selected slot round trip failed: player=%+v error=%v", restored, err)
	}
}

// TestSelectedItemSlotRejectsMalformedState validates the canonical integer and hotbar bounds.
func TestSelectedItemSlotRejectsMalformedState(t *testing.T) {
	for _, root := range []*nbt.Compound{
		nbt.NewCompound().PutInt("SelectedItemSlot", -1),
		nbt.NewCompound().PutInt("SelectedItemSlot", 9),
		nbt.NewCompound().PutByte("SelectedItemSlot", 7),
	} {
		player := &PlayerData{}
		player.parseCompound(root)
		if player.parseError == nil {
			t.Fatal("malformed selected slot was accepted")
		}
	}
	for _, slot := range []int32{-1, 9} {
		data := DefaultPlayerData("uuid", 8, 64, 8, 0)
		data.SelectedItemSlot = slot
		if err := data.Save(t.TempDir()); err == nil {
			t.Fatal("out-of-range selected slot was saved")
		}
	}
	legacy := &PlayerData{}
	legacy.parseCompound(nbt.NewCompound())
	if legacy.parseError != nil || legacy.SelectedItemSlot != 0 {
		t.Fatal("legacy absent selection did not preserve the first hotbar slot")
	}
}
