package login

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EncryptionResponse is sent by the client with encrypted shared secret and verify token.
type EncryptionResponse struct {
	SharedSecret []byte
	VerifyToken  []byte
}

// NewEncryptionResponse constructs an empty EncryptionResponse packet.
func NewEncryptionResponse() protocol.Packet {
	return &EncryptionResponse{}
}

// ID returns the protocol packet id.
func (p *EncryptionResponse) ID() int32 {
	return int32(packetid.ServerboundLoginEncryptionBegin)
}

// Decode reads EncryptionResponse fields from buffer.
func (p *EncryptionResponse) Decode(buf *protocol.Buffer) error {
	secretLen, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode encryption_response shared_secret_length: %w", err)
	}
	if secretLen < 0 {
		return fmt.Errorf("decode encryption_response: negative shared_secret_length")
	}
	secret, err := buf.ReadBytes(int(secretLen))
	if err != nil {
		return fmt.Errorf("decode encryption_response shared_secret: %w", err)
	}
	p.SharedSecret = make([]byte, len(secret))
	copy(p.SharedSecret, secret)

	tokenLen, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode encryption_response verify_token_length: %w", err)
	}
	if tokenLen < 0 {
		return fmt.Errorf("decode encryption_response: negative verify_token_length")
	}
	token, err := buf.ReadBytes(int(tokenLen))
	if err != nil {
		return fmt.Errorf("decode encryption_response verify_token: %w", err)
	}
	p.VerifyToken = make([]byte, len(token))
	copy(p.VerifyToken, token)

	return nil
}

// Encode writes EncryptionResponse fields to buffer.
func (p *EncryptionResponse) Encode(buf *protocol.Buffer) error {
	buf.WriteVarInt(int32(len(p.SharedSecret)))
	buf.WriteBytes(p.SharedSecret)
	buf.WriteVarInt(int32(len(p.VerifyToken)))
	buf.WriteBytes(p.VerifyToken)
	return nil
}
