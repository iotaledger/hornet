package tangle

import (
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/iotaledger/hive.go/events"
)

var Events = pluginEvents{
	ReceivedNewTransaction:        events.NewEvent(tangle.NewTransactionCaller),
	ReceivedKnownTransaction:      events.NewEvent(tangle.TransactionCaller),
	TransactionSolid:              events.NewEvent(tangle.TransactionCaller),
	TransactionConfirmed:          events.NewEvent(tangle.TransactionConfirmedCaller),
	TransactionStored:             events.NewEvent(tangle.TransactionCaller),
	ReceivedNewMilestone:          events.NewEvent(tangle.BundleCaller),
	LatestMilestoneChanged:        events.NewEvent(tangle.BundleCaller),
	SolidMilestoneChanged:         events.NewEvent(tangle.BundleCaller),
	SnapshotMilestoneIndexChanged: events.NewEvent(milestone_index.MilestoneIndexCaller),
}

type pluginEvents struct {
	ReceivedNewTransaction        *events.Event
	ReceivedKnownTransaction      *events.Event
	TransactionSolid              *events.Event
	TransactionConfirmed          *events.Event
	TransactionStored             *events.Event
	ReceivedNewMilestone          *events.Event
	LatestMilestoneChanged        *events.Event
	SolidMilestoneChanged         *events.Event
	SnapshotMilestoneIndexChanged *events.Event
}
