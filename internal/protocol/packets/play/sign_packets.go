package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// OpenSignEditor tells the client to open the sign editor GUI.
type OpenSignEditor struct {
	X, Y, Z int32
	IsFront bool
}

func NewOpenSignEditor() protocol.Packet { return &OpenSignEditor{} }

func (p *OpenSignEditor) ID() int32 {
	return int32(packetid.ClientboundOpenSignEntity)
}

func (p *OpenSignEditor) Decode(_ *protocol.Buffer) error { return nil }

func (p *OpenSignEditor) Encode(buf *protocol.Buffer) error {
	position := encodeBlockPosition(p.X, p.Y, p.Z)
	buf.WriteInt64(int64(position))
	if p.IsFront {
		buf.WriteBool(true)
	} else {
		buf.WriteBool(false)
	}
	return nil
}

// ServerboundUpdateSign is sent by the client after editing a sign.
type ServerboundUpdateSign struct {
	X, Y, Z int32
	IsFront bool
	Lines   [4]string
}

func NewServerboundUpdateSign() protocol.Packet { return &ServerboundUpdateSign{} }

func (p *ServerboundUpdateSign) ID() int32 {
	return int32(packetid.ServerboundUpdateSign)
}

func (p *ServerboundUpdateSign) Decode(buf *protocol.Buffer) error {
	posInt64, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode update_sign position: %w", err)
	}
	p.X, p.Y, p.Z = decodeBlockPosition(uint64(posInt64))

	isFront, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode update_sign is_front: %w", err)
	}
	p.IsFront = isFront

	for i := 0; i < 4; i++ {
		line, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode update_sign line %d: %w", i, err)
		}
		p.Lines[i] = line
	}
	return nil
}

func (p *ServerboundUpdateSign) Encode(buf *protocol.Buffer) error {
	position := encodeBlockPosition(p.X, p.Y, p.Z)
	buf.WriteInt64(int64(position))
	buf.WriteBool(p.IsFront)
	for _, line := range p.Lines {
		buf.WriteString(line)
	}
	return nil
}
