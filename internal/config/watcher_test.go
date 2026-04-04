package config

import (
	"context"
	"testing"
	"time"
)

func TestManagerWatchReloadsOnFileChange(t *testing.T) {
	path := writeConfigFile(t, `
server:
  host: "0.0.0.0"
  port: 25565
  max_players: 120
  motd: "Before"
`)

	manager := NewManager(path)
	if _, err := manager.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reloadCh := make(chan *Config, 1)
	errCh := make(chan error, 2)
	watchDone := make(chan error, 1)

	go func() {
		watchDone <- manager.Watch(ctx, WatchOptions{
			PollInterval: 20 * time.Millisecond,
			Debounce:     20 * time.Millisecond,
			OnReload: func(cfg *Config) {
				select {
				case reloadCh <- cfg:
				default:
				}
			},
			OnError: func(err error) {
				select {
				case errCh <- err:
				default:
				}
			},
		})
	}()

	time.Sleep(80 * time.Millisecond)
	mustWrite(t, path, `
server:
  host: "0.0.0.0"
  port: 25565
  max_players: 140
  motd: "After"
`)

	select {
	case err := <-errCh:
		t.Fatalf("watch reported error: %v", err)
	case cfg := <-reloadCh:
		if cfg.Server.MaxPlayers != 140 {
			t.Fatalf("expected max_players 140, got %d", cfg.Server.MaxPlayers)
		}
		if cfg.Server.MOTD != "After" {
			t.Fatalf("expected motd After, got %q", cfg.Server.MOTD)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch reload")
	}

	if manager.Get().Server.MaxPlayers != 140 {
		t.Fatalf("manager did not publish watched config, got %d", manager.Get().Server.MaxPlayers)
	}

	cancel()
	select {
	case err := <-watchDone:
		if err != nil {
			t.Fatalf("watch exited with error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch stop")
	}
}

func TestManagerWatchKeepsLastGoodConfigOnInvalidChange(t *testing.T) {
	path := writeConfigFile(t, `
server:
  host: "0.0.0.0"
  port: 25565
  max_players: 120
`)

	manager := NewManager(path)
	if _, err := manager.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 2)
	watchDone := make(chan error, 1)

	go func() {
		watchDone <- manager.Watch(ctx, WatchOptions{
			PollInterval: 20 * time.Millisecond,
			Debounce:     20 * time.Millisecond,
			OnError: func(err error) {
				select {
				case errCh <- err:
				default:
				}
			},
		})
	}()

	time.Sleep(80 * time.Millisecond)
	mustWrite(t, path, `
server:
  host: "0.0.0.0"
  port: 70000
  max_players: 120
`)

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch error on invalid config")
	}

	if manager.Get().Server.Port != 25565 {
		t.Fatalf("expected previous valid port 25565, got %d", manager.Get().Server.Port)
	}

	cancel()
	select {
	case err := <-watchDone:
		if err != nil {
			t.Fatalf("watch exited with error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch stop")
	}
}
