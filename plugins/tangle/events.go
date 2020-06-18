package tangle

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/iotaledger/hive.go/events"
)

func NewConfirmedMilestoneMetricCaller(handler interface{}, params ...interface{}) {
	handler.(func(metric *ConfirmedMilestoneMetric))(params[0].(*ConfirmedMilestoneMetric))
}

var Events = pluginEvents{
	ReceivedNewTransaction:        events.NewEvent(tangle.NewTransactionCaller),
	ReceivedKnownTransaction:      events.NewEvent(tangle.TransactionCaller),
	ProcessedTransaction:          events.NewEvent(tangle.TransactionHashCaller),
	TransactionSolid:              events.NewEvent(tangle.TransactionCaller),
	TransactionConfirmed:          events.NewEvent(tangle.TransactionConfirmedCaller),
	TransactionStored:             events.NewEvent(tangle.TransactionCaller),
	ReceivedNewMilestone:          events.NewEvent(tangle.BundleCaller),
	LatestMilestoneChanged:        events.NewEvent(tangle.BundleCaller),
	SolidMilestoneChanged:         events.NewEvent(tangle.BundleCaller),
	SnapshotMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
	PruningMilestoneIndexChanged:  events.NewEvent(milestone.IndexCaller),
	NewConfirmedMilestoneMetric:   events.NewEvent(NewConfirmedMilestoneMetricCaller),
}

type pluginEvents struct {
	ReceivedNewTransaction        *events.Event
	ReceivedKnownTransaction      *events.Event
	ProcessedTransaction          *events.Event
	TransactionSolid              *events.Event
	TransactionConfirmed          *events.Event
	TransactionStored             *events.Event
	ReceivedNewMilestone          *events.Event
	LatestMilestoneChanged        *events.Event
	SolidMilestoneChanged         *events.Event
	SnapshotMilestoneIndexChanged *events.Event
	PruningMilestoneIndexChanged  *events.Event
	NewConfirmedMilestoneMetric   *events.Event
}
