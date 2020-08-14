package mqtt

import (
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	prevSMI milestone.Index = 0
	prevLMI milestone.Index = 0
)

func onNewTx(cachedTx *tangle.CachedTransaction) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction) {

		// tx topic
		err := publishTx(tx.Tx)
		if err != nil {
			log.Warn(err.Error())
		}

		// trytes topic
		err = publishTxTrytes(tx.Tx)
		if err != nil {
			log.Warn(err.Error())
		}
	})
}

func onConfirmedTx(cachedMeta *tangle.CachedMetadata, msIndex milestone.Index, _ int64) {

	cachedMeta.ConsumeMetadata(func(metadata *hornet.TransactionMetadata) {

		cachedTx := tangle.GetCachedTransactionOrNil(metadata.GetTxHash())
		if cachedTx == nil {
			log.Warnf("%w hash: %s", tangle.ErrTransactionNotFound, metadata.GetTxHash().Trytes())
			return
		}

		cachedTx.ConsumeTransaction(func(tx *hornet.Transaction) {
			// conf_trytes topic
			if err := publishConfTrytes(tx.Tx, msIndex); err != nil {
				log.Warn(err.Error())
			}

			// sn topic
			if err := publishConfTx(tx.Tx, msIndex); err != nil {
				log.Warn(err.Error())
			}
		})
	})
}

func onNewLatestMilestone(cachedBndl *tangle.CachedBundle) {
	err := publishLMI(cachedBndl.GetBundle().GetMilestoneIndex())
	if err != nil {
		log.Warn(err.Error())
	}
	err = publishLMHS(cachedBndl.GetBundle().GetMilestoneHash().Trytes())
	if err != nil {
		log.Warn(err.Error())
	}
	err = publishLM(cachedBndl.GetBundle())
	if err != nil {
		log.Warn(err.Error())
	}
	cachedBndl.Release(true) // bundle -1
}

func onNewSolidMilestone(cachedBndl *tangle.CachedBundle) {
	err := publishLMSI(cachedBndl.GetBundle().GetMilestoneIndex())
	if err != nil {
		log.Warn(err.Error())
	}
	err = publishLSM(cachedBndl.GetBundle())
	if err != nil {
		log.Warn(err.Error())
	}
	cachedBndl.Release(true) // bundle -1
}

func onSpentAddress(addr trinary.Hash) {
	if err := publishSpentAddress(addr); err != nil {
		log.Warn(err.Error())
	}
}

// Publish latest milestone index
func publishLMI(lmi milestone.Index) error {

	err := mqttBroker.Send(topicLMI, fmt.Sprintf(`{"prevLMI":%d,"lmi":%d,"timestamp":"%s"}`,
		prevLMI, // Index of the previous solid subtangle milestone
		lmi,     // Index of the latest solid subtangle milestone
		time.Now().UTC().Format(time.RFC3339)))

	// Update previous milestone index
	prevLMI = lmi

	return err
}

// Publish latest solid subtangle milestone index
func publishLMSI(smi milestone.Index) error {

	err := mqttBroker.Send(topicLMSI, fmt.Sprintf(`{"prevSMI":%d,"smi":%d,"timestamp":"%s"}`,
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

// Publish latest milestone
func publishLM(bndl *tangle.Bundle) error {
	return mqttBroker.Send(topicLM, fmt.Sprintf(`{"index":%d,"hash":"%v","timestamp":"%s"}`,
		bndl.GetMilestoneIndex(),         // Milestone transaction index
		bndl.GetMilestoneHash().Trytes(), // Milestone transaction hash
		time.Now().UTC().Format(time.RFC3339)))
}

// Publish latest solid subtangle milestone
func publishLSM(bndl *tangle.Bundle) error {
	return mqttBroker.Send(topicLSM, fmt.Sprintf(`{"index":%d,"hash":"%v","timestamp":"%s"}`,
		bndl.GetMilestoneIndex(),         // Solid milestone transaction index
		bndl.GetMilestoneHash().Trytes(), // Solid milestone transaction hash
		time.Now().UTC().Format(time.RFC3339)))
}

// Publish confirmed transaction
func publishConfTx(iotaTx *transaction.Transaction, msIndex milestone.Index) error {

	return mqttBroker.Send(topicSN, fmt.Sprintf(`{"msIndex":%d,"txHash":"%v","address":"%v","trunk":"%v","branch":"%v","bundle":"%v","timestamp":"%s"}`,
		msIndex,                  // Index of the milestone that confirmed the transaction
		iotaTx.Hash,              // Transaction hash
		iotaTx.Address,           // Address
		iotaTx.TrunkTransaction,  // Trunk transaction hash
		iotaTx.BranchTransaction, // Branch transaction hash
		iotaTx.Bundle,            // Bundle hash
		time.Now().UTC().Format(time.RFC3339)))
}

// Publish confirmed transaction trytes
func publishConfTrytes(iotaTx *transaction.Transaction, msIndex milestone.Index) error {

	trytes, err := transaction.TransactionToTrytes(iotaTx)
	if err != nil {
		return err
	}

	return mqttBroker.Send(topicConfTrytes, fmt.Sprintf(`{"txHash":"%v","trytes":"%v","msIndex":%d,"timestamp":"%s"}`,
		iotaTx.Hash, // Transaction hash
		trytes,      // Transaction trytes
		msIndex,     // Index of the milestone that confirmed the transaction
		time.Now().UTC().Format(time.RFC3339),
	))
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

	return mqttBroker.Send(topicTX, fmt.Sprintf(`{"txHash":"%v","address":"%v","value":%d,"obsoleteTag":"%v","txTimestamp":%d,"currentIndex":%d,"lastIndex":%d,"bundle":"%v","trunk":"%v","branch":"%v","recTimestamp":%d,"tag":"%v","timestamp":"%s"}`,
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

func publishSpentAddress(addr trinary.Hash) error {
	return mqttBroker.Send(topicSpentAddress, addr)
}
