package play

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientboundCookieRequest is sent by the server to request a cookie from the client during play.
type ClientboundCookieRequest struct {
	Key string
}

func NewClientboundCookieRequest() protocol.Packet { return &ClientboundCookieRequest{} }

func (p *ClientboundCookieRequest) ID() int32 {
	return int32(packetid.ClientboundCookieRequest)
}

func (p *ClientboundCookieRequest) Decode(buf *protocol.Buffer) error {
	var err error
	p.Key, err = buf.ReadString()
	return err
}

func (p *ClientboundCookieRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Key)
	return nil
}

// ServerboundCookieResponse is sent by the client in response to a cookie request during play.
type ServerboundCookieResponse struct {
	Key     string
	HasData bool
	Data    []byte
}

func NewServerboundCookieResponse() protocol.Packet { return &ServerboundCookieResponse{} }

func (p *ServerboundCookieResponse) ID() int32 {
	return int32(packetid.ServerboundCookieResponse)
}

func (p *ServerboundCookieResponse) Decode(buf *protocol.Buffer) error {
	var err error
	if p.Key, err = buf.ReadString(); err != nil {
		return err
	}
	if p.HasData, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.HasData {
		length, err := buf.ReadVarInt()
		if err != nil {
			return err
		}
		p.Data, err = buf.ReadBytes(int(length))
		return err
	}
	return nil
}

func (p *ServerboundCookieResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Key)
	buf.WriteBool(p.HasData)
	if p.HasData {
		buf.WriteVarInt(int32(len(p.Data)))
		buf.WriteBytes(p.Data)
	}
	return nil
}
