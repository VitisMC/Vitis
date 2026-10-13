package session

import (
	"fmt"

	"github.com/vitismc/vitis/internal/protocol"
)

type loginPhase uint8

const (
	loginAwaitStart loginPhase = iota
	loginAwaitEncryptionResponse
	loginAwaitAcknowledgement
	loginComplete
)

type configurationPhase uint8

const (
	configurationNotStarted configurationPhase = iota
	configurationAwaitKnownPacks
	configurationAwaitFinishAcknowledgement
	configurationComplete
)

type sessionProtocolFlow struct {
	login         loginPhase
	configuration configurationPhase
}

type protocolFlowKey struct{}

func flowFor(s Session) (*sessionProtocolFlow, error) {
	ds, ok := s.(*DefaultSession)
	if !ok || ds == nil {
		return nil, fmt.Errorf("protocol flow: unsupported session type %T", s)
	}
	value, _ := ds.sessionData.LoadOrStore(protocolFlowKey{}, &sessionProtocolFlow{})
	flow, ok := value.(*sessionProtocolFlow)
	if !ok || flow == nil {
		return nil, fmt.Errorf("protocol flow: invalid session state")
	}
	return flow, nil
}

func transitionProtocol(s Session, next protocol.State) error {
	ds, ok := s.(*DefaultSession)
	if !ok || ds == nil {
		return fmt.Errorf("protocol transition: unsupported session type %T", s)
	}
	return ds.transitionProtocol(next)
}

func validPlayerName(name string) bool {
	if len(name) < 3 || len(name) > 16 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}
