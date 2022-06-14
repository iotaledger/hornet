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
	// block events
	ReceivedNewBlock          *events.Event
	BlockSolid                *events.Event
	ReceivedNewMilestoneBlock *events.Event // remove with dashboard removal PR

	// milestone events
	LatestMilestoneChanged        *events.Event
	LatestMilestoneIndexChanged   *events.Event
	MilestoneSolidificationFailed *events.Event
	MilestoneTimeout              *events.Event

	// metrics
	BPSMetricsUpdated *events.Event

	// Events related to milestone confirmation

	// Hint: Ledger is write locked
	ConfirmedMilestoneIndexChanged *events.Event
	// Hint: Ledger is not locked
	NewConfirmedMilestoneMetric *events.Event // used for dashboard metrics
	// Hint: Ledger is not locked
	MilestoneConfirmed *events.Event // used for dashboard metrics
	// Hint: Ledger is not locked
	ConfirmationMetricsUpdated *events.Event // used for prometheus metrics
	// Hint: Ledger is not locked
	ConfirmedMilestoneChanged *events.Event
	// Hint: Ledger is not locked
	BlockReferenced *events.Event
	// Hint: Ledger is not locked
	ReferencedBlocksCountUpdated *events.Event
	// Hint: Ledger is not locked
	LedgerUpdated *events.Event
	// Hint: Ledger is not locked
	TreasuryMutated *events.Event
	// Hint: Ledger is not locked
	NewReceipt *events.Event
}
