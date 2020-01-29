package mqtt

import (
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

var (
	prevSMI milestone_index.MilestoneIndex = 0
	prevLMI milestone_index.MilestoneIndex = 0
)

func onNewTx(tx *tangle.CachedTransaction) {

	tx.Retain() //+1
	iotaTx := tx.GetTransaction().Tx
	tx.Release()

	// tx topic
	err := publishTx(iotaTx)
	if err != nil {
		log.Error(err.Error())
	}

	// trytes topic
	err = publishTxTrytes(iotaTx)
	if err != nil {
		log.Error(err.Error())
	}
}

func onConfirmedTx(tx *tangle.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {

	tx.Retain() //+1
	iotaTx := tx.GetTransaction().Tx
	tx.Release()

	err := publishConfTx(iotaTx, msIndex)
	if err != nil {
		log.Error(err.Error())
	}
}

func onNewLatestMilestone(bundle *tangle.Bundle) {
	err := publishLMI(bundle.GetMilestoneIndex())
	if err != nil {
		log.Error(err.Error())
	}
	err = publishLMHS(bundle.GetMilestoneHash())
	if err != nil {
		log.Error(err.Error())
	}
}

func onNewSolidMilestone(bundle *tangle.Bundle) {
	err := publishLMSI(bundle.GetMilestoneIndex())
	if err != nil {
		log.Error(err.Error())
	}
}

// Publish latest milestone index
func publishLMI(lmi milestone_index.MilestoneIndex) error {

	err := mqttBroker.Send(topicLMI, fmt.Sprintf(`{"prevLMI":"%d","lmi":%d,"timestamp":"%s"}`,
		prevLMI, // Index of the previous solid subtangle milestone
		lmi,     // Index of the latest solid subtangle milestone
		time.Now().UTC().Format(time.RFC3339)))

	// Update previous milestone index
	prevLMI = lmi

	return err
}

// Publish latest solid subtangle milestone index
func publishLMSI(smi milestone_index.MilestoneIndex) error {

	err := mqttBroker.Send(topicLMSI, fmt.Sprintf(`{"prevSMI":"%d","smi":%d,"timestamp":"%s"}`,
		prevSMI, // Index of the previous solid subtangle milestone
		smi,     // Index of the latest solid subtangle milestone
		time.Now().UTC().Format(time.RFC3339)))

	// Update previous milestone index
	prevSMI = smi

	return err
}

// Publish latest solid subtangle milestone hash
func publishLMHS(solidMilestoneHash trinary.Hash) error {
	return mqttBroker.Send(topicLMHS, fmt.Sprintf(`{"solidMilestoneHash":"%v","timestamp":"%s"}`,
		solidMilestoneHash, // Solid milestone transaction hash
		time.Now().UTC().Format(time.RFC3339)))
}

// Publish confirmed transaction
func publishConfTx(iotaTx *transaction.Transaction, ms milestone_index.MilestoneIndex) error {

	return mqttBroker.Send(topicSN, fmt.Sprintf(`{"msIndex":"%d","txHash":"%v","address":"%v","trunk":"%v","branch":"%v","bundle":"%v","timestamp":"%s"}`,
		ms,                       // Index of the milestone that confirmed the transaction
		iotaTx.Hash,              // Transaction hash
		iotaTx.Address,           // Address
		iotaTx.TrunkTransaction,  // Trunk transaction hash
		iotaTx.BranchTransaction, // Branch transaction hash
		iotaTx.Bundle,            // Bundle hash
		time.Now().UTC().Format(time.RFC3339)))
}

// Publish transaction trytes of an tx that has recently been added to the ledger
func publishTxTrytes(iotaTx *transaction.Transaction) error {

	trytes, err := transaction.TransactionToTrytes(iotaTx)
	if err != nil {
		return err
	}

	err = mqttBroker.Send(topicTxTrytes, fmt.Sprintf(`{"txHash":"%v","trytes":"%v","timestamp":"%s"}`,
		iotaTx.Hash, // Transaction hash
		trytes,      // Transaction trytes
		time.Now().UTC().Format(time.RFC3339)))
	return err
}

// Publish a transaction that has recently been added to the ledger
func publishTx(iotaTx *transaction.Transaction) error {

	return mqttBroker.Send(topicTX, fmt.Sprintf(`{"txHash":"%v","address":"%v","value":"%d","obsoleteTag":"%v","txTimestamp":"%d","currentIndex":"%d","lastIndex":"%d","bundle":"%v","trunk":"%v","branch":"%v","recTimestamp":"%d","tag":"%v","timestamp":"%s"}`,
		iotaTx.Hash,              // Transaction hash
		iotaTx.Address,           // Address
		iotaTx.Value,             // Value
		iotaTx.ObsoleteTag,       // Obsolete tag
		iotaTx.Timestamp,         // Timestamp
		iotaTx.CurrentIndex,      // Index of the transaction in the bundle
		iotaTx.LastIndex,         // Last transaction index of the bundle
		iotaTx.Bundle,            // Bundle hash
		iotaTx.TrunkTransaction,  // Trunk transaction hash
		iotaTx.BranchTransaction, // Branch transaction hash
		time.Now().Unix(),        // Unix timestamp for when the transaction was received
		iotaTx.Tag,               // Tag
		time.Now().UTC().Format(time.RFC3339)))
}
