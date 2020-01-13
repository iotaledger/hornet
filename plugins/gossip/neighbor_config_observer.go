package gossip

import (
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/iotaledger/hive.go/iputils"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/parameter"
)

func configureConfigObserver() {
	parameter.NeighborsConfig.WatchConfig()
}

func runConfigObserver() {
	parameter.NeighborsConfig.OnConfigChange(func(e fsnotify.Event) {
		// auto tethering
		autoTetheringEnabledRead := parameter.NeighborsConfig.GetBool("autoTetheringEnabled")
		if autoTetheringEnabled != autoTetheringEnabledRead {
			gossipLogger.Infof("Set autoTetheringEnabled to <%v> due to config change", autoTetheringEnabledRead)
			autoTetheringEnabled = autoTetheringEnabledRead
		}

		modified, added, removed := getNeighborConfigDiff()

		// Modify neighbors
		if len(modified) > 0 {
			for _, nb := range modified {
				if err := RemoveNeighbor(nb.Identity); err != nil {
					gossipLogger.Error(err)
				}
			}
			if err := addNewNeighbors(modified); err != nil {
				gossipLogger.Error(err)
			} else {
				gossipLogger.Infof("Modify neighbors due to config change was successful")
			}
		}

		// Add neighbors
		if len(added) > 0 {
			if err := addNewNeighbors(added); err != nil {
				gossipLogger.Error(err)
			} else {
				gossipLogger.Infof("Add neighbors due to config change was successful")
			}
		}

		// Remove Neighbors
		if len(removed) > 0 {
			for _, nb := range removed {
				if err := RemoveNeighbor(nb.Identity); err != nil {
					gossipLogger.Errorf("Remove neighbor due to config change failed with: %v", err)
				} else {
					gossipLogger.Infof("Remove neighbor (%s) due to config change was successful", nb.Identity)
				}
			}
		}
	})
}

// calculates the differences between the loaded neighbors and the modified config
func getNeighborConfigDiff() (modified, added, removed []NeighborConfig) {
	boundNeighbors := GetNeighbors()
	configNeighbors := []NeighborConfig{}
	if err := parameter.NeighborsConfig.UnmarshalKey("neighbors", &configNeighbors); err != nil {
		gossipLogger.Error(err)
	}

	for _, boundNeighbor := range boundNeighbors {
		found := false
		for _, configNeighbor := range configNeighbors {
			if strings.EqualFold(boundNeighbor.Address, configNeighbor.Identity) || strings.EqualFold(boundNeighbor.DomainWithPort, configNeighbor.Identity) {
				found = true
				if boundNeighbor.PreferIPv6 != configNeighbor.PreferIPv6 {
					modified = append(modified, configNeighbor)
				}
			}
		}
		if !found {
			removed = append(removed, NeighborConfig{Identity: boundNeighbor.Address, PreferIPv6: boundNeighbor.PreferIPv6})
		}
	}

	for _, configNeighbor := range configNeighbors {
		found := false
		for _, boundNeighbor := range boundNeighbors {
			if strings.EqualFold(boundNeighbor.Address, configNeighbor.Identity) || strings.EqualFold(boundNeighbor.DomainWithPort, configNeighbor.Identity) {
				found = true
			}
		}
		if !found {
			added = append(added, configNeighbor)
		}
	}
	return
}

func addNewNeighbors(neighbors []NeighborConfig) error {
	neighborsLock.Lock()
	defer neighborsLock.Unlock()
	for _, nb := range neighbors {
		if nb.Identity == "" {
			continue
		}

		// check whether already in reconnect pool
		if _, exists := reconnectPool[nb.Identity]; exists {
			return errors.Wrapf(ErrNeighborAlreadyKnown, "%s is already known and in the reconnect pool", nb.Identity)
		}

		originAddr, err := iputils.ParseOriginAddress(nb.Identity)
		if err != nil {
			return errors.Wrapf(err, "invalid neighbor address %s", nb.Identity)
		}
		originAddr.PreferIPv6 = nb.PreferIPv6

		addNeighborToReconnectPool(&reconnectneighbor{OriginAddr: originAddr})
	}
	wakeupReconnectPool()
	return nil
}
