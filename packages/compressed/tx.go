package compressed

import (
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/curl"
	"github.com/gohornet/hornet/packages/integerutil"
)

const (
	TRANSACTION_SIZE = 1604

	// The amount of bytes used for the requested transaction hash.
	GOSSIP_REQUESTED_TX_HASH_BYTES_LENGTH = 49

	// The amount of bytes making up the non signature message fragment part of a transaction gossip payload.
	NON_SIG_TX_PART_BYTES_LENGTH = 292

	// The max amount of bytes a signature message fragment is made up from.
	SIG_DATA_MAX_BYTES_LENGTH = 1312

	// Total supply of IOTA available in the network. Used for ensuring a balanced ledger state and bundle balances
	// = (3^33 - 1) / 2
	TOTAL_SUPPLY uint64 = 2779530283277761
)

// Truncates the given bytes encoded transaction data.
//	txBytes the transaction bytes to truncate
//	return an array containing the truncated transaction data
func TruncateTx(txBytes []byte) []byte {
	// check how many bytes from the signature can be truncated
	bytesToTruncate := 0
	for i := SIG_DATA_MAX_BYTES_LENGTH - 1; i >= 0; i-- {
		if txBytes[i] != 0 {
			break
		}
		bytesToTruncate++
	}

	// allocate space for truncated tx
	truncatedTx := make([]byte, SIG_DATA_MAX_BYTES_LENGTH-bytesToTruncate+NON_SIG_TX_PART_BYTES_LENGTH)
	copy(truncatedTx, txBytes[:SIG_DATA_MAX_BYTES_LENGTH-bytesToTruncate])
	copy(truncatedTx[SIG_DATA_MAX_BYTES_LENGTH-bytesToTruncate:], txBytes[SIG_DATA_MAX_BYTES_LENGTH:SIG_DATA_MAX_BYTES_LENGTH+NON_SIG_TX_PART_BYTES_LENGTH])
	return truncatedTx
}

// Expands a truncated bytes encoded transaction payload.
func expandTx(data []byte) []byte {
	txDataBytes := make([]byte, TRANSACTION_SIZE)

	// we need to expand the tx data (signature message fragment) as
	// it could have been truncated for transmission
	//numOfBytesOfSigMsgFragToExpand := ProtocolTransactionGossipMsg.MaxLength - uint16(len(data))
	//sigMsgFragPadding := make([]byte, numOfBytesOfSigMsgFragToExpand)
	sigMsgFragBytesToCopy := len(data) - NON_SIG_TX_PART_BYTES_LENGTH

	// build up transaction payload. empty signature message fragment equals padding with 1312x 0 bytes
	copy(txDataBytes, data[:sigMsgFragBytesToCopy])
	//copy(txDataBytes[sigMsgFragBytesToCopy:], sigMsgFragPadding)
	copy(txDataBytes[SIG_DATA_MAX_BYTES_LENGTH:], data[sigMsgFragBytesToCopy:len(data)])

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
		hashTrits := curl.CURLP81.Hash(txDataTrits)
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

		if uint64(integerutil.Abs(tx.Value)) > TOTAL_SUPPLY {
			return nil, consts.ErrInsufficientBalance
		}
	}

	// set the given or calculated TxHash
	tx.Hash = txHash[0]

	return tx, nil
}
