package session

import (
	"bytes"
	"crypto/rsa"
	"fmt"

	"github.com/vitismc/vitis/internal/auth"
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/protocol"
	protocrypto "github.com/vitismc/vitis/internal/protocol/crypto"
	loginpacket "github.com/vitismc/vitis/internal/protocol/packets/login"
)

// LoginConfig configures the login handshake behavior.
type LoginConfig struct {
	CompressionThreshold int
	OnlineMode           bool
	PrivateKey           *rsa.PrivateKey
	PublicKeyDER         []byte
	OnLoginSuccess       func(session Session, name string, uuid protocol.UUID)
}

type loginSessionKey struct{}

type pendingLogin struct {
	username    string
	verifyToken []byte
}

// RegisterLoginHandlers registers handlers for Login-state packets on a packet router.
func RegisterLoginHandlers(router PacketRouter, cfg LoginConfig) error {
	if router == nil {
		return ErrNilRegistry
	}

	loginStartID := loginpacket.NewLoginStart().ID()
	if err := router.Register(protocol.StateLogin, loginStartID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*loginpacket.LoginStart)
		if !ok {
			return protocol.ErrNilPacket
		}

		logger.Info("login_start", "session", s.ID(), "name", pkt.Name)

		if cfg.OnlineMode && cfg.PrivateKey != nil {
			verifyToken, err := protocrypto.GenerateVerifyToken()
			if err != nil {
				return fmt.Errorf("generate verify token: %w", err)
			}

			s.(*DefaultSession).sessionData.Store(loginSessionKey{}, &pendingLogin{
				username:    pkt.Name,
				verifyToken: verifyToken,
			})

			encReq := &loginpacket.EncryptionRequest{
				ServerID:           "",
				PublicKey:          cfg.PublicKeyDER,
				VerifyToken:        verifyToken,
				ShouldAuthenticate: true,
			}
			if err := s.Send(encReq); err != nil {
				return err
			}
			logger.Debug("sent encryption_request", "session", s.ID())
			return nil
		}

		uuid := pkt.PlayerUUID
		if uuid == (protocol.UUID{}) {
			uuid = protocol.OfflinePlayerUUID(pkt.Name)
		}

		return completeLogin(s, pkt.Name, uuid, nil, cfg)
	}); err != nil {
		return err
	}

	if cfg.OnlineMode {
		encResponseID := loginpacket.NewEncryptionResponse().ID()
		if err := router.Register(protocol.StateLogin, encResponseID, func(s Session, packet protocol.Packet) error {
			pkt, ok := packet.(*loginpacket.EncryptionResponse)
			if !ok {
				return protocol.ErrNilPacket
			}

			ds, ok := s.(*DefaultSession)
			if !ok {
				return fmt.Errorf("encryption_response: invalid session type")
			}

			raw, _ := ds.sessionData.Load(loginSessionKey{})
			pending, ok := raw.(*pendingLogin)
			if !ok || pending == nil {
				return fmt.Errorf("encryption_response: no pending login")
			}
			ds.sessionData.Delete(loginSessionKey{})

			sharedSecret, err := protocrypto.DecryptSharedSecret(cfg.PrivateKey, pkt.SharedSecret)
			if err != nil {
				return fmt.Errorf("encryption_response: %w", err)
			}

			decryptedToken, err := protocrypto.DecryptVerifyToken(cfg.PrivateKey, pkt.VerifyToken)
			if err != nil {
				return fmt.Errorf("encryption_response: %w", err)
			}

			if !bytes.Equal(decryptedToken, pending.verifyToken) {
				return fmt.Errorf("encryption_response: verify token mismatch")
			}

			encrypter, err := protocrypto.NewCFB8Encrypter(sharedSecret)
			if err != nil {
				return fmt.Errorf("encryption_response: %w", err)
			}
			decrypter, err := protocrypto.NewCFB8Decrypter(sharedSecret)
			if err != nil {
				return fmt.Errorf("encryption_response: %w", err)
			}

			s.EnableNetworkEncryption(encrypter, decrypter)
			logger.Debug("encryption enabled", "session", s.ID())

			serverHash := auth.ServerHash("", sharedSecret, cfg.PublicKeyDER)
			profile, err := auth.HasJoined(pending.username, serverHash)
			if err != nil {
				return fmt.Errorf("encryption_response: authentication failed: %w", err)
			}

			logger.Info("authenticated", "session", s.ID(), "name", profile.Name, "uuid", protocol.UUIDToString(profile.UUID))

			var properties []loginpacket.LoginProperty
			for _, p := range profile.Properties {
				prop := loginpacket.LoginProperty{
					Name:  p.Name,
					Value: p.Value,
				}
				if p.Signature != "" {
					prop.HasSig = true
					prop.Signature = p.Signature
				}
				properties = append(properties, prop)
			}

			return completeLogin(s, profile.Name, profile.UUID, properties, cfg)
		}); err != nil {
			return err
		}
	}

	loginAcknowledgedID := loginpacket.NewLoginAcknowledged().ID()
	if err := router.Register(protocol.StateLogin, loginAcknowledgedID, func(s Session, packet protocol.Packet) error {
		logger.Debug("login_acknowledged", "session", s.ID())
		return handleEnterConfiguration(s)
	}); err != nil {
		return err
	}

	return nil
}

// completeLogin sends SetCompression (if enabled) and LoginSuccess, then fires the callback.
func completeLogin(s Session, name string, uuid protocol.UUID, properties []loginpacket.LoginProperty, cfg LoginConfig) error {
	if ds, ok := s.(*DefaultSession); ok {
		ds.sessionData.Store("login_name", name)
		ds.sessionData.Store("login_uuid", uuid)
		ds.sessionData.Store("login_properties", properties)
	}

	if cfg.CompressionThreshold >= 0 {
		setComp := &loginpacket.SetCompression{Threshold: int32(cfg.CompressionThreshold)}
		if err := s.Send(setComp); err != nil {
			return err
		}
		s.EnableNetworkCompression(cfg.CompressionThreshold)
		logger.Debug("compression enabled", "session", s.ID(), "threshold", cfg.CompressionThreshold)
	}

	response := &loginpacket.LoginSuccess{
		UUID:       uuid,
		Name:       name,
		Properties: properties,
	}
	if err := s.Send(response); err != nil {
		return err
	}

	if cfg.OnLoginSuccess != nil {
		cfg.OnLoginSuccess(s, name, uuid)
	}

	return nil
}
