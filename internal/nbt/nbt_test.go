package nbt

import (
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	original := NewCompound().
		PutByte("byte_val", 42).
		PutShort("short_val", 1234).
		PutInt("int_val", 100000).
		PutLong("long_val", 9876543210).
		PutFloat("float_val", 3.14).
		PutDouble("double_val", 2.718281828).
		PutString("string_val", "hello nbt").
		PutBool("bool_true", true).
		PutBool("bool_false", false).
		PutByteArray("bytes", []byte{1, 2, 3, 4}).
		PutIntArray("ints", []int32{10, 20, 30}).
		PutLongArray("longs", []int64{100, 200, 300})

	enc := NewEncoder(256)
	if err := enc.WriteRootCompound(original); err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := NewDecoder(enc.Bytes())
	result, err := dec.ReadRootCompound()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	entries := result.Entries()
	if len(entries) != len(original.Entries()) {
		t.Fatalf("entry count mismatch: got %d, want %d", len(entries), len(original.Entries()))
	}

	assertEntry(t, entries[0], "byte_val", TagByte, int8(42))
	assertEntry(t, entries[1], "short_val", TagShort, int16(1234))
	assertEntry(t, entries[2], "int_val", TagInt, int32(100000))
	assertEntry(t, entries[3], "long_val", TagLong, int64(9876543210))
	assertEntry(t, entries[9], "bytes", TagByteArray, nil)
	assertEntry(t, entries[10], "ints", TagIntArray, nil)
	assertEntry(t, entries[11], "longs", TagLongArray, nil)
}

func TestNestedCompound(t *testing.T) {
	inner := NewCompound().PutString("key", "value")
	outer := NewCompound().PutCompound("nested", inner)

	enc := NewEncoder(128)
	if err := enc.WriteRootCompound(outer); err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := NewDecoder(enc.Bytes())
	result, err := dec.ReadRootCompound()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	entries := result.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].name != "nested" || entries[0].tagID != TagCompound {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	nested := entries[0].value.(*Compound)
	if len(nested.Entries()) != 1 {
		t.Fatalf("nested entry count: %d", len(nested.Entries()))
	}
	if nested.Entries()[0].value.(string) != "value" {
		t.Fatalf("nested value: %v", nested.Entries()[0].value)
	}
}

func TestListRoundtrip(t *testing.T) {
	list := NewList(TagInt).Add(int32(1)).Add(int32(2)).Add(int32(3))
	c := NewCompound().PutList("numbers", list)

	enc := NewEncoder(128)
	if err := enc.WriteRootCompound(c); err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := NewDecoder(enc.Bytes())
	result, err := dec.ReadRootCompound()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	entries := result.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	decoded := entries[0].value.(*List)
	if decoded.Len() != 3 {
		t.Fatalf("list length: %d", decoded.Len())
	}
	for i, want := range []int32{1, 2, 3} {
		got := decoded.Elements()[i].(int32)
		if got != want {
			t.Errorf("list[%d] = %d, want %d", i, got, want)
		}
	}
}

func TestEmptyCompound(t *testing.T) {
	c := NewCompound()
	enc := NewEncoder(16)
	if err := enc.WriteRootCompound(c); err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := NewDecoder(enc.Bytes())
	result, err := dec.ReadRootCompound()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Entries()) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result.Entries()))
	}
}

func TestNamedRootCompound(t *testing.T) {
	c := NewCompound().PutInt("x", 10)
	enc := NewEncoder(64)
	if err := enc.WriteNamedRootCompound("root", c); err != nil {
		t.Fatalf("encode: %v", err)
	}

	dec := NewDecoder(enc.Bytes())
	name, result, err := dec.ReadNamedRootCompound()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if name != "root" {
		t.Fatalf("root name: %q", name)
	}
	if len(result.Entries()) != 1 {
		t.Fatalf("entry count: %d", len(result.Entries()))
	}
}

func assertEntry(t *testing.T, e entry, name string, tagID byte, value interface{}) {
	t.Helper()
	if e.name != name {
		t.Errorf("entry name: got %q, want %q", e.name, name)
	}
	if e.tagID != tagID {
		t.Errorf("entry %q tagID: got %d, want %d", name, e.tagID, tagID)
	}
	if value != nil && e.value != value {
		t.Errorf("entry %q value: got %v, want %v", name, e.value, value)
	}
}
