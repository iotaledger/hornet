package utxo

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	iotago "github.com/iotaledger/iota.go/v2"
)

// ReceiptToOutputs extracts the migrated funds to outputs.
func ReceiptToOutputs(r *iotago.Receipt, msId *iotago.MilestoneID) ([]*Output, error) {
	outputs := make([]*Output, len(r.Funds))
	for outputIndex, migFundsEntry := range r.Funds {
		entry := migFundsEntry.(*iotago.MigratedFundsEntry)
		utxoID := OutputIDForMigratedFunds(*msId, uint16(outputIndex))
		// we use the milestone hash as the "origin message"
		outputs[outputIndex] = CreateOutput(&utxoID, hornet.MessageIDFromArray(*msId), iotago.OutputSigLockedSingleOutput, entry.Address.(iotago.Address), entry.Deposit)
	}
	return outputs, nil
}

// OutputIDForMigratedFunds returns the UTXO ID for a migrated funds entry given the milestone containing the receipt
// and the index of the entry.
func OutputIDForMigratedFunds(milestoneHash iotago.MilestoneID, outputIndex uint16) iotago.UTXOInputID {
	var utxoID iotago.UTXOInputID
	copy(utxoID[:], milestoneHash[:])
	binary.LittleEndian.PutUint16(utxoID[len(utxoID)-2:], outputIndex)
	return utxoID
}

// TreasuryTransactionOutputAmount returns the amount of the treasury output within the treasury transaction.
func TreasuryTransactionOutputAmount(tx *iotago.TreasuryTransaction) uint64 {
	return tx.Output.(*iotago.TreasuryOutput).Amount
}

// ReceiptToTreasuryMutation converts a receipt to a treasury mutation tuple.
func ReceiptToTreasuryMutation(r *iotago.Receipt, unspentTreasuryOutput *TreasuryOutput, newMsId *iotago.MilestoneID) (*TreasuryMutationTuple, error) {
	newOutput := &TreasuryOutput{
		Amount: r.Transaction.(*iotago.TreasuryTransaction).Output.(*iotago.TreasuryOutput).Amount,
		Spent:  false,
	}
	copy(newOutput.MilestoneID[:], newMsId[:])

	return &TreasuryMutationTuple{
		NewOutput:   newOutput,
		SpentOutput: unspentTreasuryOutput,
	}, nil
}
