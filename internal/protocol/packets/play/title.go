package play

import (
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// SetTitleText sets the title text shown to the player.
type SetTitleText struct {
	Text string
}

func NewSetTitleText() protocol.Packet { return &SetTitleText{} }
func (p *SetTitleText) ID() int32      { return int32(packetid.ClientboundSetTitleText) }
func (p *SetTitleText) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetTitleText) Encode(buf *protocol.Buffer) error {
	return writeNBTTextComponent(buf, p.Text)
}

// SetTitleSubtitle sets the subtitle text shown to the player.
type SetTitleSubtitle struct {
	Text string
}

func NewSetTitleSubtitle() protocol.Packet { return &SetTitleSubtitle{} }
func (p *SetTitleSubtitle) ID() int32      { return int32(packetid.ClientboundSetTitleSubtitle) }
func (p *SetTitleSubtitle) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetTitleSubtitle) Encode(buf *protocol.Buffer) error {
	return writeNBTTextComponent(buf, p.Text)
}

// SetTitleTime sets the fade-in, stay, and fade-out times for title display.
type SetTitleTime struct {
	FadeIn  int32
	Stay    int32
	FadeOut int32
}

func NewSetTitleTime() protocol.Packet { return &SetTitleTime{} }
func (p *SetTitleTime) ID() int32      { return int32(packetid.ClientboundSetTitleTime) }
func (p *SetTitleTime) Decode(_ *protocol.Buffer) error { return nil }
func (p *SetTitleTime) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.FadeIn)
	buf.WriteInt32(p.Stay)
	buf.WriteInt32(p.FadeOut)
	return nil
}

// ActionBar sets the action bar text.
type ActionBar struct {
	Text string
}

func NewActionBar() protocol.Packet { return &ActionBar{} }
func (p *ActionBar) ID() int32      { return int32(packetid.ClientboundActionBar) }
func (p *ActionBar) Decode(_ *protocol.Buffer) error { return nil }
func (p *ActionBar) Encode(buf *protocol.Buffer) error {
	return writeNBTTextComponent(buf, p.Text)
}

// ClearTitles clears the current title.
type ClearTitles struct {
	Reset bool
}

func NewClearTitles() protocol.Packet { return &ClearTitles{} }
func (p *ClearTitles) ID() int32      { return int32(packetid.ClientboundClearTitles) }
func (p *ClearTitles) Decode(_ *protocol.Buffer) error { return nil }
func (p *ClearTitles) Encode(buf *protocol.Buffer) error {
	if p.Reset {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return nil
}

func writeNBTTextComponent(buf *protocol.Buffer, text string) error {
	comp := nbt.NewCompound()
	comp.PutString("text", text)
	enc := nbt.NewEncoder(128)
	if err := enc.WriteRootCompound(comp); err != nil {
		return err
	}
	buf.WriteBytes(enc.Bytes())
	return nil
}
