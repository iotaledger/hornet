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
	ReceivedNewTransaction:        events.NewEvent(tangle.NewTransactionCaller),
	ReceivedKnownTransaction:      events.NewEvent(tangle.TransactionCaller),
	ProcessedTransaction:          events.NewEvent(tangle.TransactionHashCaller),
	TransactionSolid:              events.NewEvent(tangle.TransactionHashCaller),
	TransactionConfirmed:          events.NewEvent(tangle.TransactionConfirmedCaller),
	TransactionStored:             events.NewEvent(tangle.TransactionCaller),
	BundleSolid:                   events.NewEvent(tangle.BundleCaller),
	ReceivedNewMilestone:          events.NewEvent(tangle.BundleCaller),
	LatestMilestoneChanged:        events.NewEvent(tangle.BundleCaller),
	LatestMilestoneIndexChanged:   events.NewEvent(milestone.IndexCaller),
	MilestoneConfirmed:            events.NewEvent(ConfirmedMilestoneCaller),
	SolidMilestoneChanged:         events.NewEvent(tangle.BundleCaller),
	SolidMilestoneIndexChanged:    events.NewEvent(milestone.IndexCaller),
	SnapshotMilestoneIndexChanged: events.NewEvent(milestone.IndexCaller),
	PruningMilestoneIndexChanged:  events.NewEvent(milestone.IndexCaller),
	NewConfirmedMilestoneMetric:   events.NewEvent(NewConfirmedMilestoneMetricCaller),
	MilestoneSolidificationFailed: events.NewEvent(milestone.IndexCaller),
}

type pluginEvents struct {
	ReceivedNewTransaction        *events.Event
	ReceivedKnownTransaction      *events.Event
	ProcessedTransaction          *events.Event
	TransactionSolid              *events.Event
	TransactionConfirmed          *events.Event
	TransactionStored             *events.Event
	BundleSolid                   *events.Event
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
