package vanilla

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// downloadServer resolves one official version and verifies the complete JAR download.
func downloadServer(ctx context.Context, client *http.Client, manifest, versionID, target string) error {
	var versions struct {
		Versions []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"versions"`
	}
	if err := downloadJSON(ctx, client, manifest, &versions); err != nil {
		return fmt.Errorf("download Minecraft manifest: %w", err)
	}
	versionURL := ""
	for _, version := range versions.Versions {
		if version.ID == versionID {
			if versionURL != "" {
				return fmt.Errorf("duplicate Minecraft version %q", versionID)
			}
			versionURL = version.URL
		}
	}
	if versionURL == "" {
		return fmt.Errorf("Minecraft version %q is absent from official manifest", versionID)
	}
	var metadata struct {
		ID        string `json:"id"`
		Downloads struct {
			Server struct {
				URL  string `json:"url"`
				SHA1 string `json:"sha1"`
				Size int64  `json:"size"`
			} `json:"server"`
		} `json:"downloads"`
	}
	if err := downloadJSON(ctx, client, versionURL, &metadata); err != nil {
		return fmt.Errorf("download Minecraft version metadata: %w", err)
	}
	server := metadata.Downloads.Server
	checksum, err := hex.DecodeString(server.SHA1)
	if metadata.ID != versionID || err != nil || len(checksum) != sha1.Size || server.Size <= 0 || server.Size > maxServerSize {
		return fmt.Errorf("invalid server download metadata for %q", versionID)
	}
	response, err := get(ctx, client, server.URL)
	if err != nil {
		return fmt.Errorf("download server JAR: %w", err)
	}
	defer response.Body.Close()
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha1.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, server.Size+1))
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return fmt.Errorf("write server JAR: %w", err)
	}
	if size != server.Size || !bytes.Equal(hash.Sum(nil), checksum) {
		return fmt.Errorf("server JAR size or SHA-1 checksum mismatch")
	}
	return ctx.Err()
}

// downloadJSON reads a bounded metadata response and rejects malformed JSON.
func downloadJSON(ctx context.Context, client *http.Client, address string, target any) error {
	response, err := get(ctx, client, address)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxMetadataSize {
		return fmt.Errorf("Minecraft metadata exceeds size limit")
	}
	return json.Unmarshal(data, target)
}

// get opens an HTTPS resource and rejects unsuccessful responses.
func get(ctx context.Context, client *http.Client, address string) (*http.Response, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("invalid HTTPS download URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", response.StatusCode, parsed.Host)
	}
	return response, nil
}

// secureRedirect bounds redirects and prevents an HTTPS downgrade.
func secureRedirect(request *http.Request, previous []*http.Request) error {
	if len(previous) >= 10 || request.URL.Scheme != "https" || request.URL.User != nil {
		return fmt.Errorf("unsafe or excessive download redirects")
	}
	return nil
}
