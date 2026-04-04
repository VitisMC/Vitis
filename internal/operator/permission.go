package operator

// Permission levels matching vanilla Minecraft.
const (
	LevelNormal     = 0
	LevelModerator  = 1
	LevelGamemaster = 2
	LevelAdmin      = 3
	LevelOwner      = 4
)

// LevelName returns a human-readable name for a permission level.
func LevelName(level int) string {
	switch level {
	case LevelNormal:
		return "normal"
	case LevelModerator:
		return "moderator"
	case LevelGamemaster:
		return "gamemaster"
	case LevelAdmin:
		return "admin"
	case LevelOwner:
		return "owner"
	default:
		return "unknown"
	}
}
