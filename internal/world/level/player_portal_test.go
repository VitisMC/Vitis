package level

import (
	"github.com/vitismc/vitis/internal/nbt"
	"testing"
)

// TestPortalCooldownRejectsMalformedNBT requires a nonnegative canonical integer.
func TestPortalCooldownRejectsMalformedNBT(t *testing.T) {
	for _, root := range []*nbt.Compound{nbt.NewCompound().PutInt("PortalCooldown", -1), nbt.NewCompound().PutShort("PortalCooldown", 2)} {
		player := &PlayerData{}
		player.parseCompound(root)
		if player.parseError == nil {
			t.Fatal("invalid cooldown was accepted")
		}
	}
	data := DefaultPlayerData("uuid", 8, 64, 8, 0)
	data.PortalCooldown = -1
	if err := data.Save(t.TempDir()); err == nil {
		t.Fatal("invalid cooldown was saved")
	}
}

// TestPlayerDimensionRejectsMalformedNBT prevents silently restoring a player in another world.
func TestPlayerDimensionRejectsMalformedNBT(t *testing.T) {
	for _, root := range []*nbt.Compound{nbt.NewCompound().PutInt("Dimension", 1), nbt.NewCompound().PutString("Dimension", "")} {
		player := &PlayerData{}
		player.parseCompound(root)
		if player.parseError == nil {
			t.Fatal("malformed dimension became the default world")
		}
	}
}
