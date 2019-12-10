package tangle

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

var Events = pluginEvents{
	ReceivedNewTransaction:        events.NewEvent(hornet.NewTransactionCaller),
	ReceivedKnownTransaction:      events.NewEvent(hornet.TransactionCaller),
	TransactionSolid:              events.NewEvent(hornet.TransactionCaller),
	TransactionConfirmed:          events.NewEvent(hornet.TransactionConfirmedCaller),
	TransactionStored:             events.NewEvent(hornet.TransactionCaller),
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
