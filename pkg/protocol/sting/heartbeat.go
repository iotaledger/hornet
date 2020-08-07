package sting

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// Heartbeat contains information about a nodes current solid and pruned milestone index.
type Heartbeat struct {
	SolidMilestoneIndex  milestone.Index `json:"solid_milestone_index"`
	PrunedMilestoneIndex milestone.Index `json:"pruned_milestone_index"`
	LatestMilestoneIndex milestone.Index `json:"latest_milestone_index"`
}

/// ParseHeartbeat parses the given message into a heartbeat.
func ParseHeartbeat(data []byte) (*Heartbeat, error) {

	if len(data) < 8 {
		return nil, ErrInvalidSourceLength
	}

	solidMilestoneIndex := milestone.Index(binary.BigEndian.Uint32(data[:4]))
	prunedMilestoneIndex := milestone.Index(binary.BigEndian.Uint32(data[4:8]))

	// fallback if neighbors use the old heartbeat version without LMI
	latestMilestoneIndex := solidMilestoneIndex

	if len(data) >= 12 {
		latestMilestoneIndex = milestone.Index(binary.BigEndian.Uint32(data[8:12]))
	}

	return &Heartbeat{
		SolidMilestoneIndex:  solidMilestoneIndex,
		PrunedMilestoneIndex: prunedMilestoneIndex,
		LatestMilestoneIndex: latestMilestoneIndex,
	}, nil
}

func HeartbeatCaller(handler interface{}, params ...interface{}) {
	handler.(func(heartbeat *Heartbeat))(params[0].(*Heartbeat))
}
