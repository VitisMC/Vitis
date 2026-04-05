package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// Difficulty notifies the client of the current server difficulty.
type Difficulty struct {
	DifficultyLevel byte
	Locked          bool
}

func NewDifficulty() protocol.Packet { return &Difficulty{} }

func (p *Difficulty) ID() int32 {
	return int32(packetid.ClientboundDifficulty)
}

func (p *Difficulty) Decode(buf *protocol.Buffer) error {
	var err error
	if p.DifficultyLevel, err = buf.ReadByte(); err != nil {
		return err
	}
	p.Locked, err = buf.ReadBool()
	return err
}

func (p *Difficulty) Encode(buf *protocol.Buffer) error {
	buf.WriteByte(p.DifficultyLevel)
	buf.WriteBool(p.Locked)
	return nil
}
