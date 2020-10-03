package helpers

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/sting"
)

// SendMessage sends a message msg to the given peer.
func SendMessage(p *peer.Peer, msgData []byte) {
	messageMsg, _ := sting.NewMessageMsg(msgData)
	p.EnqueueForSending(messageMsg)
}

// SendHeartbeat sends a heartbeat message to the given peer.
func SendHeartbeat(p *peer.Peer, solidMsIndex milestone.Index, pruningMsIndex milestone.Index, latestMsIndex milestone.Index, connectedNeighbors uint8, syncedNeighbors uint8) {
	heartbeatData, _ := sting.NewHeartbeatMsg(solidMsIndex, pruningMsIndex, latestMsIndex, connectedNeighbors, syncedNeighbors)
	p.EnqueueForSending(heartbeatData)
}

// SendMessageRequest sends a message request message to the given peer.
func SendMessageRequest(p *peer.Peer, requestedMessageID *hornet.MessageID) {
	txReqData, _ := sting.NewMessageRequestMsg(requestedMessageID)
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
