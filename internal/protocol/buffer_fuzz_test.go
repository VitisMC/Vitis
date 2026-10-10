package protocol

import "testing"

func FuzzBufferBoundedReads(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x80, 0x80, 0x80, 0x80, 0x80})
	f.Add([]byte{3, 'f', 'o', 'o'})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		buffer := WrapBuffer(data)
		_, _ = buffer.ReadVarInt()
		buffer = WrapBuffer(data)
		_, _ = buffer.ReadVarLong()
		buffer = WrapBuffer(data)
		_, _ = buffer.ReadStringLimit(1024)
	})
}
