package peering

import (
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gohornet/hornet/pkg/config"
)

func configurePeerConfigWatcher() {
	config.PeeringConfig.WatchConfig()
}

func runConfigWatcher() {
	config.PeeringConfig.OnConfigChange(func(e fsnotify.Event) {
		if !config.AcquirePeeringConfigHotReload() {
			return
		}

		// whether to accept any incoming peer connection
		acceptAnyPeer := config.PeeringConfig.GetBool(config.CfgPeeringAcceptAnyConnection)
		if Manager().Opts.AcceptAnyPeer != acceptAnyPeer {
			log.Infof("set '%s' to <%v> due to config change", config.CfgPeeringAcceptAnyConnection, acceptAnyPeer)
			Manager().Opts.AcceptAnyPeer = acceptAnyPeer
		}

		modified, added, removed := getPeerConfigDiff()

		// remove peers if we do not accept connections from unknown peers
		if !acceptAnyPeer && len(removed) > 0 {
			for _, p := range removed {
				if err := Manager().Remove(p.ID); err != nil {
					log.Warnf("removing peer due to config change failed with: %v", err)
					continue
				}
				log.Infof("removed peer %s due to config change", p.ID)
			}
		}

		// modify peers
		if len(modified) > 0 {
			log.Infof("modifying peers due to config change")
			for _, p := range modified {
				// remove the peer
				if err := Manager().Remove(p.ID); err != nil {
					log.Warn(err)
				}
				// and re-add it with the updated info
				if err := Manager().Add(p.ID, p.PreferIPv6, p.Alias); err != nil {
					log.Warn("was unable to re-add modified peer %s", p.ID)
				}
			}
		}

		// add peers
		if len(added) > 0 {
			log.Infof("adding peers due to config change")
			for _, p := range added {
				if err := Manager().Add(p.ID, p.PreferIPv6, p.Alias); err != nil {
					log.Warn("was unable to re-add modified peer %s", p.ID)
				}
			}
		}
	})
}

// calculates the diffs between the loaded peers and the modified config.
func getPeerConfigDiff() (modified, added, removed []config.PeerConfig) {
	currentPeers := Manager().PeerInfos()
	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		log.Warn(err)
		return
	}

	for _, currentPeer := range currentPeers {
		if currentPeer.Autopeered {
			// ignore autopeered neighbors
			continue
		}

		found := false
		for _, configPeer := range configPeers {
			if strings.EqualFold(currentPeer.Address, configPeer.ID) || strings.EqualFold(currentPeer.DomainWithPort, configPeer.ID) {
				found = true
				if (currentPeer.PreferIPv6 != configPeer.PreferIPv6) || (currentPeer.Alias != configPeer.Alias) {
					modified = append(modified, configPeer)
				}
				break
			}
		}

		if !found {
			removed = append(removed, config.PeerConfig{ID: currentPeer.Address, PreferIPv6: currentPeer.PreferIPv6})
		}
	}

	for _, configPeer := range configPeers {

		// ignore the example peer
		if configPeer.ID == ExamplePeerURI {
			continue
		}

		found := false
		for _, currentPeer := range currentPeers {
			if currentPeer.Autopeered {
				// ignore autopeered neighbors
				found = true
				break
			}

			if strings.EqualFold(currentPeer.Address, configPeer.ID) || strings.EqualFold(currentPeer.DomainWithPort, configPeer.ID) {
				found = true
				break
			}
		}

		if !found {
			added = append(added, configPeer)
		}
	}
	return
}
