package tangle

import (
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

type MPSMetrics struct {
	Incoming uint32 `json:"incoming"`
	New      uint32 `json:"new"`
	Outgoing uint32 `json:"outgoing"`
}

func NewConfirmedMilestoneMetricCaller(handler interface{}, params ...interface{}) {
	handler.(func(metric *ConfirmedMilestoneMetric))(params[0].(*ConfirmedMilestoneMetric))
}

func ConfirmedMilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(confirmation *whiteflag.Confirmation))(params[0].(*whiteflag.Confirmation))
}

func MPSMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*MPSMetrics))(params[0].(*MPSMetrics))
}

func UTXOOutputCaller(handler interface{}, params ...interface{}) {
	handler.(func(*utxo.Output))(params[0].(*utxo.Output))
}

func UTXOSpentCaller(handler interface{}, params ...interface{}) {
	handler.(func(*utxo.Spent))(params[0].(*utxo.Spent))
}

func ReceiptCaller(handler interface{}, params ...interface{}) {
	handler.(func(*iotago.Receipt))(params[0].(*iotago.Receipt))
}

type pluginEvents struct {
	MPSMetricsUpdated             *events.Event
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
	NewUTXOOutput                 *events.Event
	NewUTXOSpent                  *events.Event
	NewReceipt                    *events.Event
}
