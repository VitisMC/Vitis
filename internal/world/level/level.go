package level

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vitismc/vitis/internal/nbt"
)

// Level holds the parsed level.dat data for a Minecraft world.
type Level struct {
	LevelName  string
	SpawnX     int32
	SpawnY     int32
	SpawnZ     int32
	SpawnAngle float32
	GameType   int32
	Hardcore   bool
	Difficulty int8
	DayTime    int64
	Time       int64
	LastPlayed int64
	Version    int32

	AllowCommands bool
	Initialized   bool

	Seed             int64
	GenerateFeatures bool

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

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("level: gzip reader: %w", err)
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
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
	return l, nil
}

func (l *Level) parseData(data *nbt.Compound) {
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
}

// New creates a new Level with sensible defaults for world creation.
func New(levelName string, spawnX, spawnY, spawnZ int32) *Level {
	return &Level{
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
	}
}

// Save writes the level.dat file to the given world directory.
func (l *Level) Save(worldDir string) error {
	if err := os.MkdirAll(worldDir, 0755); err != nil {
		return fmt.Errorf("level: mkdir %s: %w", worldDir, err)
	}

	data := l.buildDataCompound()
	root := nbt.NewCompound()
	root.PutCompound("Data", data)

	enc := nbt.NewEncoder(4096)
	if err := enc.WriteNamedRootCompound("", root); err != nil {
		return fmt.Errorf("level: encode nbt: %w", err)
	}

	path := worldDir + "/level.dat"
	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("level: create tmp: %w", err)
	}

	gz := gzip.NewWriter(f)
	if _, err := gz.Write(enc.Bytes()); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("level: gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("level: gzip close: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("level: close tmp: %w", err)
	}

	oldPath := path + "_old"
	os.Rename(path, oldPath)
	if err := os.Rename(tmpPath, path); err != nil {
		os.Rename(oldPath, path)
		return fmt.Errorf("level: rename: %w", err)
	}

	return nil
}

func (l *Level) buildDataCompound() *nbt.Compound {
	data := nbt.NewCompound()
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

	data.PutInt("DataVersion", 4189)

	version := nbt.NewCompound()
	version.PutInt("Id", 4189)
	version.PutString("Name", "1.21.4")
	version.PutString("Series", "main")
	version.PutByte("Snapshot", 0)
	data.PutCompound("Version", version)

	brands := nbt.NewList(nbt.TagString)
	for _, b := range l.ServerBrands {
		brands.Add(b)
	}
	data.PutList("ServerBrands", brands)

	dataPacks := nbt.NewCompound()
	enabled := nbt.NewList(nbt.TagString)
	enabled.Add("vanilla")
	dataPacks.PutList("Enabled", enabled)
	dataPacks.PutList("Disabled", nbt.NewList(nbt.TagString))
	data.PutCompound("DataPacks", dataPacks)

	worldGen := nbt.NewCompound()
	worldGen.PutBool("bonus_chest", false)
	worldGen.PutBool("generate_features", l.GenerateFeatures)
	worldGen.PutLong("seed", l.Seed)
	data.PutCompound("WorldGenSettings", worldGen)

	return data
}
