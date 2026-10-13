package nbt

import "testing"

func FuzzDecodeRootCompound(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{TagEnd})
	f.Add([]byte{TagCompound, TagEnd})
	f.Add([]byte{TagCompound, TagList, 0, 1, 'x', TagCompound, 0, 0, 0, 1, TagEnd})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		decoder := NewDecoderWithLimits(data, DecoderLimits{
			MaxBytes:            1 << 20,
			MaxDepth:            32,
			MaxNodes:            4096,
			MaxCollectionLength: 4096,
		})
		_, _ = decoder.ReadRootCompound()
	})
}
