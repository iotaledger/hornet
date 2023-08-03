package webapi

import (
	"github.com/iotaledger/hornet/pkg/model/milestone"

	"github.com/iotaledger/iota.go/trinary"
)

// infoResponse defines the response of a GET info REST API call.
type infoResponse struct {
	AppName                            string          `json:"appName"`
	AppVersion                         string          `json:"appVersion"`
	LatestMilestone                    trinary.Hash    `json:"latestMilestone"`
	LatestMilestoneIndex               milestone.Index `json:"latestMilestoneIndex"`
	LatestSolidSubtangleMilestone      trinary.Hash    `json:"latestSolidSubtangleMilestone"`
	LatestSolidSubtangleMilestoneIndex milestone.Index `json:"latestSolidSubtangleMilestoneIndex"`
	IsSynced                           bool            `json:"isSynced"`
	Health                             bool            `json:"isHealthy"`
	MilestoneStartIndex                milestone.Index `json:"milestoneStartIndex"`
	LastSnapshottedMilestoneIndex      milestone.Index `json:"lastSnapshottedMilestoneIndex"`
	Neighbors                          uint            `json:"neighbors"`
	Time                               int64           `json:"time"`
	Tips                               uint32          `json:"tips"`
	TransactionsToRequest              int             `json:"transactionsToRequest"`
	Features                           []string        `json:"features"`
	CoordinatorAddress                 trinary.Hash    `json:"coordinatorAddress"`
}

// milestoneResponse struct.
type milestoneResponse struct {
	MilestoneIndex     milestone.Index `json:"milestoneIndex"`
	MilestoneHash      trinary.Hash    `json:"milestoneHash"`
	MilestoneTimestamp uint64          `json:"milestoneTimestamp"` // The milestone timestamp this transaction was referenced.
}

// transactionsResponse struct.
type transactionsResponse struct {
	Bundle            trinary.Hash    `json:"bundle,omitempty"`
	Address           trinary.Hash    `json:"address,omitempty"`
	Tag               trinary.Hash    `json:"tag,omitempty"`
	Approvee          trinary.Hash    `json:"approvee,omitempty"`
	TransactionHashes []trinary.Hash  `json:"txHashes"`
	LedgerIndex       milestone.Index `json:"ledgerIndex"`
}

// transactionTrytesResponse struct.
type transactionTrytesResponse struct {
	TxHash trinary.Hash   `json:"txHash"`
	Trytes trinary.Trytes `json:"trytes"`
}

// transactionMetadataResponse struct.
type transactionMetadataResponse struct {
	TxHash                       trinary.Hash    `json:"txHash"`
	Solid                        bool            `json:"isSolid"`
	Included                     bool            `json:"included"`
	Confirmed                    bool            `json:"confirmed"`
	Conflicting                  bool            `json:"conflicting"`
	ReferencedByMilestoneIndex   milestone.Index `json:"referencedByMilestoneIndex,omitempty"` // The milestone index that references this transaction.
	MilestoneTimestampReferenced uint64          `json:"milestoneTimestampReferenced"`         // The milestone timestamp this transaction was referenced.
	MilestoneIndex               milestone.Index `json:"milestoneIndex,omitempty"`             // If this transaction represents a milestone this is the milestone index.
	LedgerIndex                  milestone.Index `json:"ledgerIndex"`
}

// addressWasSpentResponse struct.
type addressWasSpentResponse struct {
	Address     trinary.Hash    `json:"address"`
	WasSpent    bool            `json:"wasSpent"`
	LedgerIndex milestone.Index `json:"ledgerIndex"`
}

// balanceResponse struct.
type balanceResponse struct {
	Address     trinary.Hash    `json:"address"`
	Balance     string          `json:"balance"`
	LedgerIndex milestone.Index `json:"ledgerIndex"`
}

// ledgerStateResponse struct.
type ledgerStateResponse struct {
	Balances    map[trinary.Hash]string `json:"balances"`
	LedgerIndex milestone.Index         `json:"ledgerIndex"`
	// Checksum is the SHA256 checksum of the ledger state for this MilestoneIndex.
	Checksum string `json:"checksum"`
}

// ledgerDiffResponse struct.
type ledgerDiffResponse struct {
	AddressDiffs map[trinary.Hash]string `json:"addressDiffs"`
	LedgerIndex  milestone.Index         `json:"ledgerIndex"`
}

// txHashWithValue struct.
type txHashWithValue struct {
	TxHash     trinary.Hash `json:"txHash"`
	TailTxHash trinary.Hash `json:"tailTxHash"`
	Bundle     trinary.Hash `json:"bundle"`
	Address    trinary.Hash `json:"address"`
	Value      string       `json:"value"`
}

func (tx *txHashWithValue) Item() Container {
	return tx
}

// txWithValue struct.
type txWithValue struct {
	TxHash  trinary.Hash `json:"txHash"`
	Address trinary.Hash `json:"address"`
	Index   uint32       `json:"index"`
	Value   string       `json:"value"`
}

func (tx *txWithValue) Item() Container {
	return tx
}

// bundleWithValue struct.
type bundleWithValue struct {
	Bundle     trinary.Hash   `json:"bundle"`
	TailTxHash trinary.Hash   `json:"tailTxHash"`
	LastIndex  uint32         `json:"lastIndex"`
	Txs        []*txWithValue `json:"transactions"`
}

func (b *bundleWithValue) Item() Container {
	return b
}

// ledgerDiffExtendedResponse struct.
type ledgerDiffExtendedResponse struct {
	ConfirmedTxWithValue      []*txHashWithValue      `json:"confirmedTransactionsWithValue"`
	ConfirmedBundlesWithValue []*bundleWithValue      `json:"confirmedBundlesWithValue"`
	AddressDiffs              map[trinary.Hash]string `json:"addressDiffs"`
	LedgerIndex               milestone.Index         `json:"ledgerIndex"`
}
