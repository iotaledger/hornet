package compressed

import (
	"fmt"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/encoding/t5b1"
	"github.com/iotaledger/iota.go/math"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/plugins/curl"
)

const (
	// The size of a transaction in bytes.
	TransactionSize = 1604

	// The amount of bytes making up the non signature message fragment part of a transaction gossip payload.
	NonSigTxPartBytesLength = 292

	// The max amount of bytes a signature message fragment is made up from.
	SigDataMaxBytesLength = 1312
)

// TruncateTxTrits encodes txTrits into bytes and truncates them.
func TruncateTxTrits(txTrits trinary.Trits) []byte {
	txBytes := make([]byte, t5b1.EncodedLen(len(txTrits)))
	t5b1.Encode(txBytes, txTrits)
	return TruncateTx(txBytes)
}

// TruncateTx truncates the encoded transaction data.
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
func expandTx(data []byte) ([]byte, error) {
	if len(data) < NonSigTxPartBytesLength {
		return nil, fmt.Errorf("insufficient tx payload length. minimum: %d, actual: %d", NonSigTxPartBytesLength, len(data))
	}

	txDataBytes := make([]byte, TransactionSize)

	// we need to expand the tx data (signature message fragment) as
	// it could have been truncated for transmission
	sigMsgFragBytesToCopy := len(data) - NonSigTxPartBytesLength

	// build up transaction payload. empty signature message fragment equals padding with 1312x 0 bytes
	copy(txDataBytes, data[:sigMsgFragBytesToCopy])
	copy(txDataBytes[SigDataMaxBytesLength:], data[sigMsgFragBytesToCopy:])

	return txDataBytes, nil
}

func TransactionFromCompressedBytes(transactionData []byte, txHash ...trinary.Hash) (*transaction.Transaction, error) {
	// expand received tx data
	txDataBytes, err := expandTx(transactionData)
	if err != nil {
		return nil, err
	}

	// decode the bytes back into trits
	txDataTrits := make(trinary.Trits, t5b1.DecodedLen(TransactionSize))
	_, err = t5b1.Decode(txDataTrits, txDataBytes)
	if err != nil {
		return nil, err
	}
	// truncate the padding
	txDataTrits = txDataTrits[:consts.TransactionTrinarySize]

	// calculate the transaction hash with the batched hasher if not given
	skipHashCalc := len(txHash) > 0
	if !skipHashCalc {
		hashTrits, err := curl.Hasher().Hash(txDataTrits)
		if err != nil {
			return nil, err
		}
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

		if math.AbsInt64(tx.Value) > consts.TotalSupply {
			return nil, consts.ErrInsufficientBalance
		}
	}

	// set the given or calculated TxHash
	tx.Hash = txHash[0]

	return tx, nil
}
