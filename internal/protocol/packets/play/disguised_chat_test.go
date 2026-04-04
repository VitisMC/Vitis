package play

import (
	"fmt"
	"testing"

	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
)

func TestDisguisedChatRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		sender     string
		chatType   int32
		hasTarget  bool
		targetName string
	}{
		{"simple ascii", "hello", "Steve", 0, false, ""},
		{"single char", "w", "Test", 0, false, ""},
		{"cyrillic", "пцукп", "JanekDev", 0, false, ""},
		{"with target", "whisper", "Alice", 2, true, "Bob"},
		{"empty message", "", "Server", 0, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pkt *DisguisedChatMessage
			if tt.hasTarget {
				targetComp := nbt.NewCompound()
				targetComp.PutString("text", tt.targetName)
				targetEnc := nbt.NewEncoder(64)
				_ = targetEnc.WriteRootCompound(targetComp)

				msgComp := nbt.NewCompound()
				msgComp.PutString("text", tt.message)
				msgEnc := nbt.NewEncoder(128)
				_ = msgEnc.WriteRootCompound(msgComp)

				senderComp := nbt.NewCompound()
				senderComp.PutString("text", tt.sender)
				senderEnc := nbt.NewEncoder(64)
				_ = senderEnc.WriteRootCompound(senderComp)

				pkt = &DisguisedChatMessage{
					MessageNBT: msgEnc.Bytes(),
					ChatType:   tt.chatType,
					SenderNBT:  senderEnc.Bytes(),
					HasTarget:  true,
					TargetNBT:  targetEnc.Bytes(),
				}
			} else {
				pkt = NewDisguisedChatSimple(tt.message, tt.sender)
				pkt.ChatType = tt.chatType
			}

			buf := protocol.NewBuffer(256)
			if err := pkt.Encode(buf); err != nil {
				t.Fatalf("encode: %v", err)
			}
			encoded := buf.Bytes()

			pos := 0

			msgNBT, msgEnd, err := readNBTTextComponent(encoded, pos)
			if err != nil {
				t.Fatalf("read message NBT at pos %d: %v", pos, err)
			}
			t.Logf("message NBT: %d bytes (pos %d->%d)", msgEnd-pos, pos, msgEnd)
			_ = msgNBT
			pos = msgEnd

			chatType, vtLen := decodeTestVarInt(encoded[pos:])
			if vtLen <= 0 {
				t.Fatalf("read chat type VarInt at pos %d: failed", pos)
			}
			expectedWire := tt.chatType + 1
			if chatType != expectedWire {
				t.Fatalf("chat type wire: got %d, want %d (ID-or-X: 0=inline, N>0=registry ref N-1)", chatType, expectedWire)
			}
			t.Logf("chat type: wire=%d (registry ref %d), %d bytes, pos %d->%d", chatType, chatType-1, vtLen, pos, pos+vtLen)
			pos += vtLen

			senderNBT, senderEnd, err := readNBTTextComponent(encoded, pos)
			if err != nil {
				t.Fatalf("read sender NBT at pos %d: %v", pos, err)
			}
			t.Logf("sender NBT: %d bytes (pos %d->%d)", senderEnd-pos, pos, senderEnd)
			_ = senderNBT
			pos = senderEnd

			if pos >= len(encoded) {
				t.Fatalf("no bytes left for HasTarget bool at pos %d (total %d)", pos, len(encoded))
			}
			hasTarget := encoded[pos] != 0
			t.Logf("hasTarget: %v (pos %d)", hasTarget, pos)
			pos++

			if hasTarget != tt.hasTarget {
				t.Fatalf("hasTarget: got %v, want %v", hasTarget, tt.hasTarget)
			}

			if hasTarget {
				targetNBT, targetEnd, err := readNBTTextComponent(encoded, pos)
				if err != nil {
					t.Fatalf("read target NBT at pos %d: %v", pos, err)
				}
				t.Logf("target NBT: %d bytes (pos %d->%d)", targetEnd-pos, pos, targetEnd)
				_ = targetNBT
				pos = targetEnd
			}

			if pos != len(encoded) {
				t.Fatalf("trailing bytes: consumed %d of %d", pos, len(encoded))
			}

			t.Logf("OK: total %d bytes, all consumed", len(encoded))
		})
	}
}

func readNBTTextComponent(data []byte, offset int) (*nbt.Compound, int, error) {
	if offset >= len(data) {
		return nil, 0, fmt.Errorf("no data at offset %d", offset)
	}
	tagType := data[offset]
	if tagType == nbt.TagEnd {
		return nil, offset + 1, nil
	}
	if tagType == nbt.TagString {
		pos := offset + 1
		if pos+2 > len(data) {
			return nil, 0, fmt.Errorf("string length truncated at pos %d", pos)
		}
		strLen := int(uint16(data[pos])<<8 | uint16(data[pos+1]))
		pos += 2
		if pos+strLen > len(data) {
			return nil, 0, fmt.Errorf("string data truncated at pos %d, need %d", pos, strLen)
		}
		pos += strLen
		c := nbt.NewCompound()
		c.PutString("text", string(data[offset+3:offset+3+strLen]))
		return c, pos, nil
	}
	if tagType != nbt.TagCompound {
		return nil, 0, fmt.Errorf("unexpected tag type %d at offset %d", tagType, offset)
	}
	dec := nbt.NewDecoder(data[offset:])
	comp, err := dec.ReadRootCompound()
	if err != nil {
		return nil, 0, fmt.Errorf("decode compound: %w", err)
	}
	return comp, offset + dec.Pos(), nil
}

func decodeTestVarInt(data []byte) (int32, int) {
	var value int32
	for i := 0; i < 5 && i < len(data); i++ {
		b := data[i]
		value |= int32(b&0x7F) << (i * 7)
		if b&0x80 == 0 {
			return value, i + 1
		}
	}
	return 0, 0
}
