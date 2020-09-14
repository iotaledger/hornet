package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/utils"
)

// GenerateAddress generates an address for the given seed and index with medium security.
func GenerateAddress(t *testing.T, seed trinary.Trytes, index uint64) hornet.Hash {
	seedAddress, err := address.GenerateAddress(seed, index, consts.SecurityLevelMedium, false)
	require.NoError(t, err)

	return hornet.HashFromAddressTrytes(seedAddress)
}

// ZeroValueTx creates a zero value transaction to a random address with the given tag.
func ZeroValueTx(t *testing.T, tag trinary.Trytes) []trinary.Trytes {

	var b bundle.Bundle
	entry := bundle.BundleEntry{
		Address:                   trinary.MustPad(utils.RandomTrytesInsecure(consts.AddressTrinarySize/3), consts.AddressTrinarySize/3),
		Value:                     0,
		Tag:                       tag,
		Timestamp:                 uint64(time.Now().UnixNano() / int64(time.Second)),
		Length:                    uint64(1),
		SignatureMessageFragments: []trinary.Trytes{trinary.MustPad("", consts.SignatureMessageFragmentSizeInTrytes)},
	}
	b, err := bundle.Finalize(bundle.AddEntry(b, entry))
	require.NoError(t, err)

	return transaction.MustFinalTransactionTrytes(b)
}

// ValueTx creates a value transaction with the given tag from an input seed index to an address created by a given output seed and index.
func ValueTx(t *testing.T, tag trinary.Trytes, fromSeed trinary.Trytes, fromIndex uint64, balance uint64, toSeed trinary.Trytes, toIndex uint64, value uint64) []trinary.Trytes {

	_, powFunc := pow.GetFastestProofOfWorkImpl()
	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
		LocalProofOfWorkFunc: powFunc,
	})
	require.NoError(t, err)

	fromAddress, err := address.GenerateAddresses(fromSeed, fromIndex, 2, consts.SecurityLevelMedium, true)
	require.NoError(t, err)

	toAddress, err := address.GenerateAddress(toSeed, toIndex, consts.SecurityLevelMedium, true)
	require.NoError(t, err)

	fmt.Println("Send", value, "from", fromAddress[0], "to", toAddress, "and remaining", balance-value, "to", fromAddress[1])

	transfers := bundle.Transfers{
		{
			Address: toAddress,
			Value:   value,
			Tag:     tag,
		},
	}

	inputs := []api.Input{
		{
			Address:  fromAddress[0],
			Security: consts.SecurityLevelMedium,
			KeyIndex: fromIndex,
			Balance:  balance,
		},
	}

	prepTransferOpts := api.PrepareTransfersOptions{Inputs: inputs, RemainderAddress: &fromAddress[1]}

	trytes, err := iotaAPI.PrepareTransfers(fromSeed, transfers, prepTransferOpts)
	require.NoError(t, err)

	return trytes
}
