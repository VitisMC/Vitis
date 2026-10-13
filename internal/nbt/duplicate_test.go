package nbt

import "testing"

// TestDecoderRejectsDuplicateKeys prevents ambiguous persistent and network values.
func TestDecoderRejectsDuplicateKeys(t *testing.T) {
	// The unnamed compound contains two byte tags with the same key.
	data := []byte{TagCompound, TagByte, 0, 1, 'a', 1, TagByte, 0, 1, 'a', 2, TagEnd}
	if _, err := NewDecoder(data).ReadRootCompound(); err == nil {
		t.Fatal("duplicate key accepted")
	}
}
