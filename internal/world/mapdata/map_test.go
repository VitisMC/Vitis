package mapdata

import "testing"

// TestMapCenterUsesScaleGrid verifies positive and negative map alignment.
func TestMapCenterUsesScaleGrid(t *testing.T) {
	positive, err := New(1, "minecraft:overworld", 200, 200, 1, true, false)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := New(2, "minecraft:overworld", -200, -200, 1, true, false)
	if err != nil {
		t.Fatal(err)
	}
	px, pz := positive.Center()
	nx, nz := negative.Center()
	if px != 320 || pz != 320 || nx != -192 || nz != -192 {
		t.Fatalf("centers positive=(%d,%d) negative=(%d,%d)", px, pz, nx, nz)
	}
}

// TestMapPatchTracksMinimalRectangle verifies incremental patch construction.
func TestMapPatchTracksMinimalRectangle(t *testing.T) {
	m, err := New(1, "minecraft:overworld", 0, 0, 0, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !m.SetColor(4, 5, 7) || !m.SetColor(6, 6, 9) {
		t.Fatal("color update rejected")
	}
	patch := m.TakePatch()
	if patch == nil || patch.StartX != 4 || patch.StartY != 5 || patch.Width != 3 || patch.Height != 2 || len(patch.Colors) != 6 || patch.Colors[0] != 7 || patch.Colors[5] != 9 {
		t.Fatalf("patch = %+v", patch)
	}
	if m.TakePatch() != nil {
		t.Fatal("dirty state was not cleared")
	}
	m.Lock()
	if m.SetColor(1, 1, 3) {
		t.Fatal("locked map accepted update")
	}
}
