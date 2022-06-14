package tangle

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

type BPSMetrics struct {
	Incoming uint32 `json:"incoming"`
	New      uint32 `json:"new"`
	Outgoing uint32 `json:"outgoing"`
}

// ConfirmationMetricsCaller is used to signal updated confirmation metrics.
func ConfirmationMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(metrics *whiteflag.ConfirmationMetrics))(params[0].(*whiteflag.ConfirmationMetrics))
}

func NewConfirmedMilestoneMetricCaller(handler interface{}, params ...interface{}) {
	handler.(func(metric *ConfirmedMilestoneMetric))(params[0].(*ConfirmedMilestoneMetric))
}

func ConfirmedMilestoneCaller(handler interface{}, params ...interface{}) {
	handler.(func(confirmation *whiteflag.Confirmation))(params[0].(*whiteflag.Confirmation))
}

func BPSMetricsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*BPSMetrics))(params[0].(*BPSMetrics))
}

func LedgerUpdatedCaller(handler interface{}, params ...interface{}) {
	handler.(func(milestone.Index, utxo.Outputs, utxo.Spents))(params[0].(milestone.Index), params[1].(utxo.Outputs), params[2].(utxo.Spents))
}

func TreasuryMutationCaller(handler interface{}, params ...interface{}) {
	handler.(func(milestone.Index, *utxo.TreasuryMutationTuple))(params[0].(milestone.Index), params[1].(*utxo.TreasuryMutationTuple))
}

func ReceiptCaller(handler interface{}, params ...interface{}) {
	handler.(func(*iotago.ReceiptMilestoneOpt))(params[0].(*iotago.ReceiptMilestoneOpt))
}

func ReferencedBlocksCountUpdatedCaller(handler interface{}, params ...interface{}) {
	handler.(func(msIndex milestone.Index, referencedBlocksCount int))(params[0].(milestone.Index), params[1].(int))
}

type Events struct {
	BPSMetricsUpdated              *events.Event
	ReceivedNewBlock               *events.Event
	ReceivedKnownBlock             *events.Event
	ProcessedBlock                 *events.Event
	BlockSolid                     *events.Event
	BlockReferenced                *events.Event
	ReceivedNewMilestoneBlock      *events.Event
	LatestMilestoneChanged         *events.Event
	LatestMilestoneIndexChanged    *events.Event
	MilestoneConfirmed             *events.Event
	ConfirmedMilestoneChanged      *events.Event
	ConfirmedMilestoneIndexChanged *events.Event
	NewConfirmedMilestoneMetric    *events.Event
	ConfirmationMetricsUpdated     *events.Event
	MilestoneSolidificationFailed  *events.Event
	MilestoneTimeout               *events.Event
	LedgerUpdated                  *events.Event
	TreasuryMutated                *events.Event
	NewReceipt                     *events.Event
	ReferencedBlocksCountUpdated   *events.Event
}
