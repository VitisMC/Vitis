package level

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vitismc/vitis/internal/nbt"
)

// PlayerData holds persistent player state loaded from/saved to playerdata NBT files.
type PlayerData struct {
	UUID      string
	PosX      float64
	PosY      float64
	PosZ      float64
	Yaw       float32
	Pitch     float32
	OnGround  bool
	GameMode  int32
	Health    float32
	FoodLevel int32
	FoodSat   float32
	Dimension string
	XPLevel   int32
	XPTotal   int32
}

// DefaultPlayerData returns a new player data with sensible defaults.
func DefaultPlayerData(uuid string, spawnX, spawnY, spawnZ float64, gameMode int32) *PlayerData {
	return &PlayerData{
		UUID:      uuid,
		PosX:      spawnX,
		PosY:      spawnY,
		PosZ:      spawnZ,
		GameMode:  gameMode,
		Health:    20.0,
		FoodLevel: 20,
		FoodSat:   5.0,
		Dimension: "minecraft:overworld",
	}
}

// LoadPlayerData reads player data from a gzipped NBT file in the world directory.
func LoadPlayerData(worldDir, uuid string) (*PlayerData, error) {
	path := playerDataPath(worldDir, uuid)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("playerdata: open %s: %w", path, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("playerdata: gzip: %w", err)
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("playerdata: read: %w", err)
	}

	dec := nbt.NewDecoder(data)
	_, root, err := dec.ReadNamedRootCompound()
	if err != nil {
		return nil, fmt.Errorf("playerdata: decode: %w", err)
	}

	pd := &PlayerData{UUID: uuid}
	pd.parseCompound(root)
	return pd, nil
}

func (pd *PlayerData) parseCompound(c *nbt.Compound) {
	if v, ok := c.GetInt("playerGameType"); ok {
		pd.GameMode = v
	}
	if v, ok := c.GetFloat("Health"); ok {
		pd.Health = v
	}
	if v, ok := c.GetInt("foodLevel"); ok {
		pd.FoodLevel = v
	}
	if v, ok := c.GetFloat("foodSaturationLevel"); ok {
		pd.FoodSat = v
	}
	if v, ok := c.GetString("Dimension"); ok {
		pd.Dimension = v
	}
	if v, ok := c.GetBool("OnGround"); ok {
		pd.OnGround = v
	}
	if v, ok := c.GetInt("XpLevel"); ok {
		pd.XPLevel = v
	}
	if v, ok := c.GetInt("XpTotal"); ok {
		pd.XPTotal = v
	}

	if posList, ok := c.GetList("Pos"); ok && posList.Len() >= 3 {
		elems := posList.Elements()
		if x, ok := elems[0].(float64); ok {
			pd.PosX = x
		}
		if y, ok := elems[1].(float64); ok {
			pd.PosY = y
		}
		if z, ok := elems[2].(float64); ok {
			pd.PosZ = z
		}
	}

	if rotList, ok := c.GetList("Rotation"); ok && rotList.Len() >= 2 {
		elems := rotList.Elements()
		if yaw, ok := elems[0].(float32); ok {
			pd.Yaw = yaw
		}
		if pitch, ok := elems[1].(float32); ok {
			pd.Pitch = pitch
		}
	}
}

// Save writes player data to a gzipped NBT file in the world directory.
func (pd *PlayerData) Save(worldDir string) error {
	dir := filepath.Join(worldDir, "playerdata")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("playerdata: mkdir: %w", err)
	}

	c := nbt.NewCompound()
	c.PutInt("DataVersion", 4189)
	c.PutInt("playerGameType", pd.GameMode)
	c.PutFloat("Health", pd.Health)
	c.PutInt("foodLevel", pd.FoodLevel)
	c.PutFloat("foodSaturationLevel", pd.FoodSat)
	c.PutString("Dimension", pd.Dimension)
	c.PutBool("OnGround", pd.OnGround)
	c.PutInt("XpLevel", pd.XPLevel)
	c.PutInt("XpTotal", pd.XPTotal)

	pos := nbt.NewList(nbt.TagDouble)
	pos.Add(pd.PosX)
	pos.Add(pd.PosY)
	pos.Add(pd.PosZ)
	c.PutList("Pos", pos)

	rot := nbt.NewList(nbt.TagFloat)
	rot.Add(pd.Yaw)
	rot.Add(pd.Pitch)
	c.PutList("Rotation", rot)

	enc := nbt.NewEncoder(2048)
	if err := enc.WriteNamedRootCompound("", c); err != nil {
		return fmt.Errorf("playerdata: encode: %w", err)
	}

	path := playerDataPath(worldDir, pd.UUID)
	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("playerdata: create: %w", err)
	}

	gz := gzip.NewWriter(f)
	if _, err := gz.Write(enc.Bytes()); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := gz.Close(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	os.Rename(path, path+"_old")
	return os.Rename(tmpPath, path)
}

func playerDataPath(worldDir, uuid string) string {
	return filepath.Join(worldDir, "playerdata", uuid+".dat")
}
