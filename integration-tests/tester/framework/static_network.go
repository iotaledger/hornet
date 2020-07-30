package framework

import (
	"context"
	"fmt"
	"time"
)

// connected is a bool whether nodes are connected or not.
type connected bool

// StaticPeeringLayout defines how in a statically peered
// network nodes are peered to each other.
type StaticPeeringLayout map[int]map[int]connected

// DefaultStaticPeeringLayout defines a static peering layout with 4 nodes
// which are all statically peered to each other.
var DefaultStaticPeeringLayout StaticPeeringLayout = map[int]map[int]connected{
	0: {1: false, 2: false, 3: false},
	1: {0: false, 2: false, 3: false},
	2: {0: false, 1: false, 3: false},
	3: {0: false, 1: false, 2: false},
}

// StaticNetwork defines a network made out of statically peered nodes.
type StaticNetwork struct {
	*Network
	layout StaticPeeringLayout
}

// ConnectNodes peers the nodes of the network according to the given layout with each other.
func (n *StaticNetwork) ConnectNodes() error {

	// await for all nodes to become online
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := n.AwaitOnline(ctx); err != nil {
		return err
	}

	// cool down period
	time.Sleep(2 * time.Second)

	// manually connect peers to each other according to the layout
	for i := 0; i < len(n.layout); i++ {
		peers := n.layout[i]
		node := n.Nodes[i]
		for peerIndex := range peers {
			peer := n.Nodes[peerIndex]
			if alreadyPeered := n.layout[peerIndex][i]; alreadyPeered {
				continue
			}
			uri := fmt.Sprintf("tcp://%s:15600", peer.IP)
			n.layout[i][peerIndex] = true
			n.layout[peerIndex][i] = true
			if _, err := node.WebAPI.AddNeighbors(uri); err != nil {
				return fmt.Errorf("%w: couldn't add peer %v", err, uri)
			}
			time.Sleep(1* time.Millisecond)
		}
	}
	return nil
}

// AwaitPeering awaits until all nodes are peered according to the peering layout.
func (n *StaticNetwork) AwaitPeering(ctx context.Context) error {
	for nodeID, itsPeers := range n.layout {
		for {
			select {
			case <-ctx.Done():
				return ErrNodesDidNotPeerInTime
			default:
			}
			node := n.Nodes[nodeID]
			neighbors, err := node.DebugWebAPI.Neighbors()
			if err != nil {
				time.Sleep(250 * time.Millisecond)
				continue
			}

			// we don't check whether the node is actually peered
			// to what was layed out but since there is no autopeering
			// the neighbors must match what was defined in the layout.
			if len(neighbors) == len(itsPeers) {
				break
			}
		}
	}
	return nil
}
