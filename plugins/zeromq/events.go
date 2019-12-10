package zeromq

import (
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

var (
	prevSMI milestone_index.MilestoneIndex = 0
	prevLMI milestone_index.MilestoneIndex = 0
)

func onNewTx(tx *hornet.Transaction) {
	// tx topic
	err := publishTx(tx)
	if err != nil {
		log.Error(err.Error())
	}

	// tx_trytes topic
	err = publishTxTrytes(tx)
	if err != nil {
		log.Error(err.Error())
	}
}

func onConfirmedTx(tx *hornet.Transaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
	err := publishConfTx(tx, msIndex)
	if err != nil {
		log.Error(err.Error())
	}

	addresses := GetAddressTopics()
	for _, addr := range addresses {
		if strings.ToUpper(tx.Tx.Address) == strings.ToUpper(addr) {
			err := publishConfTxForAddress(tx, msIndex)
			if err != nil {
				log.Error(err.Error())
			}
		}
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
func publishConfTx(tx *hornet.Transaction, ms milestone_index.MilestoneIndex) error {

	messages := []string{
		strconv.FormatInt(int64(ms), 10), // Index of the milestone that confirmed the transaction
		tx.Tx.Hash,                       // Transaction hash
		tx.Tx.Address,                    // Address
		tx.Tx.TrunkTransaction,           // Trunk transaction hash
		tx.Tx.BranchTransaction,          // Branch transaction hash
		tx.Tx.Bundle,                     // Bundle hash
	}

	return publisher.Send(topicSN, messages)
}

// Publish transaction trytes of an tx that has recently been added to the ledger
func publishTxTrytes(tx *hornet.Transaction) error {

	trytes, err := transaction.TransactionToTrytes(tx.Tx)
	if err != nil {
		return err
	}

	messages := []string{
		tx.Tx.Hash, // Transaction hash
		trytes,     // Transaction trytes
	}

	return publisher.Send(topicTxTrytes, messages)
}

// Publish a transaction that has recently been added to the ledger
func publishTx(tx *hornet.Transaction) error {

	messages := []string{
		tx.Tx.Hash,                         // Transaction hash
		tx.Tx.Address,                      // Address
		strconv.FormatInt(tx.Tx.Value, 10), // Value
		tx.Tx.ObsoleteTag,                  // Obsolete tag
		strconv.FormatInt(int64(tx.Tx.Timestamp), 10),    // Timestamp
		strconv.FormatInt(int64(tx.Tx.CurrentIndex), 10), // Index of the transaction in the bundle
		strconv.FormatInt(int64(tx.Tx.LastIndex), 10),    // Last transaction index of the bundle
		tx.Tx.Bundle,                             // Bundle hash
		tx.Tx.TrunkTransaction,                   // Trunk transaction hash
		tx.Tx.BranchTransaction,                  // Branch transaction hash
		strconv.FormatInt(time.Now().Unix(), 10), // Unix timestamp for when the transaction was received
		tx.Tx.Tag,                                // Tag
	}

	return publisher.Send(topicTX, messages)
}

// Publish a confirmed transaction for a specific address
func publishConfTxForAddress(tx *hornet.Transaction, msIndex milestone_index.MilestoneIndex) error {

	addr := strings.ToUpper(tx.Tx.Address)
	messages := []string{
		addr,
		tx.Tx.Hash,
		strconv.FormatInt(int64(msIndex), 10),
	}

	return publisher.Send(addr, messages)
}
