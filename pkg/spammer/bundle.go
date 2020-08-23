package spammer

import (
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/converter"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/signing"
	"github.com/iotaledger/iota.go/signing/key"
	"github.com/iotaledger/iota.go/trinary"
)

const (
	alphabet = "9ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func integerToAscii(number int) string {
	result := ""
	for index := 0; index < 7; index++ {
		pos := number % 27
		number /= 27
		result = string(alphabet[pos]) + result
	}
	return result
}

// We don't need to care about the M-Bug in the spammer => much faster without
func finalizeInsecure(bundle bundle.Bundle) (bundle.Bundle, error) {
	var valueTrits = make([]trinary.Trits, len(bundle))
	var timestampTrits = make([]trinary.Trits, len(bundle))
	var currentIndexTrits = make([]trinary.Trits, len(bundle))
	var obsoleteTagTrits = make([]trinary.Trits, len(bundle))
	var lastIndexTrits = trinary.MustPadTrits(trinary.IntToTrits(int64(bundle[0].LastIndex)), 27)

	for i := range bundle {
		valueTrits[i] = trinary.MustPadTrits(trinary.IntToTrits(bundle[i].Value), 81)
		timestampTrits[i] = trinary.MustPadTrits(trinary.IntToTrits(int64(bundle[i].Timestamp)), 27)
		currentIndexTrits[i] = trinary.MustPadTrits(trinary.IntToTrits(int64(bundle[i].CurrentIndex)), 27)
		obsoleteTagTrits[i] = trinary.MustPadTrits(trinary.MustTrytesToTrits(bundle[i].ObsoleteTag), 81)
	}

	var bundleHash trinary.Hash

	k := kerl.NewKerl()

	for i := 0; i < len(bundle); i++ {
		relevantTritsForBundleHash := trinary.MustTrytesToTrits(
			bundle[i].Address +
				trinary.MustTritsToTrytes(valueTrits[i]) +
				trinary.MustTritsToTrytes(obsoleteTagTrits[i]) +
				trinary.MustTritsToTrytes(timestampTrits[i]) +
				trinary.MustTritsToTrytes(currentIndexTrits[i]) +
				trinary.MustTritsToTrytes(lastIndexTrits),
		)
		k.Absorb(relevantTritsForBundleHash)
	}

	bundleHashTrits, err := k.Squeeze(consts.HashTrinarySize)
	if err != nil {
		return nil, err
	}
	bundleHash = trinary.MustTritsToTrytes(bundleHashTrits)

	// set the computed bundle hash on each tx in the bundle
	for i := range bundle {
		tx := &bundle[i]
		tx.Bundle = bundleHash
	}

	return bundle, nil
}

func createBundle(seed trinary.Trytes, seedIndex uint64, txAddress trinary.Hash, msg string, tagSubstring string, bundleSize int, valueSpam bool, txCount int, additionalMesssage ...string) (bundle.Bundle, error) {

	tag, err := trinary.NewTrytes(tagSubstring + integerToAscii(txCount))
	if err != nil {
		return nil, fmt.Errorf("NewTrytes: %v", err.Error())
	}
	now := time.Now()

	messageString := msg + fmt.Sprintf("\nCount: %06d", txCount)
	messageString += fmt.Sprintf("\nTimestamp: %s", now.Format(time.RFC3339))
	if len(additionalMesssage) > 0 {
		messageString = fmt.Sprintf("%v\n%v", messageString, additionalMesssage[0])
	}

	message, err := converter.ASCIIToTrytes(messageString)
	if err != nil {
		return nil, fmt.Errorf("ASCIIToTrytes: %v", err.Error())
	}

	timestamp := uint64(now.UnixNano() / int64(time.Second))

	var b bundle.Bundle

	if !valueSpam {
		for i := 0; i < bundleSize; i++ {
			outEntry := bundle.BundleEntry{
				Address:                   txAddress,
				Value:                     0,
				Tag:                       tag,
				Timestamp:                 timestamp,
				Length:                    uint64(1),
				SignatureMessageFragments: []trinary.Trytes{trinary.MustPad(message, consts.SignatureMessageFragmentSizeInTrytes)},
			}
			b = bundle.AddEntry(b, outEntry)
		}
	} else {
		addresses, err := address.GenerateAddresses(seed, seedIndex, 1, consts.SecurityLevelLow, true)
		if err != nil {
			return nil, fmt.Errorf("address.GenerateAddresses: %v", err.Error())
		}

		addr := addresses[0][:consts.HashTrytesSize]

		outEntry := bundle.BundleEntry{
			Address:                   addr,
			Value:                     int64(bundleSize - 1),
			Tag:                       tag,
			Timestamp:                 timestamp,
			Length:                    uint64(1),
			SignatureMessageFragments: []trinary.Trytes{trinary.MustPad(message, consts.SignatureMessageFragmentSizeInTrytes)},
		}
		b = bundle.AddEntry(b, outEntry)

		for i := 0; i < bundleSize-1; i++ {
			inEntry := bundle.BundleEntry{
				Address:   addr,
				Value:     int64(-1),
				Tag:       tag,
				Timestamp: timestamp,
				Length:    uint64(consts.SecurityLevelLow),
			}
			b = bundle.AddEntry(b, inEntry)
		}
	}

	// finalize bundle by adding the bundle hash
	b, err = finalizeInsecure(b)
	if err != nil {
		return nil, fmt.Errorf("Bundle.Finalize: %v", err.Error())
	}

	if valueSpam {
		// compute signatures for all input txs
		normalizedBundleHash := signing.NormalizedBundleHash(b[0].Bundle)

		subseed, err := signing.Subseed(seed, seedIndex)
		if err != nil {
			return nil, fmt.Errorf("signing.Subseed: %v", err.Error())
		}

		h := kerl.NewKerl()

		prvKey, err := key.Sponge(subseed, consts.SecurityLevelLow, h)
		if err != nil {
			return nil, fmt.Errorf("signing.Key: %v", err.Error())
		}

		signedFrags := make([]trinary.Trytes, consts.SecurityLevelLow)
		for i := 0; i < int(consts.SecurityLevelLow); i++ {
			signedFragTrits, err := signing.SignatureFragment(
				normalizedBundleHash[i*consts.HashTrytesSize/3:(i+1)*consts.HashTrytesSize/3],
				prvKey[i*consts.KeyFragmentLength:(i+1)*consts.KeyFragmentLength],
			)
			if err != nil {
				return nil, fmt.Errorf("signing.SignatureFragment: %v", err.Error())
			}
			signedFrags[i] = trinary.MustTritsToTrytes(signedFragTrits)
		}

		// add signed fragments to txs
		var indexFirstInputTx int = 1
		for i := 0; i < bundleSize-1; i++ {
			b = bundle.AddTrytes(b, signedFrags, indexFirstInputTx)
			indexFirstInputTx++
		}
	}

	return b, nil
}
