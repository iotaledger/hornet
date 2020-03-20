package peering

import (
	"bytes"

	"github.com/gohornet/hornet/packages/peering/peer"
	"github.com/gohornet/hornet/packages/protocol"
	"github.com/gohornet/hornet/packages/protocol/handshake"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/pkg/errors"
)

func (m *Manager) setupHandshakeEventHandlers(p *peer.Peer) {
	// mark when a handshake was sent off
	p.Protocol.Events.Sent[handshake.HandshakeMessageDefinition.ID].Attach(events.NewClosure(func() {
		p.Protocol.Handshaked()
	}))

	// verify received handshake
	p.Protocol.Events.Received[handshake.HandshakeMessageDefinition.ID].Attach(events.NewClosure(func(data []byte) {
		handshakeMsg, err := handshake.ParseHandshake(data)
		if err != nil {
			p.Protocol.Events.Error.Trigger(err)
			return
		}

		if err := m.verifyHandshake(p, handshakeMsg); err != nil {
			p.Protocol.Events.Error.Trigger(err)
		}
	}))

	// propagate handshake completion to the manager
	p.Protocol.Events.HandshakeCompleted.Attach(events.NewClosure(func() {
		p.MoveBackToReconnectPool = true
		m.Events.PeerConnected.Trigger(p)
	}))
}

func (m *Manager) verifyHandshake(p *peer.Peer, handshakeMsg *handshake.Handshake) error {
	m.handshakeVerifyMu.Lock()
	defer m.handshakeVerifyMu.Unlock()

	if m.shutdown.Load() {
		return ErrManagerIsShutdown
	}

	// drop the connection if in the meantime the available peering slots were filled
	if m.Opts.AcceptAnyPeer && m.SlotsFilled() {
		return ErrPeeringSlotsFilled
	}

	// check whether same MWM is used
	if handshakeMsg.MWM != m.Opts.ValidHandshake.MWM {
		return errors.Wrapf(ErrNonMatchingMWM, "(%d instead of %d)", handshakeMsg.MWM, m.Opts.ValidHandshake.MWM)
	}

	// check whether the peer uses the same coordinator address
	if !bytes.Equal(handshakeMsg.ByteEncodedCooAddress, m.Opts.ValidHandshake.ByteEncodedCooAddress) {
		return ErrNonMatchingCooAddr
	}

	// check feature set compatibility
	version, err := handshakeMsg.SupportedVersion(protocol.SupportedFeatureSets)
	if err != nil {
		return errors.Wrapf(err, "protocol version %d is not supported", version)
	}

	switch p.ConnectionOrigin {
	case peer.Inbound:
		// set the inbound peer's ID given that we now have the server socket port number
		p.ID = peer.NewID(p.PrimaryAddress.String(), handshakeMsg.ServerSocketPort)
		p.InitAddress = &iputils.OriginAddress{
			Addr: p.PrimaryAddress.String(),
			Port: handshakeMsg.ServerSocketPort,
		}

		// init autopeering info if this peer was previously whitelisted
		if autopeeringInfo, ok := m.Whitelisted(p.ID); ok && autopeeringInfo != nil {
			p.Autopeering = autopeeringInfo
			m.Events.ConnectedAutopeeredPeer.Trigger(p)
		}
	case peer.Outbound:
		expectedPort := p.InitAddress.Port
		if handshakeMsg.ServerSocketPort != expectedPort {
			return errors.Wrapf(ErrNonMatchingSrvSocketPort, "expected %d as the server socket port but got %d", expectedPort, handshakeMsg.ServerSocketPort)
		}
	}

	// check whether the peer is already connected by checking each peer's IP addresses
	m.Lock()
	for _, connectedPeer := range m.connected {
		// skip self: we must check this now as we no longer have a concept about in-flight connections
		if connectedPeer == p {
			continue
		}
		// we need to loop through because the map holds pointer values
		for handshakingPeerIP := range p.Addresses.IPs {
			for ip := range connectedPeer.Addresses.IPs {
				if ip.String() == handshakingPeerIP.String() &&
					connectedPeer.InitAddress.Port == p.InitAddress.Port {
					m.Unlock()
					return errors.Wrapf(ErrPeerAlreadyConnected, p.ID)
				}
			}
		}
	}

	// check whether the peer is whitelisted
	if !m.Opts.AcceptAnyPeer {
		if _, whitelisted := m.Whitelisted(p.ID); !whitelisted {
			m.Unlock()
			m.Blacklist(p.PrimaryAddress.String())
			return errors.Wrapf(ErrUnknownPeerID, p.ID)
		}
	}

	// mark inbound peer now as connected
	if p.IsInbound() {
		m.moveToConnected(p)
	}

	m.Unlock()

	p.Protocol.FeatureSet = byte(version)
	p.Protocol.Handshaked()
	return nil
}
