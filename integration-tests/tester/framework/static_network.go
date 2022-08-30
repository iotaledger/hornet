package framework

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Connected is a bool whether nodes are Connected or not.
type Connected bool

// StaticPeeringLayout defines how in a statically peered
// network nodes are peered to each other.
type StaticPeeringLayout map[int]map[int]Connected

// Validate validates whether the static peering layout is valid by checking:
//   - the layout isn't empty
//   - keys must be continuous numbers reflecting the ID of the node
//   - a node must hold nodes to peer to and they must exist in the map
//   - a node doesn't peer to itself
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

// DefaultStaticPeeringLayout returns a new static peering layout with 4 nodes
// which are all statically peered to each other.
func DefaultStaticPeeringLayout() map[int]map[int]Connected {
	return map[int]map[int]Connected{
		0: {1: false, 2: false, 3: false},
		1: {0: false, 2: false, 3: false},
		2: {0: false, 1: false, 3: false},
		3: {0: false, 1: false, 2: false},
	}
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
			multiAddress := fmt.Sprintf("/ip4/%s/tcp/15600/p2p/%s", peer.IP, peer.ID.String())
			n.layout[i][peerIndex] = true
			n.layout[peerIndex][i] = true
			if _, err := node.DebugNodeAPIClient.AddPeer(context.Background(), multiAddress); err != nil {
				return fmt.Errorf("%w: couldn't add peer %v", err, multiAddress)
			}
			log.Printf("connected %s (%s) with %s (%s)", node.IP, node.ID.String(), peer.IP, peer.ID.String())
		}
	}

	peeringCtx, peeringCtxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer peeringCtxCancel()

	if err := n.AwaitPeering(peeringCtx); err != nil {
		return err
	}
	log.Printf("static peering took %v", time.Since(s).Truncate(time.Millisecond))

	return nil
}

// AwaitPeering awaits until all nodes are peered according to the peering layout.
func (n *StaticNetwork) AwaitPeering(ctx context.Context) error {
	log.Println("verifying peering ...")
	for nodeID, layoutPeers := range n.layout {
		node := n.Nodes[nodeID]
		for {
			if err := returnErrIfCtxDone(ctx, ErrNodesDidNotPeerInTime); err != nil {
				return err
			}

			if len(n.Nodes) < len(n.layout) {
				return fmt.Errorf("not enough nodes: %d", len(n.Nodes))
			}

			ctxPeers, ctxPeersCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer ctxPeersCancel()

			peers, err := node.DebugNodeAPIClient.Peers(ctxPeers)
			if err != nil {
				log.Printf("node %s, peering: %s\n", node.ID.String(), err)
				time.Sleep(500 * time.Millisecond)

				continue
			}

			if len(peers) < len(layoutPeers) {
				time.Sleep(500 * time.Millisecond)

				continue
			}

			var peered int

		layoutLoop:
			for layoutNeighbor := range layoutPeers {
				layoutNode := n.Nodes[layoutNeighbor]
				for _, peer := range peers {
					if peer.ID == layoutNode.ID.String() && peer.Connected {
						peered++

						continue layoutLoop
					}
				}
			}

			if peered == len(layoutPeers) {
				break
			}
		}
	}

	return nil
}
