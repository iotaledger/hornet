package helpers

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

// SendTransaction sends a transaction message to the given peer.
func SendTransaction(p *peer.Peer, txData []byte) {
	transactionMsg, _ := sting.NewMessageMsg(txData)
	p.EnqueueForSending(transactionMsg)
}

// SendHeartbeat sends a heartbeat message to the given peer.
func SendHeartbeat(p *peer.Peer, solidMsIndex milestone.Index, pruningMsIndex milestone.Index, latestMsIndex milestone.Index, connectedNeighbors uint8, syncedNeighbors uint8) {
	heartbeatData, _ := sting.NewHeartbeatMsg(solidMsIndex, pruningMsIndex, latestMsIndex, connectedNeighbors, syncedNeighbors)
	p.EnqueueForSending(heartbeatData)
}

// SendTransactionRequest sends a transaction request message to the given peer.
func SendTransactionRequest(p *peer.Peer, requestedHash hornet.Hash) {
	txReqData, _ := sting.NewMessageRequestMsg(requestedHash)
	p.EnqueueForSending(txReqData)
}

// SendMilestoneRequest sends a milestone request to the given peer.
func SendMilestoneRequest(p *peer.Peer, index milestone.Index) {
	milestoneRequestData, _ := sting.NewMilestoneRequestMsg(index)
	p.EnqueueForSending(milestoneRequestData)
}

// SendLatestMilestoneRequest sends a milestone request which requests the latest known milestone from the given peer.
func SendLatestMilestoneRequest(p *peer.Peer) {
	SendMilestoneRequest(p, sting.LatestMilestoneRequestIndex)
}
