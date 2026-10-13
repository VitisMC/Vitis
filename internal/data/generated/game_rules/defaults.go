package gamerules

import (
	"fmt"
	"maps"
	"strconv"
)

var defaults = map[string]string{
	"doFireTick":                       "true",
	"mobGriefing":                      "true",
	"keepInventory":                    "false",
	"doMobSpawning":                    "true",
	"doMobLoot":                        "true",
	"projectilesCanBreakBlocks":        "true",
	"doTileDrops":                      "true",
	"doEntityDrops":                    "true",
	"commandBlockOutput":               "true",
	"naturalRegeneration":              "true",
	"doDaylightCycle":                  "true",
	"logAdminCommands":                 "true",
	"showDeathMessages":                "true",
	"randomTickSpeed":                  "3",
	"sendCommandFeedback":              "true",
	"reducedDebugInfo":                 "false",
	"spectatorsGenerateChunks":         "true",
	"spawnRadius":                      "10",
	"disablePlayerMovementCheck":       "false",
	"disableElytraMovementCheck":       "false",
	"maxEntityCramming":                "24",
	"doWeatherCycle":                   "true",
	"doLimitedCrafting":                "false",
	"maxCommandChainLength":            "65536",
	"maxCommandForkCount":              "65536",
	"commandModificationBlockLimit":    "32768",
	"announceAdvancements":             "true",
	"disableRaids":                     "false",
	"doInsomnia":                       "true",
	"doImmediateRespawn":               "false",
	"playersNetherPortalDefaultDelay":  "80",
	"playersNetherPortalCreativeDelay": "0",
	"drowningDamage":                   "true",
	"fallDamage":                       "true",
	"fireDamage":                       "true",
	"freezeDamage":                     "true",
	"doPatrolSpawning":                 "true",
	"doTraderSpawning":                 "true",
	"doWardenSpawning":                 "true",
	"forgiveDeadPlayers":               "true",
	"universalAnger":                   "false",
	"playersSleepingPercentage":        "100",
	"blockExplosionDropDecay":          "true",
	"mobExplosionDropDecay":            "true",
	"tntExplosionDropDecay":            "false",
	"snowAccumulationHeight":           "1",
	"waterSourceConversion":            "true",
	"lavaSourceConversion":             "false",
	"globalSoundEvents":                "true",
	"doVinesSpread":                    "true",
	"enderPearlsVanishOnDeath":         "true",
	"minecartMaxSpeed":                 "8",
	"spawnChunkRadius":                 "2",
}

// Defaults returns an independent set of Java Edition 1.21.4 game rules.
func Defaults() map[string]string { return maps.Clone(defaults) }

// Default returns the default value of a registered rule.
func Default(name string) string { return defaults[name] }

// Validate checks the vanilla type and explicit range of a game-rule value.
func Validate(name, value string) error {
	info := GameRuleByName(name)
	if info == nil {
		return fmt.Errorf("unknown game rule: %s", name)
	}
	if info.Type == "boolean" {
		if value != "true" && value != "false" {
			return fmt.Errorf("invalid boolean game rule")
		}
		return nil
	}
	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid integer game rule: %w", err)
	}
	if name == "spawnChunkRadius" && (number < 0 || number > 32) || name == "minecartMaxSpeed" && (number < 1 || number > 1000) {
		return fmt.Errorf("game rule out of range")
	}
	return nil
}
