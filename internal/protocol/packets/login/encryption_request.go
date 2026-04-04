package login

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// EncryptionRequest is sent by the server to initiate encryption handshake.
type EncryptionRequest struct {
	ServerID           string
	PublicKey          []byte
	VerifyToken        []byte
	ShouldAuthenticate bool
}

// NewEncryptionRequest constructs an empty EncryptionRequest packet.
func NewEncryptionRequest() protocol.Packet {
	return &EncryptionRequest{}
}

// ID returns the protocol packet id.
func (p *EncryptionRequest) ID() int32 {
	return int32(packetid.ClientboundLoginEncryptionBegin)
}

// Decode reads EncryptionRequest fields from buffer.
func (p *EncryptionRequest) Decode(buf *protocol.Buffer) error {
	serverID, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode encryption_request server_id: %w", err)
	}
	p.ServerID = serverID

	pubKeyLen, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode encryption_request public_key_length: %w", err)
	}
	if pubKeyLen < 0 {
		return fmt.Errorf("decode encryption_request: negative public_key_length")
	}
	pubKey, err := buf.ReadBytes(int(pubKeyLen))
	if err != nil {
		return fmt.Errorf("decode encryption_request public_key: %w", err)
	}
	p.PublicKey = make([]byte, len(pubKey))
	copy(p.PublicKey, pubKey)

	tokenLen, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode encryption_request verify_token_length: %w", err)
	}
	if tokenLen < 0 {
		return fmt.Errorf("decode encryption_request: negative verify_token_length")
	}
	token, err := buf.ReadBytes(int(tokenLen))
	if err != nil {
		return fmt.Errorf("decode encryption_request verify_token: %w", err)
	}
	p.VerifyToken = make([]byte, len(token))
	copy(p.VerifyToken, token)

	shouldAuth, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode encryption_request should_authenticate: %w", err)
	}
	p.ShouldAuthenticate = shouldAuth

	return nil
}

// Encode writes EncryptionRequest fields to buffer.
func (p *EncryptionRequest) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.ServerID); err != nil {
		return fmt.Errorf("encode encryption_request server_id: %w", err)
	}
	buf.WriteVarInt(int32(len(p.PublicKey)))
	buf.WriteBytes(p.PublicKey)
	buf.WriteVarInt(int32(len(p.VerifyToken)))
	buf.WriteBytes(p.VerifyToken)
	buf.WriteBool(p.ShouldAuthenticate)
	return nil
}
