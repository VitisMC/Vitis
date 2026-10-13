package chunk

import (
	"math"
	"testing"

	"github.com/vitismc/vitis/internal/nbt"
)

// TestInhabitedTimeIsFrozenAndRestored preserves local difficulty through asynchronous saves.
func TestInhabitedTimeIsFrozenAndRestored(t *testing.T) {
	c := New(-2, 3)
	packet := c.EncodePacketPayload()
	revision := c.Revision()
	c.AddInhabitedTime(120)
	if c.InhabitedTime() != 120 || !c.Dirty() || c.Revision() <= revision || &c.EncodePacketPayload()[0] != &packet[0] {
		t.Fatal("inhabited counter did not dirty persistence or invalidated cached packets")
	}
	snapshot, err := SnapshotForSave(c)
	if err != nil {
		t.Fatal(err)
	}
	c.AddInhabitedTime(80)
	encoded, err := EncodeChunkNBT(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeChunkNBT(encoded, -2, 3, 0)
	if err != nil || loaded.InhabitedTime() != 120 || c.InhabitedTime() != 200 {
		t.Fatalf("counter snapshot changed: %v", err)
	}
	loaded.AddInhabitedTime(-1)
	loaded.AddInhabitedTime(math.MaxInt64)
	loaded.AddInhabitedTime(1)
	if loaded.InhabitedTime() != math.MaxInt64 {
		t.Fatal("counter overflowed")
	}
}

// TestInhabitedTimeRejectsMalformedNBT validates persisted counters before chunk publication.
func TestInhabitedTimeRejectsMalformedNBT(t *testing.T) {
	encoded, err := EncodeChunkNBT(New(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, root, err := nbt.NewDecoder(encoded).ReadNamedRootCompound()
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nbt.Compound){
		func(raw *nbt.Compound) { raw.PutLong("InhabitedTime", -1) },
		func(raw *nbt.Compound) { raw.PutInt("InhabitedTime", 20) },
	} {
		copy := root.Clone()
		mutate(copy)
		encoder := nbt.NewEncoder(32768)
		if err := encoder.WriteNamedRootCompound("", copy); err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeChunkNBT(encoder.Bytes(), 0, 0, 0); err == nil {
			t.Fatal("malformed chunk counter was accepted")
		}
	}
}
