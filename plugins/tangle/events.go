package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

func NewConfirmedMilestoneMetricCaller(handler interface{}, params ...interface{}) {
	handler.(func(metric *ConfirmedMilestoneMetric))(params[0].(*ConfirmedMilestoneMetric))
}

func ConfirmedMilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(confirmation *whiteflag.Confirmation))(params[0].(*whiteflag.Confirmation))
}

var Events = pluginEvents{
	ReceivedNewMessage:            events.NewEvent(tangle.NewMessageCaller),
	ReceivedKnownMessage:          events.NewEvent(tangle.MessageCaller),
	ProcessedMessage:              events.NewEvent(tangle.MessageIDCaller),
	MessageSolid:                  events.NewEvent(tangle.MessageMetadataCaller),
	MessageReferenced:             events.NewEvent(tangle.MessageReferencedCaller),
	ReceivedNewMilestone:          events.NewEvent(tangle.MilestoneCaller),
	LatestMilestoneChanged:        events.NewEvent(tangle.MilestoneCaller),
	LatestMilestoneIndexChanged:   events.NewEvent(milestone.IndexCaller),
	MilestoneConfirmed:            events.NewEvent(ConfirmedMilestoneCaller),
	SolidMilestoneChanged:         events.NewEvent(tangle.MilestoneCaller),
	SolidMilestoneIndexChanged:    events.NewEvent(milestone.IndexCaller),
	SnapshotMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
	PruningMilestoneIndexChanged:  events.NewEvent(milestone.IndexCaller),
	NewConfirmedMilestoneMetric:   events.NewEvent(NewConfirmedMilestoneMetricCaller),
	MilestoneSolidificationFailed: events.NewEvent(milestone.IndexCaller),
}

type pluginEvents struct {
	ReceivedNewMessage            *events.Event
	ReceivedKnownMessage          *events.Event
	ProcessedMessage              *events.Event
	MessageSolid                  *events.Event
	MessageReferenced             *events.Event
	ReceivedNewMilestone          *events.Event
	LatestMilestoneChanged        *events.Event
	LatestMilestoneIndexChanged   *events.Event
	MilestoneConfirmed            *events.Event
	SolidMilestoneChanged         *events.Event
	SolidMilestoneIndexChanged    *events.Event
	SnapshotMilestoneIndexChanged *events.Event
	PruningMilestoneIndexChanged  *events.Event
	NewConfirmedMilestoneMetric   *events.Event
	MilestoneSolidificationFailed *events.Event
}
