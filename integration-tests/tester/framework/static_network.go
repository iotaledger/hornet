package framework

import (
	"context"
	"time"
)

// StaticPeeringLayout defines how in a statically peered
// network nodes are peered to each other.
type StaticPeeringLayout map[int]map[int]bool

// DefaultStaticPeeringLayout defines a static peering layout with 4 nodes
// which are all statically peered to each other.
var DefaultStaticPeeringLayout StaticPeeringLayout = map[int]map[int]bool{
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
