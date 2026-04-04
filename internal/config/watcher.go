package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"time"
)

const (
	defaultWatchPollInterval = 500 * time.Millisecond
	defaultWatchDebounce     = 150 * time.Millisecond
)

// WatchOptions configures manager file watch behavior for automatic hot reload.
type WatchOptions struct {
	PollInterval time.Duration
	Debounce     time.Duration
	OnReload     func(*Config)
	OnError      func(error)
}

// Watch polls the managed configuration file and applies reload when file contents change.
func (m *Manager) Watch(ctx context.Context, options WatchOptions) error {
	if m == nil {
		return fmt.Errorf("watch config manager: nil manager")
	}
	if ctx == nil {
		return fmt.Errorf("watch config manager: nil context")
	}

	pollInterval := options.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultWatchPollInterval
	}

	debounce := options.Debounce
	if debounce < 0 {
		debounce = 0
	}
	if debounce == 0 {
		debounce = defaultWatchDebounce
	}

	path := m.Path()
	lastAppliedDigest, err := fileDigest(path)
	if err != nil {
		return fmt.Errorf("watch config manager path=%q: %w", path, err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastReportedFailureDigest []byte

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			digest, digestErr := fileDigest(path)
			if digestErr != nil {
				reportWatchError(options.OnError, fmt.Errorf("watch config path=%q: %w", path, digestErr))
				continue
			}
			if bytes.Equal(digest, lastAppliedDigest) {
				lastReportedFailureDigest = nil
				continue
			}

			if debounce > 0 {
				waitTimer := time.NewTimer(debounce)
				select {
				case <-ctx.Done():
					waitTimer.Stop()
					return nil
				case <-waitTimer.C:
				}

				stableDigest, stableDigestErr := fileDigest(path)
				if stableDigestErr != nil {
					reportWatchError(options.OnError, fmt.Errorf("watch config path=%q after debounce: %w", path, stableDigestErr))
					continue
				}
				if !bytes.Equal(stableDigest, digest) {
					continue
				}
			}

			cfg, reloadErr := m.Reload()
			if reloadErr != nil {
				if !bytes.Equal(lastReportedFailureDigest, digest) {
					reportWatchError(options.OnError, fmt.Errorf("hot reload config path=%q: %w", path, reloadErr))
					lastReportedFailureDigest = append(lastReportedFailureDigest[:0], digest...)
				}
				continue
			}

			lastAppliedDigest = append(lastAppliedDigest[:0], digest...)
			lastReportedFailureDigest = nil
			if options.OnReload != nil {
				options.OnReload(cfg)
			}
		}
	}
}

func reportWatchError(callback func(error), err error) {
	if callback == nil || err == nil {
		return
	}
	callback(err)
}

func fileDigest(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(content)
	digest := make([]byte, len(hash))
	copy(digest, hash[:])
	return digest, nil
}
