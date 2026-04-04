package configuration

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// ClientInformation is sent by the client during configuration to inform the server of client settings.
type ClientInformation struct {
	Locale              string
	ViewDistance        int8
	ChatMode            int32
	ChatColors          bool
	DisplayedSkinParts  byte
	MainHand            int32
	EnableTextFiltering bool
	AllowServerListings bool
	ParticleStatus      int32
}

// NewClientInformation constructs an empty ClientInformation packet.
func NewClientInformation() protocol.Packet {
	return &ClientInformation{}
}

// ID returns the protocol packet id.
func (p *ClientInformation) ID() int32 {
	return int32(packetid.ServerboundConfigSettings)
}

// Decode reads ClientInformation fields from buffer.
func (p *ClientInformation) Decode(buf *protocol.Buffer) error {
	locale, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode client_information locale: %w", err)
	}
	p.Locale = locale

	viewDist, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode client_information view_distance: %w", err)
	}
	p.ViewDistance = int8(viewDist)

	chatMode, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode client_information chat_mode: %w", err)
	}
	p.ChatMode = chatMode

	chatColors, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode client_information chat_colors: %w", err)
	}
	p.ChatColors = chatColors

	skinParts, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode client_information skin_parts: %w", err)
	}
	p.DisplayedSkinParts = skinParts

	mainHand, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode client_information main_hand: %w", err)
	}
	p.MainHand = mainHand

	textFiltering, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode client_information text_filtering: %w", err)
	}
	p.EnableTextFiltering = textFiltering

	allowListings, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode client_information allow_listings: %w", err)
	}
	p.AllowServerListings = allowListings

	particleStatus, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode client_information particle_status: %w", err)
	}
	p.ParticleStatus = particleStatus

	return nil
}

// Encode writes ClientInformation fields to buffer.
func (p *ClientInformation) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.Locale); err != nil {
		return fmt.Errorf("encode client_information locale: %w", err)
	}
	if err := buf.WriteByte(byte(p.ViewDistance)); err != nil {
		return fmt.Errorf("encode client_information view_distance: %w", err)
	}
	buf.WriteVarInt(p.ChatMode)
	buf.WriteBool(p.ChatColors)
	if err := buf.WriteByte(p.DisplayedSkinParts); err != nil {
		return fmt.Errorf("encode client_information skin_parts: %w", err)
	}
	buf.WriteVarInt(p.MainHand)
	buf.WriteBool(p.EnableTextFiltering)
	buf.WriteBool(p.AllowServerListings)
	buf.WriteVarInt(p.ParticleStatus)
	return nil
}
