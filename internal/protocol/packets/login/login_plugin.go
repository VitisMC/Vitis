package login

import (
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// LoginPluginRequest is sent by the server to request a response from the client via a custom login channel.
type LoginPluginRequest struct {
	MessageID int32
	Channel   string
	Data      []byte
}

func NewLoginPluginRequest() protocol.Packet { return &LoginPluginRequest{} }

func (p *LoginPluginRequest) ID() int32 {
	return int32(packetid.ClientboundLoginLoginPluginRequest)
}

func (p *LoginPluginRequest) Decode(buf *protocol.Buffer) error {
	var err error
	if p.MessageID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Channel, err = buf.ReadString(); err != nil {
		return err
	}
	remaining := buf.Remaining()
	if remaining > 0 {
		p.Data, err = buf.ReadBytes(remaining)
	}
	return err
}

func (p *LoginPluginRequest) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.MessageID)
	buf.WriteString(p.Channel)
	if len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}

// LoginPluginResponse is sent by the client in response to a login plugin request.
type LoginPluginResponse struct {
	MessageID  int32
	Successful bool
	Data       []byte
}

func NewLoginPluginResponse() protocol.Packet { return &LoginPluginResponse{} }

func (p *LoginPluginResponse) ID() int32 {
	return int32(packetid.ServerboundLoginLoginPluginResponse)
}

func (p *LoginPluginResponse) Decode(buf *protocol.Buffer) error {
	var err error
	if p.MessageID, err = buf.ReadVarInt(); err != nil {
		return err
	}
	if p.Successful, err = buf.ReadBool(); err != nil {
		return err
	}
	if p.Successful {
		remaining := buf.Remaining()
		if remaining > 0 {
			p.Data, err = buf.ReadBytes(remaining)
		}
	}
	return err
}

func (p *LoginPluginResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(p.MessageID)
	buf.WriteBool(p.Successful)
	if p.Successful && len(p.Data) > 0 {
		buf.WriteBytes(p.Data)
	}
	return nil
}
