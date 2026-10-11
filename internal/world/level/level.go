package level

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"time"

	gamerules "github.com/vitismc/vitis/internal/data/generated/game_rules"
	"github.com/vitismc/vitis/internal/nbt"
	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/storage"
)

const maxPersistentNBTSize = 16 << 20

// readGzipNBT reads one bounded gzip-compressed NBT document and verifies the
// complete stream before returning it to an NBT decoder.
func readGzipNBT(source io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(source)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(gz, maxPersistentNBTSize+1))
	closeErr := gz.Close()
	if readErr != nil {
		return nil, readErr
	}
	if len(data) > maxPersistentNBTSize {
		return nil, fmt.Errorf("decompressed NBT exceeds %d byte limit", maxPersistentNBTSize)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return data, nil
}

// Level holds the parsed level.dat data for a Minecraft world.
type Level struct {
	DataVersion int32
	LevelName   string
	SpawnX      int32
	SpawnY      int32
	SpawnZ      int32
	SpawnAngle  float32
	GameType    int32
	Hardcore    bool
	Difficulty  int8
	DayTime     int64
	Time        int64
	LastPlayed  int64
	Version     int32

	AllowCommands bool
	Initialized   bool

	Seed             int64
	GenerateFeatures bool

	GameRules map[string]string

	ServerBrands []string

	raw *nbt.Compound
}

// Open reads and parses a level.dat file from the given world directory.
func Open(worldDir string) (*Level, error) {
	path := worldDir + "/level.dat"
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("level: open %s: %w", path, err)
	}
	defer f.Close()

	data, err := readGzipNBT(f)
	if err != nil {
		return nil, fmt.Errorf("level: read data: %w", err)
	}

	dec := nbt.NewDecoder(data)
	_, root, err := dec.ReadNamedRootCompound()
	if err != nil {
		return nil, fmt.Errorf("level: decode nbt: %w", err)
	}

	dataComp, ok := root.GetCompound("Data")
	if !ok {
		return nil, fmt.Errorf("level: missing Data compound")
	}

	l := &Level{raw: root}
	l.parseData(dataComp)
	for name, value := range l.GameRules {
		if gamerules.GameRuleByName(name) != nil {
			if err := gamerules.Validate(name, value); err != nil {
				return nil, fmt.Errorf("level game rule %s: %w", name, err)
			}
		}
	}
	return l, nil
}

func (l *Level) parseData(data *nbt.Compound) {
	l.GameRules = gamerules.Defaults()
	if rules, ok := data.GetCompound("GameRules"); ok {
		for _, name := range rules.Keys() {
			if value, ok := rules.GetString(name); ok {
				l.GameRules[name] = value
			}
		}
	}
	if v, ok := data.GetInt("DataVersion"); ok {
		l.DataVersion = v
	}
	if v, ok := data.GetString("LevelName"); ok {
		l.LevelName = v
	}
	if v, ok := data.GetInt("SpawnX"); ok {
		l.SpawnX = v
	}
	if v, ok := data.GetInt("SpawnY"); ok {
		l.SpawnY = v
	}
	if v, ok := data.GetInt("SpawnZ"); ok {
		l.SpawnZ = v
	}
	if v, ok := data.GetFloat("SpawnAngle"); ok {
		l.SpawnAngle = v
	}
	if v, ok := data.GetInt("GameType"); ok {
		l.GameType = v
	}
	if v, ok := data.GetBool("hardcore"); ok {
		l.Hardcore = v
	}
	if v, ok := data.GetByte("Difficulty"); ok {
		l.Difficulty = v
	}
	if v, ok := data.GetLong("DayTime"); ok {
		l.DayTime = v
	}
	if v, ok := data.GetLong("Time"); ok {
		l.Time = v
	}
	if v, ok := data.GetLong("LastPlayed"); ok {
		l.LastPlayed = v
	}
	if v, ok := data.GetBool("allowCommands"); ok {
		l.AllowCommands = v
	}
	if v, ok := data.GetBool("initialized"); ok {
		l.Initialized = v
	}
	if v, ok := data.GetInt("version"); ok {
		l.Version = v
	}
	if worldGen, ok := data.GetCompound("WorldGenSettings"); ok {
		if v, exists := worldGen.GetLong("seed"); exists {
			l.Seed = v
		}
		if v, exists := worldGen.GetBool("generate_features"); exists {
			l.GenerateFeatures = v
		}
	}
}

// BonusChestEnabled reads the optional spawn chest flag from preserved world metadata.
func (l *Level) BonusChestEnabled() bool {
	if l == nil || l.raw == nil {
		return false
	}
	data, ok := l.raw.GetCompound("Data")
	if !ok {
		return false
	}
	settings, ok := data.GetCompound("WorldGenSettings")
	if !ok {
		return false
	}
	enabled, _ := settings.GetBool("bonus_chest")
	return enabled
}

