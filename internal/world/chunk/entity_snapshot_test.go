package chunk

import "testing"

// TestEntitySnapshotRevisionPreservesPacketCache keeps entity lifetime updates out of chunk encoding.
func TestEntitySnapshotRevisionPreservesPacketCache(t *testing.T) {
	c := New(0, 0)
	before := c.EncodePacketPayload()
	revision := c.Revision()
	c.SetEntityNBT([]byte("immutable snapshot"))
	if !c.Dirty() || c.Revision() == revision || c.ClearDirtyIfRevision(revision) {
		t.Fatal("entity mutation did not advance persistence revision")
	}
	after := c.EncodePacketPayload()
	if len(before) == 0 || &before[0] != &after[0] {
		t.Fatal("entity-only mutation invalidated block packet cache")
	}
	c.SetEntityNBT([]byte("immutable snapshot"))
	if c.Revision() != revision+1 {
		t.Fatal("unchanged entity snapshot advanced revision")
	}
	frozen, err := SnapshotForSave(c)
	if err != nil {
		t.Fatal(err)
	}
	c.SetEntityNBT([]byte("next snapshot"))
	data, err := frozen.EntityNBT()
	if err != nil || string(data) != "immutable snapshot" {
		t.Fatal("frozen snapshot shared entity payload")
	}
	data[0] = 'X'
	again, _ := frozen.EntityNBT()
	if string(again) != "immutable snapshot" {
		t.Fatal("entity payload accessor leaked writable storage")
	}
}
