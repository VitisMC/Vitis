package auth

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/vitismc/vitis/internal/protocol"
)

const sessionServerURL = "https://sessionserver.mojang.com/session/minecraft/hasJoined"

var httpClient = &http.Client{Timeout: 10 * time.Second}

// GameProfile holds the authenticated player profile from Mojang.
type GameProfile struct {
	UUID       protocol.UUID
	Name       string
	Properties []ProfileProperty
}

// ProfileProperty is a single property in the game profile (e.g. textures).
type ProfileProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

type hasJoinedResponse struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Properties []ProfileProperty `json:"properties"`
}

// HasJoined verifies a player's join with Mojang's session server.
func HasJoined(username string, serverHash string) (*GameProfile, error) {
	url := fmt.Sprintf("%s?username=%s&serverId=%s", sessionServerURL, username, serverHash)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("mojang has_joined request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("mojang has_joined: authentication failed for %s", username)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mojang has_joined: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("mojang has_joined: read body: %w", err)
	}

	var result hasJoinedResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("mojang has_joined: decode json: %w", err)
	}

	uuid, err := parseUUIDNoDashes(result.ID)
	if err != nil {
		return nil, fmt.Errorf("mojang has_joined: parse uuid: %w", err)
	}

	return &GameProfile{
		UUID:       uuid,
		Name:       result.Name,
		Properties: result.Properties,
	}, nil
}

// ServerHash computes the Minecraft-style server hash for session authentication.
func ServerHash(serverID string, sharedSecret, publicKey []byte) string {
	h := sha1.New()
	h.Write([]byte(serverID))
	h.Write(sharedSecret)
	h.Write(publicKey)
	hash := h.Sum(nil)
	return minecraftHexDigest(hash)
}

// minecraftHexDigest formats a SHA-1 hash using Minecraft's twos-complement hex format.
func minecraftHexDigest(hash []byte) string {
	bigInt := new(big.Int).SetBytes(hash)
	if hash[0]&0x80 != 0 {
		// Negative: twos complement
		complement := new(big.Int).SetBytes(hash)
		complement.Not(complement)
		for i := range hash {
			hash[i] = ^hash[i]
		}
		bigInt = new(big.Int).SetBytes(hash)
		bigInt.Add(bigInt, big.NewInt(1))
		return "-" + strings.TrimLeft(bigInt.Text(16), "0")
	}
	return strings.TrimLeft(bigInt.Text(16), "0")
}

func parseUUIDNoDashes(s string) (protocol.UUID, error) {
	if len(s) != 32 {
		return protocol.UUID{}, fmt.Errorf("invalid uuid length: %d", len(s))
	}
	withDashes := s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
	return protocol.ParseUUID(withDashes)
}
