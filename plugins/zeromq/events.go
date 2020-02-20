package zeromq

import (
	"strconv"
	"strings"
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

func onNewTx(cachedTx *tangle.CachedTransaction) {

	cachedTx.ConsumeTransaction(func(tx *tangle.Transaction) {
		// tx topic
		err := publishTx(tx.Tx)
		if err != nil {
			log.Error(err.Error())
		}

		// trytes topic
		err = publishTxTrytes(tx.Tx)
		if err != nil {
			log.Error(err.Error())
		}
	})
}

func onConfirmedTx(cachedTx *tangle.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {

	cachedTx.ConsumeTransaction(func(tx *tangle.Transaction) {
		err := publishConfTx(tx.Tx, msIndex)
		if err != nil {
			log.Error(err.Error())
		}

		addresses := GetAddressTopics()
		for _, addr := range addresses {
			if strings.EqualFold(tx.Tx.Address, addr) {
				err := publishConfTxForAddress(tx.Tx, msIndex)
				if err != nil {
					log.Error(err.Error())
				}
			}
		}
	})
}

func onNewLatestMilestone(cachedBndl *tangle.CachedBundle) {
	err := publishLMI(cachedBndl.GetBundle().GetMilestoneIndex())
	if err != nil {
		log.Error(err.Error())
	}
	err = publishLMHS(cachedBndl.GetBundle().GetMilestoneHash())
	if err != nil {
		log.Error(err.Error())
	}
	cachedBndl.Release() // bundle -1
}

func onNewSolidMilestone(cachedBndl *tangle.CachedBundle) {
	err := publishLMSI(cachedBndl.GetBundle().GetMilestoneIndex())
	if err != nil {
		log.Error(err.Error())
	}
	cachedBndl.Release() // bundle -1
}

func onSpentAddress(addr trinary.Hash) {
	if err := publishSpentAddress(addr); err != nil {
		log.Error(err.Error())
	}
}

// Publish latest milestone index
func publishLMI(lmi milestone_index.MilestoneIndex) error {

	messages := []string{
		strconv.FormatInt(int64(prevLMI), 10), // Index of the previous solid subtangle milestone
		strconv.FormatInt(int64(lmi), 10),     // Index of the latest solid subtangle milestone
	}

	// Update previous milestone index
	prevLMI = lmi

	return publisher.Send(topicLMI, messages)
}

// Publish latest solid subtangle milestone index
func publishLMSI(smi milestone_index.MilestoneIndex) error {

	messages := []string{
		strconv.FormatInt(int64(prevSMI), 10), // Index of the previous solid subtangle milestone
		strconv.FormatInt(int64(smi), 10),     // Index of the latest solid subtangle milestone
	}

	// Update previous milestone index
	prevSMI = smi

	return publisher.Send(topicLMSI, messages)
}

// Publish latest solid subtangle milestone hash
func publishLMHS(solidMilestoneHash trinary.Hash) error {
	messages := []string{
		solidMilestoneHash, // Solid milestone transaction hash
	}

	return publisher.Send(topicLMHS, messages)
}

// Publish confirmed transaction
func publishConfTx(iotaTx *transaction.Transaction, msIndex milestone_index.MilestoneIndex) error {

	messages := []string{
		strconv.FormatInt(int64(msIndex), 10), // Index of the milestone that confirmed the transaction
		iotaTx.Hash,                           // Transaction hash
		iotaTx.Address,                        // Address
		iotaTx.TrunkTransaction,               // Trunk transaction hash
		iotaTx.BranchTransaction,              // Branch transaction hash
		iotaTx.Bundle,                         // Bundle hash
	}

	return publisher.Send(topicSN, messages)
}

// Publish transaction trytes of an tx that has recently been added to the ledger
func publishTxTrytes(iotaTx *transaction.Transaction) error {

	trytes, err := transaction.TransactionToTrytes(iotaTx)
	if err != nil {
		return err
	}

	messages := []string{
		trytes,      // Transaction trytes
		iotaTx.Hash, // Transaction hash
	}

	return publisher.Send(topicTxTrytes, messages)
}

// Publish a transaction that has recently been added to the ledger
func publishTx(iotaTx *transaction.Transaction) error {

	messages := []string{
		iotaTx.Hash,                         // Transaction hash
		iotaTx.Address,                      // Address
		strconv.FormatInt(iotaTx.Value, 10), // Value
		iotaTx.ObsoleteTag,                  // Obsolete tag
		strconv.FormatInt(int64(iotaTx.Timestamp), 10),    // Timestamp
		strconv.FormatInt(int64(iotaTx.CurrentIndex), 10), // Index of the transaction in the bundle
		strconv.FormatInt(int64(iotaTx.LastIndex), 10),    // Last transaction index of the bundle
		iotaTx.Bundle,                            // Bundle hash
		iotaTx.TrunkTransaction,                  // Trunk transaction hash
		iotaTx.BranchTransaction,                 // Branch transaction hash
		strconv.FormatInt(time.Now().Unix(), 10), // Unix timestamp for when the transaction was received
		iotaTx.Tag,                               // Tag
	}

	return publisher.Send(topicTX, messages)
}

// Publish a confirmed transaction for a specific address
func publishConfTxForAddress(iotaTx *transaction.Transaction, msIndex milestone_index.MilestoneIndex) error {

	addr := strings.ToUpper(iotaTx.Address)
	messages := []string{
		addr,
		iotaTx.Hash,
		strconv.FormatInt(int64(msIndex), 10),
	}

	return publisher.Send(addr, messages)
}

func publishSpentAddress(addr trinary.Hash) error {
	return publisher.Send(topicSpentAddress, []string{addr})
}
