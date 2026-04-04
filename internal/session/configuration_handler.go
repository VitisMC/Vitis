package session

import (
	"github.com/vitismc/vitis/internal/logger"
	"github.com/vitismc/vitis/internal/protocol"
	cfgpacket "github.com/vitismc/vitis/internal/protocol/packets/configuration"
	"github.com/vitismc/vitis/internal/registry"
)

// RegisterConfigurationHandlers registers handlers for Configuration-state packets on a packet router.
func RegisterConfigurationHandlers(router PacketRouter, regMgr *registry.Manager, onFinishConfiguration func(session Session)) error {
	if router == nil {
		return ErrNilRegistry
	}

	clientInfoID := cfgpacket.NewClientInformation().ID()
	if err := router.Register(protocol.StateConfiguration, clientInfoID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*cfgpacket.ClientInformation)
		if !ok {
			return protocol.ErrNilPacket
		}
		logger.Debug("client_information", "session", s.ID(), "locale", pkt.Locale, "view_distance", pkt.ViewDistance)
		return nil
	}); err != nil {
		return err
	}

	sbKnownPacksID := cfgpacket.NewServerboundKnownPacks().ID()
	if err := router.Register(protocol.StateConfiguration, sbKnownPacksID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*cfgpacket.ServerboundKnownPacks)
		if !ok {
			return protocol.ErrNilPacket
		}
		clientKnowsVanilla := false
		for _, p := range pkt.Packs {
			if p.Namespace == "minecraft" && p.ID == "core" && p.Version == "1.21.4" {
				clientKnowsVanilla = true
				break
			}
		}
		logger.Debug("serverbound_known_packs", "session", s.ID(), "count", len(pkt.Packs), "vanilla", clientKnowsVanilla)
		return handleKnownPacksResponse(s, regMgr, clientKnowsVanilla)
	}); err != nil {
		return err
	}

	sbPluginMsgID := cfgpacket.NewServerboundPluginMessage().ID()
	if err := router.Register(protocol.StateConfiguration, sbPluginMsgID, func(s Session, packet protocol.Packet) error {
		pkt, ok := packet.(*cfgpacket.ServerboundPluginMessage)
		if !ok {
			return protocol.ErrNilPacket
		}
		logger.Debug("serverbound_plugin_message", "session", s.ID(), "channel", pkt.Channel, "len", len(pkt.Data))
		return nil
	}); err != nil {
		return err
	}

	ackFinishID := cfgpacket.NewAcknowledgeFinishConfiguration().ID()
	if err := router.Register(protocol.StateConfiguration, ackFinishID, func(s Session, packet protocol.Packet) error {
		logger.Debug("acknowledge_finish_configuration", "session", s.ID())
		if onFinishConfiguration != nil {
			onFinishConfiguration(s)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// handleEnterConfiguration sends the initial configuration packets after login acknowledged.
func handleEnterConfiguration(s Session) error {
	brandData := encodeBrandString("Vitis")
	brandPkt := &cfgpacket.ClientboundPluginMessage{
		Channel: "minecraft:brand",
		Data:    brandData,
	}
	if err := s.Send(brandPkt); err != nil {
		return err
	}

	knownPacks := &cfgpacket.ClientboundKnownPacks{
		Packs: []cfgpacket.KnownPack{
			{Namespace: "minecraft", ID: "core", Version: "1.21.4"},
		},
	}
	if err := s.Send(knownPacks); err != nil {
		return err
	}
	logger.Debug("sent clientbound_known_packs", "session", s.ID())
	return nil
}

func encodeBrandString(brand string) []byte {
	data := make([]byte, 0, len(brand)+1)
	uv := uint32(len(brand))
	for uv >= 0x80 {
		data = append(data, byte(uv&0x7F)|0x80)
		uv >>= 7
	}
	data = append(data, byte(uv))
	data = append(data, brand...)
	return data
}

// handleKnownPacksResponse sends registry data and finish configuration after receiving known packs.
func handleKnownPacksResponse(s Session, regMgr *registry.Manager, clientKnowsVanilla bool) error {
	packets := regMgr.BuildRegistryDataPackets(clientKnowsVanilla)
	for _, pkt := range packets {
		if err := s.Send(pkt); err != nil {
			return err
		}
	}
	logger.Debug("sent registry_data", "session", s.ID(), "packets", len(packets), "known_vanilla", clientKnowsVanilla)

	updateTags := regMgr.BuildUpdateTags()
	if err := s.Send(updateTags); err != nil {
		return err
	}
	logger.Debug("sent update_tags", "session", s.ID())

	finishCfg := &cfgpacket.FinishConfiguration{}
	if err := s.Send(finishCfg); err != nil {
		return err
	}
	logger.Debug("sent finish_configuration", "session", s.ID())
	return nil
}
