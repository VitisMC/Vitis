package protocol

import "fmt"

const (
	// CurrentMinecraftVersion is the supported release name.
	CurrentMinecraftVersion = "1.21.4"
	// CurrentVersion is the only wire protocol version supported by this build.
	CurrentVersion int32 = 769
	// CurrentDataVersion is the persistent data version used by Minecraft 1.21.4.
	CurrentDataVersion int32 = 4189
)

const (
	StateHandshake State = iota
	StateStatus
	StateLogin
	StateConfiguration
	StatePlay
)

// State is the active protocol phase for a client session.
type State uint8

// Valid reports whether the protocol state is recognized.
func (s State) Valid() bool {
	switch s {
	case StateHandshake, StateStatus, StateLogin, StateConfiguration, StatePlay:
		return true
	default:
		return false
	}
}

// String returns a stable string representation for logs and metrics.
func (s State) String() string {
	switch s {
	case StateHandshake:
		return "handshake"
	case StateStatus:
		return "status"
	case StateLogin:
		return "login"
	case StateConfiguration:
		return "configuration"
	case StatePlay:
		return "play"
	default:
		return fmt.Sprintf("state(%d)", s)
	}
}
