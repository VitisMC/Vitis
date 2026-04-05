package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetDifficulty is sent by the client to request a difficulty change.
type SetDifficulty struct {
	Difficulty byte
}

func NewSetDifficulty() protocol.Packet { return &SetDifficulty{} }

func (p *SetDifficulty) ID() int32 {
	return int32(packetid.ServerboundSetDifficulty)
}

func (p *SetDifficulty) Decode(buf *protocol.Buffer) error {
	var err error
	p.Difficulty, err = buf.ReadByte()
	return err
}

func (p *SetDifficulty) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.Difficulty)
	return nil
}

// LockDifficulty is sent by the client to lock the server difficulty.
type LockDifficulty struct {
	Locked bool
}

func NewLockDifficulty() protocol.Packet { return &LockDifficulty{} }

func (p *LockDifficulty) ID() int32 {
	return int32(packetid.ServerboundLockDifficulty)
}

func (p *LockDifficulty) Decode(buf *protocol.Buffer) error {
	var err error
	p.Locked, err = buf.ReadBool()
	return err
}

func (p *LockDifficulty) Encode(buf *protocol.Buffer) error {
	buf.WriteBool(p.Locked)
	return nil
}
