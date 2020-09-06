package framework

import (
	"context"
	"fmt"
	"log"
	"time"
)

// connected is a bool whether nodes are connected or not.
type connected bool

// StaticPeeringLayout defines how in a statically peered
// network nodes are peered to each other.
type StaticPeeringLayout map[int]map[int]connected

// Validate validates whether the static peering layout is valid by checking:
//	- the layout isn't empty
//	- keys must be continuous numbers reflecting the ID of the node
//	- a node must hold nodes to peer to and they must exist in the map
//	- a node doesn't peer to itself
func (spl StaticPeeringLayout) Validate() error {
	if len(spl) == 0 {
		return ErrLayoutEmpty
	}

	// we i-loop over the map to verify that the keys are indeed continuous numbers
	for i := 0; i < len(spl); i++ {
		peers, has := spl[i]
		if !has {
			return fmt.Errorf("%w: %d", ErrNodeMissingInLayout, i)
		}
		if len(peers) == 0 {
			return fmt.Errorf("%w: %d", ErrNoStaticPeers, i)
		}
		for peerID := range peers {
			if peerID == i {
				return fmt.Errorf("%w: id %d", ErrSelfPeering, i)
			}
			if _, has := spl[peerID]; !has {
				return fmt.Errorf("%w: %d can't peer to undefined peer %d", ErrNodeMissingInLayout, i, peerID)
			}
		}
	}

	return nil
}

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := n.AwaitOnline(ctx); err != nil {
		return err
	}

	// manually connect peers to each other according to the layout
	log.Printf("statically peering %d nodes", len(n.layout))
	s := time.Now()
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
			log.Printf("connected %s with %s", node.IP, peer.IP)
		}
	}

	peeringCtx, peeringCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer peeringCtxCancel()
	if err := n.AwaitPeering(peeringCtx); err != nil {
		return err
	}
	log.Printf("static peering took %v", time.Since(s))
	return nil
}

// AwaitPeering awaits until all nodes are peered according to the peering layout.
func (n *StaticNetwork) AwaitPeering(ctx context.Context) error {
	log.Println("verifying peering...")
	for nodeID, layoutNeighbors := range n.layout {
		node := n.Nodes[nodeID]
		for {
			if err := returnErrIfCtxDone(ctx, ErrNodesDidNotPeerInTime); err != nil {
				return err
			}

			neighbors, err := node.DebugWebAPI.Neighbors()
			if err != nil {
				continue
			}

			var peered int
			for layoutNeighbor := range layoutNeighbors {
				layoutNode := n.Nodes[layoutNeighbor]
				for _, neighbor := range neighbors {
					if neighbor.Address == fmt.Sprintf("%s:15600", layoutNode.IP) {
						peered++
					}
				}
			}

			if peered == len(layoutNeighbors) {
				break
			}
		}
	}
	return nil
}
