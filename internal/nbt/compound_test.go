package nbt

import "testing"

func TestCompoundPutReplacesExistingTag(t *testing.T) {
	c := NewCompound().PutString("name", "old").PutString("name", "new")

	got, ok := c.GetString("name")
	if !ok || got != "new" {
		t.Fatalf("GetString(name) = %q, %v; want new, true", got, ok)
	}
	if len(c.Entries()) != 1 {
		t.Fatalf("entry count = %d; want 1", len(c.Entries()))
	}
}

func TestCompoundCloneIsDeepCopy(t *testing.T) {
	original := NewCompound().
		PutByteArray("bytes", []byte{1, 2}).
		PutCompound("nested", NewCompound().PutInt("value", 1)).
		PutList("list", NewList(TagCompound).Add(NewCompound().PutString("value", "old")))
	clone := original.Clone()

	cloneBytes, _, _ := clone.Get("bytes")
	cloneBytes.([]byte)[0] = 9
	cloneNested, _ := clone.GetCompound("nested")
	cloneNested.PutInt("value", 2)
	cloneList, _ := clone.GetList("list")
	cloneList.Elements()[0].(*Compound).PutString("value", "new")

	originalBytes, _, _ := original.Get("bytes")
	if originalBytes.([]byte)[0] != 1 {
		t.Fatal("byte array was shared with clone")
	}
	originalNested, _ := original.GetCompound("nested")
	if value, _ := originalNested.GetInt("value"); value != 1 {
		t.Fatal("nested compound was shared with clone")
	}
	originalList, _ := original.GetList("list")
	if value, _ := originalList.Elements()[0].(*Compound).GetString("value"); value != "old" {
		t.Fatal("list element was shared with clone")
	}
}

// TestCompoundDeletePreservesRemainingTags checks stable NBT merge behavior.
func TestCompoundDeletePreservesRemainingTags(t *testing.T) {
	compound := NewCompound().PutInt("first", 1).PutInt("remove", 2).PutInt("last", 3)
	compound.Delete("remove")
	if _, _, ok := compound.Get("remove"); ok || len(compound.Entries()) != 2 {
		t.Fatal("tag was not removed")
	}
	if value, _ := compound.GetInt("last"); value != 3 {
		t.Fatal("remaining tag changed")
	}
	if ints := compound.GetAllInts(); len(ints) != 2 || ints["first"] != 1 {
		t.Fatalf("int entries = %#v", ints)
	}
}

func TestReadRootTagAcceptsNonCompoundNetworkNBT(t *testing.T) {
	data := []byte{TagString, 0, 5, 'h', 'e', 'l', 'l', 'o'}
	tagType, value, err := NewDecoder(data).ReadRootTag()
	if err != nil {
		t.Fatalf("ReadRootTag: %v", err)
	}
	if tagType != TagString || value != "hello" {
		t.Fatalf("tag=%d value=%#v", tagType, value)
	}
	if _, _, err := NewDecoder([]byte{TagInt, 0, 0, 0}).ReadRootTag(); err == nil {
		t.Fatal("truncated root tag was accepted")
	}
}
