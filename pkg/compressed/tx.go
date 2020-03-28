package compressed

import (
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/batchhasher"
	"github.com/iotaledger/hive.go/math"
)

const (
	// The size of a transaction in bytes.
	TransactionSize = 1604

	// The amount of bytes making up the non signature message fragment part of a transaction gossip payload.
	NonSigTxPartBytesLength = 292

	// The max amount of bytes a signature message fragment is made up from.
	SigDataMaxBytesLength = 1312

	// Total supply of IOTA available in the network. Used for ensuring a balanced ledger state and bundle balances
	// = (3^33 - 1) / 2
	TotalSupply uint64 = 2779530283277761
)

// Truncates the given bytes encoded transaction data.
//	txBytes the transaction bytes to truncate
//	return an array containing the truncated transaction data
func TruncateTx(txBytes []byte) []byte {
	// check how many bytes from the signature can be truncated
	bytesToTruncate := 0
	for i := SigDataMaxBytesLength - 1; i >= 0; i-- {
		if txBytes[i] != 0 {
			break
		}
		bytesToTruncate++
	}

	// allocate space for truncated tx
	truncatedTx := make([]byte, SigDataMaxBytesLength-bytesToTruncate+NonSigTxPartBytesLength)
	copy(truncatedTx, txBytes[:SigDataMaxBytesLength-bytesToTruncate])
	copy(truncatedTx[SigDataMaxBytesLength-bytesToTruncate:], txBytes[SigDataMaxBytesLength:SigDataMaxBytesLength+NonSigTxPartBytesLength])
	return truncatedTx
}

// Expands a truncated bytes encoded transaction payload.
func expandTx(data []byte) []byte {
	txDataBytes := make([]byte, TransactionSize)

	// we need to expand the tx data (signature message fragment) as
	// it could have been truncated for transmission
	//numOfBytesOfSigMsgFragToExpand := ProtocolTransactionGossipMsg.MaxBytesLength - uint16(len(data))
	//sigMsgFragPadding := make([]byte, numOfBytesOfSigMsgFragToExpand)
	sigMsgFragBytesToCopy := len(data) - NonSigTxPartBytesLength

	// build up transaction payload. empty signature message fragment equals padding with 1312x 0 bytes
	copy(txDataBytes, data[:sigMsgFragBytesToCopy])
	//copy(txDataBytes[sigMsgFragBytesToCopy:], sigMsgFragPadding)
	copy(txDataBytes[SigDataMaxBytesLength:], data[sigMsgFragBytesToCopy:])

	return txDataBytes
}

func TransactionFromCompressedBytes(transactionData []byte, txHash ...trinary.Hash) (*transaction.Transaction, error) {

	// expand received tx data
	txDataBytes := expandTx(transactionData)

	// convert bytes to trits
	txDataTrits, err := trinary.BytesToTrits(txDataBytes, consts.TransactionTrinarySize)
	if err != nil {
		return nil, err
	}

	// calculate the transaction hash with the batched hasher if not given
	skipHashCalc := len(txHash) > 0
	if !skipHashCalc {
		hashTrits := batchhasher.CURLP81.Hash(txDataTrits)
		txHash = []trinary.Hash{trinary.MustTritsToTrytes(hashTrits)}
	}

	tx, err := transaction.ParseTransaction(txDataTrits, true)
	if err != nil {
		return nil, err
	}

	if tx.Value != 0 {
		// Additional checks
		if txDataTrits[consts.AddressTrinaryOffset+consts.AddressTrinarySize-1] != 0 {
			// The last trit is always zero because of KERL/keccak
			return nil, consts.ErrInvalidAddress
		}

		if uint64(math.Abs(tx.Value)) > TotalSupply {
			return nil, consts.ErrInsufficientBalance
		}
	}

	// set the given or calculated TxHash
	tx.Hash = txHash[0]

	return tx, nil
}
