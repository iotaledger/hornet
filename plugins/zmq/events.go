package zmq

import (
	"strconv"
	"strings"
	"time"

	"github.com/muxxer/iota.go/transaction"
	"github.com/muxxer/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	prevSMI milestone.Index = 0
	prevLMI milestone.Index = 0
)

func onNewTx(cachedTx *tangle.CachedMessage) {

	cachedTx.ConsumeMessage(func(msg *tangle.Message) {
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

	cachedMeta.ConsumeMetadata(func(metadata *hornet.MessageMetadata) {

		cachedTx := tangle.GetCachedMessageOrNil(metadata.GetMessageID())
		if cachedTx == nil {
			log.Warnf("%w hash: %s", tangle.ErrMessageNotFound, metadata.GetMessageID().Hex())
			return
		}

		cachedTx.ConsumeMessage(func(msg *tangle.Message) {
			if err := publishConfTx(tx.Tx, msIndex); err != nil {
				log.Warn(err.Error())
			}

			if err := publishConfTrytes(tx.Tx, msIndex); err != nil {
				log.Warn(err.Error())
			}

			addresses := GetAddressTopics()
			for _, addr := range addresses {
				if strings.EqualFold(tx.Tx.Address, addr) {
					err := publishConfTxForAddress(tx.Tx, msIndex)
					if err != nil {
						log.Warn(err.Error())
					}
				}
			}
		})
	})
}

func onNewLatestMilestone(cachedMilestone *tangle.CachedMilestone) {
	err := publishLMI(cachedMilestone.GetMilestone().Index)
	if err != nil {
		log.Warn(err.Error())
	}
	err = publishLMHS(cachedMilestone.GetMilestone().MessageID.Hex())
	if err != nil {
		log.Warn(err.Error())
	}
	err = publishLM(cachedMilestone.GetMilestone())
	if err != nil {
		log.Warn(err.Error())
	}
	cachedMilestone.Release(true) // message -1
}

func onNewSolidMilestone(cachedMilestone *tangle.CachedMilestone) {
	err := publishLMSI(cachedMilestone.GetMilestone().Index)
	if err != nil {
		log.Warn(err.Error())
	}
	err = publishLSM(cachedMilestone.GetMilestone())
	if err != nil {
		log.Warn(err.Error())
	}
	cachedMilestone.Release(true) // message -1
}

func onSpentAddress(addr trinary.Hash) {
	if err := publishSpentAddress(addr); err != nil {
		log.Warn(err.Error())
	}
}

// Publish latest milestone index
func publishLMI(lmi milestone.Index) error {

	messages := []string{
		strconv.FormatInt(int64(prevLMI), 10), // Index of the previous solid subtangle milestone
		strconv.FormatInt(int64(lmi), 10),     // Index of the latest solid subtangle milestone
	}

	// Update previous milestone index
	prevLMI = lmi

	return publisher.Send(topicLMI, messages)
}

// Publish latest solid subtangle milestone index
func publishLMSI(smi milestone.Index) error {

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

// Publish latest milestone
func publishLM(ms *tangle.Milestone) error {
	messages := []string{
		strconv.FormatUint(uint64(ms.Index), 10),
		ms.MessageID.Hex(),
	}

	return publisher.Send(topicLM, messages)
}

// Publish latest solid subtangle milestone
func publishLSM(ms *tangle.Milestone) error {
	messages := []string{
		strconv.FormatUint(uint64(ms.Index), 10),
		ms.MessageID.Hex(),
	}

	return publisher.Send(topicLSM, messages)
}

// Publish confirmed transaction
func publishConfTx(iotaTx *transaction.Transaction, msIndex milestone.Index) error {

	messages := []string{
		strconv.FormatInt(int64(msIndex), 10), // Index of the milestone that confirmed the transaction
		iotaTx.Hash,                           // Transaction hash
		iotaTx.Address,                        // Address
		iotaTx.TrunkTransaction,               // Trunk transaction hash
		iotaTx.BranchTransaction,              // Branch transaction hash
		iotaTx.Bundle,                         // Message hash
	}

	return publisher.Send(topicSN, messages)
}

// Publish confirmed trytes
func publishConfTrytes(iotaTx *transaction.Transaction, msIndex milestone.Index) error {

	trytes, err := transaction.TransactionToTrytes(iotaTx)
	if err != nil {
		return err
	}

	messages := []string{
		iotaTx.Hash,                           // Transaction hash
		trytes,                                // Transaction trytes
		strconv.FormatInt(int64(msIndex), 10), // Index of the milestone that confirmed the transaction
	}

	return publisher.Send(topicConfTrytes, messages)
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
		iotaTx.Bundle,                            // Message hash
		iotaTx.TrunkTransaction,                  // Trunk transaction hash
		iotaTx.BranchTransaction,                 // Branch transaction hash
		strconv.FormatInt(time.Now().Unix(), 10), // Unix timestamp for when the transaction was received
		iotaTx.Tag,                               // Tag
	}

	return publisher.Send(topicTX, messages)
}

// Publish a confirmed transaction for a specific address
func publishConfTxForAddress(iotaTx *transaction.Transaction, msIndex milestone.Index) error {

	messages := []string{
		iotaTx.Hash,
		strconv.FormatInt(int64(msIndex), 10),
	}

	return publisher.Send(strings.ToUpper(iotaTx.Address), messages)
}

func publishSpentAddress(addr trinary.Hash) error {
	return publisher.Send(topicSpentAddress, []string{addr})
}