// New creates a new Level with sensible defaults for world creation.
func New(levelName string, spawnX, spawnY, spawnZ int32) *Level {
	return &Level{
		DataVersion:   protocol.CurrentDataVersion,
		LevelName:     levelName,
		SpawnX:        spawnX,
		SpawnY:        spawnY,
		SpawnZ:        spawnZ,
		GameType:      1,
		Difficulty:    1,
		AllowCommands: true,
		Initialized:   true,
		Version:       19133,
		LastPlayed:    time.Now().UnixMilli(),
		ServerBrands:  []string{"Vitis"},
		GameRules:     gamerules.Defaults(),
	}
}

// Save writes the level.dat file to the given world directory.
func (l *Level) Save(worldDir string) error {
	if l == nil {
		return fmt.Errorf("level: nil level")
	}
	if l.DataVersion != protocol.CurrentDataVersion {
		return fmt.Errorf("level: unsupported DataVersion %d", l.DataVersion)
	}
	if err := os.MkdirAll(worldDir, 0755); err != nil {
		return fmt.Errorf("level: mkdir %s: %w", worldDir, err)
	}

	root := l.raw.Clone()
	if root == nil {
		root = nbt.NewCompound()
	}
	data := l.buildDataCompound()
	root.PutCompound("Data", data)

	path := worldDir + "/level.dat"
	payload, err := encodeGzipNBT(root, 4096)
	if err != nil {
		return fmt.Errorf("level: %w", err)
	}
	if err := backupCurrentFile(path, path+"_old"); err != nil {
		return fmt.Errorf("level: %w", err)
	}
	if err := storage.WriteFileAtomic(path, payload, 0644); err != nil {
		return fmt.Errorf("level: save: %w", err)
	}
	l.raw = root.Clone()
	return nil
}

func (l *Level) buildDataCompound() *nbt.Compound {
	var data *nbt.Compound
	if l.raw != nil {
		data, _ = l.raw.GetCompound("Data")
		data = data.Clone()
	}
	if data == nil {
		data = nbt.NewCompound()
	}
	rules := nbt.NewCompound()
	for name, value := range l.GameRules {
		rules.PutString(name, value)
	}
	data.PutCompound("GameRules", rules)
	data.PutString("LevelName", l.LevelName)
	data.PutInt("SpawnX", l.SpawnX)
	data.PutInt("SpawnY", l.SpawnY)
	data.PutInt("SpawnZ", l.SpawnZ)
	data.PutFloat("SpawnAngle", l.SpawnAngle)
	data.PutInt("GameType", l.GameType)
	data.PutBool("hardcore", l.Hardcore)
	data.PutByte("Difficulty", l.Difficulty)
	data.PutLong("DayTime", l.DayTime)
	data.PutLong("Time", l.Time)
	data.PutLong("LastPlayed", time.Now().UnixMilli())
	data.PutBool("allowCommands", l.AllowCommands)
	data.PutBool("initialized", l.Initialized)
	data.PutInt("version", l.Version)

	data.PutInt("DataVersion", protocol.CurrentDataVersion)

	version, _ := data.GetCompound("Version")
	version = version.Clone()
	if version == nil {
		version = nbt.NewCompound()
	}
	version.PutInt("Id", protocol.CurrentDataVersion)
	version.PutString("Name", protocol.CurrentMinecraftVersion)
	version.PutString("Series", "main")
	version.PutByte("Snapshot", 0)
	data.PutCompound("Version", version)

	if l.ServerBrands != nil {
		brands := nbt.NewList(nbt.TagString)
		for _, b := range l.ServerBrands {
			brands.Add(b)
		}
		data.PutList("ServerBrands", brands)
	}

	if _, exists := data.GetCompound("DataPacks"); !exists {
		dataPacks := nbt.NewCompound()
		dataPacks.PutList("Enabled", nbt.NewList(nbt.TagString).Add("vanilla"))
		dataPacks.PutList("Disabled", nbt.NewList(nbt.TagString))
		data.PutCompound("DataPacks", dataPacks)
	}

	worldGen, _ := data.GetCompound("WorldGenSettings")
	worldGen = worldGen.Clone()
	if worldGen == nil {
		worldGen = nbt.NewCompound()
	}
	if _, exists := worldGen.GetBool("bonus_chest"); !exists {
		worldGen.PutBool("bonus_chest", false)
	}
	worldGen.PutBool("generate_features", l.GenerateFeatures)
	worldGen.PutLong("seed", l.Seed)
	data.PutCompound("WorldGenSettings", worldGen)

	return data
}
