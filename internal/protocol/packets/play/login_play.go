package play

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// LoginPlay is sent by the server when a player enters the play state.
type LoginPlay struct {
	EntityID            int32
	IsHardcore          bool
	DimensionNames      []string
	MaxPlayers          int32
	ViewDistance        int32
	SimulationDistance  int32
	ReducedDebugInfo    bool
	EnableRespawnScreen bool
	DoLimitedCrafting   bool
	DimensionType       int32
	DimensionName       string
	HashedSeed          int64
	GameMode            byte
	PreviousGameMode    int8
	IsDebug             bool
	IsFlat              bool
	HasDeathLocation    bool
	DeathDimensionName  string
	DeathLocationX      int32
	DeathLocationY      int32
	DeathLocationZ      int32
	PortalCooldown      int32
	SeaLevel            int32
	EnforcesSecureChat  bool
}

// NewLoginPlay constructs an empty LoginPlay packet.
func NewLoginPlay() protocol.Packet {
	return &LoginPlay{}
}

// ID returns the protocol packet id.
func (p *LoginPlay) ID() int32 {
	return int32(packetid.ClientboundLogin)
}

// Decode reads LoginPlay fields from buffer.
func (p *LoginPlay) Decode(buf *protocol.Buffer) error {
	entityID, err := buf.ReadInt32()
	if err != nil {
		return fmt.Errorf("decode login_play entity_id: %w", err)
	}
	p.EntityID = entityID

	isHardcore, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play is_hardcore: %w", err)
	}
	p.IsHardcore = isHardcore

	dimCount, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play dimension_count: %w", err)
	}
	p.DimensionNames = make([]string, dimCount)
	for i := int32(0); i < dimCount; i++ {
		name, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode login_play dimension_name[%d]: %w", i, err)
		}
		p.DimensionNames[i] = name
	}

	maxPlayers, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play max_players: %w", err)
	}
	p.MaxPlayers = maxPlayers

	viewDist, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play view_distance: %w", err)
	}
	p.ViewDistance = viewDist

	simDist, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play simulation_distance: %w", err)
	}
	p.SimulationDistance = simDist

	reducedDebug, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play reduced_debug: %w", err)
	}
	p.ReducedDebugInfo = reducedDebug

	respawnScreen, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play respawn_screen: %w", err)
	}
	p.EnableRespawnScreen = respawnScreen

	limitedCrafting, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play limited_crafting: %w", err)
	}
	p.DoLimitedCrafting = limitedCrafting

	dimType, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play dimension_type: %w", err)
	}
	p.DimensionType = dimType

	dimName, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode login_play dimension_name: %w", err)
	}
	p.DimensionName = dimName

	hashedSeed, err := buf.ReadInt64()
	if err != nil {
		return fmt.Errorf("decode login_play hashed_seed: %w", err)
	}
	p.HashedSeed = hashedSeed

	gameMode, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode login_play game_mode: %w", err)
	}
	p.GameMode = gameMode

	prevGameMode, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("decode login_play previous_game_mode: %w", err)
	}
	p.PreviousGameMode = int8(prevGameMode)

	isDebug, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play is_debug: %w", err)
	}
	p.IsDebug = isDebug

	isFlat, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play is_flat: %w", err)
	}
	p.IsFlat = isFlat

	hasDeathLoc, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play has_death_location: %w", err)
	}
	p.HasDeathLocation = hasDeathLoc

	if p.HasDeathLocation {
		deathDim, err := buf.ReadString()
		if err != nil {
			return fmt.Errorf("decode login_play death_dimension: %w", err)
		}
		p.DeathDimensionName = deathDim

		posVal, err := buf.ReadInt64()
		if err != nil {
			return fmt.Errorf("decode login_play death_location: %w", err)
		}
		p.DeathLocationX = int32(posVal >> 38)
		p.DeathLocationY = int32(posVal << 52 >> 52)
		p.DeathLocationZ = int32(posVal << 26 >> 38)
	}

	portalCooldown, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play portal_cooldown: %w", err)
	}
	p.PortalCooldown = portalCooldown

	seaLevel, err := buf.ReadVarInt()
	if err != nil {
		return fmt.Errorf("decode login_play sea_level: %w", err)
	}
	p.SeaLevel = seaLevel

	enforcesChat, err := buf.ReadBool()
	if err != nil {
		return fmt.Errorf("decode login_play enforces_secure_chat: %w", err)
	}
	p.EnforcesSecureChat = enforcesChat

	return nil
}

// Encode writes LoginPlay fields to buffer.
func (p *LoginPlay) Encode(buf *protocol.Buffer) error {
	buf.WriteInt32(p.EntityID)
	buf.WriteBool(p.IsHardcore)

	buf.WriteVarInt(int32(len(p.DimensionNames)))
	for _, name := range p.DimensionNames {
		if err := buf.WriteString(name); err != nil {
			return fmt.Errorf("encode login_play dimension_name: %w", err)
		}
	}

	buf.WriteVarInt(p.MaxPlayers)
	buf.WriteVarInt(p.ViewDistance)
	buf.WriteVarInt(p.SimulationDistance)
	buf.WriteBool(p.ReducedDebugInfo)
	buf.WriteBool(p.EnableRespawnScreen)
	buf.WriteBool(p.DoLimitedCrafting)
	buf.WriteVarInt(p.DimensionType)

	if err := buf.WriteString(p.DimensionName); err != nil {
		return fmt.Errorf("encode login_play dimension_name: %w", err)
	}

	buf.WriteInt64(p.HashedSeed)
	if err := buf.WriteByte(p.GameMode); err != nil {
		return fmt.Errorf("encode login_play game_mode: %w", err)
	}
	if err := buf.WriteByte(byte(p.PreviousGameMode)); err != nil {
		return fmt.Errorf("encode login_play previous_game_mode: %w", err)
	}
	buf.WriteBool(p.IsDebug)
	buf.WriteBool(p.IsFlat)
	buf.WriteBool(p.HasDeathLocation)

	if p.HasDeathLocation {
		if err := buf.WriteString(p.DeathDimensionName); err != nil {
			return fmt.Errorf("encode login_play death_dimension: %w", err)
		}
		posVal := (int64(p.DeathLocationX&0x3FFFFFF) << 38) |
			(int64(p.DeathLocationZ&0x3FFFFFF) << 12) |
			int64(p.DeathLocationY&0xFFF)
		buf.WriteInt64(posVal)
	}

	buf.WriteVarInt(p.PortalCooldown)
	buf.WriteVarInt(p.SeaLevel)
	buf.WriteBool(p.EnforcesSecureChat)

	return nil
}
