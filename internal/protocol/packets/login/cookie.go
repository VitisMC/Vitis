package login

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// CookieRequest is sent by the server to request a cookie from the client during login.
type CookieRequest struct {
	Key string
}

func NewCookieRequest() protocol.Packet { return &CookieRequest{} }

func (p *CookieRequest) ID() int32 {
	return int32(packetid.ClientboundLoginCookieRequest)
}

func (p *CookieRequest) Decode(buf *protocol.Buffer) error {
	var err error
	p.Key, err = buf.ReadString()
	return err
}

func (p *CookieRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Key)
	return nil
}

// CookieResponse is sent by the client in response to a cookie request during login.
type CookieResponse struct {
	Key     string
	HasData bool
	Data    []byte
}

func NewCookieResponse() protocol.Packet { return &CookieResponse{} }

func (p *CookieResponse) ID() int32 {
	return int32(packetid.ServerboundLoginCookieResponse)
}

func (p *CookieResponse) Decode(buf *protocol.Buffer) error {
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

func (p *CookieResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteString(p.Key)
	buf.WriteBool(p.HasData)
	if p.HasData {
		buf.WriteVarInt(int32(len(p.Data)))
		buf.WriteBytes(p.Data)
	}
	return nil
}
