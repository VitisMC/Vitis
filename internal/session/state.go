package session

import (
	"errors"
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
)

var (
	ErrNilConnection             = errors.New("nil connection")
	ErrNilRegistry               = errors.New("nil protocol registry")
	ErrSessionClosing            = errors.New("session closing")
	ErrSessionClosed             = errors.New("session closed")
	ErrSessionNotFound           = errors.New("session not found")
	ErrSessionAlreadyExists      = errors.New("session already exists")
	ErrInvalidProtocolTransition = errors.New("invalid protocol transition")
)

const (
	LifecycleActive LifecycleState = iota
	LifecycleClosing
	LifecycleClosed
)

// LifecycleState tracks session lifecycle state.
type LifecycleState uint8

// String returns a stable string representation of the lifecycle state.
func (s LifecycleState) String() string {
	switch s {
	case LifecycleActive:
		return "active"
	case LifecycleClosing:
		return "closing"
	case LifecycleClosed:
		return "closed"
	default:
		return fmt.Sprintf("lifecycle(%d)", s)
	}
}

// ValidateProtocolTransition validates session protocol state transitions.
func ValidateProtocolTransition(current protocol.State, next protocol.State) error {
	if !current.Valid() || !next.Valid() {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidProtocolTransition, current.String(), next.String())
	}
	if current == next {
		return nil
	}

	switch current {
	case protocol.StateHandshake:
		if next == protocol.StateStatus || next == protocol.StateLogin {
			return nil
		}
	case protocol.StateLogin:
		if next == protocol.StateConfiguration {
			return nil
		}
	case protocol.StateConfiguration:
		if next == protocol.StatePlay || next == protocol.StateConfiguration {
			return nil
		}
	case protocol.StateStatus:
		if next == protocol.StateStatus {
			return nil
		}
	case protocol.StatePlay:
		if next == protocol.StatePlay || next == protocol.StateConfiguration {
			return nil
		}
	}

	return fmt.Errorf("%w: %s -> %s", ErrInvalidProtocolTransition, current.String(), next.String())
}
