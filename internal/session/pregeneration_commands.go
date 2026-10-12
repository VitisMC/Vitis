package session

import (
	"fmt"

	"github.com/vitismc/vitis/internal/world"
)

// DefaultChunkyWorld returns the configured overworld name.
func (s *ServerControlAdapter) DefaultChunkyWorld() string {
	return s.DefaultPregenerationWorld
}

// StartChunky starts pregeneration in one world.
func (s *ServerControlAdapter) StartChunky(name, shape string, centerX, centerZ, radius int) error {
	w, err := s.chunkyWorld(name)
	if err != nil {
		return err
	}
	return w.StartPregeneration(shape, centerX, centerZ, radius)
}

// PauseChunky pauses pregeneration in one world.
func (s *ServerControlAdapter) PauseChunky(name string) error {
	w, err := s.chunkyWorld(name)
	if err != nil {
		return err
	}
	return w.PausePregeneration()
}

// ContinueChunky resumes pregeneration in one world.
func (s *ServerControlAdapter) ContinueChunky(name string) error {
	w, err := s.chunkyWorld(name)
	if err != nil {
		return err
	}
	return w.ContinuePregeneration()
}

// CancelChunky cancels pregeneration in one world.
func (s *ServerControlAdapter) CancelChunky(name string) error {
	w, err := s.chunkyWorld(name)
	if err != nil {
		return err
	}
	return w.CancelPregeneration()
}

// ChunkyStatus returns one world's pregeneration progress.
func (s *ServerControlAdapter) ChunkyStatus(name string) (world.PregenerationStatus, error) {
	w, err := s.chunkyWorld(name)
	if err != nil {
		return world.PregenerationStatus{}, err
	}
	return w.PregenerationStatus(), nil
}

func (s *ServerControlAdapter) chunkyWorld(name string) (*world.World, error) {
	if s == nil || s.PregenerationWorlds == nil {
		return nil, fmt.Errorf("world manager unavailable")
	}
	if name == "" {
		name = s.DefaultPregenerationWorld
	}
	w, ok := s.PregenerationWorlds.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown world %q", name)
	}
	return w, nil
}
