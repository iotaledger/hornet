package toolset

import (
	"encoding/binary"
	"sync/atomic"

	legacy "github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/curl/bct"
	"github.com/iotaledger/iota.go/encoding/b1t6"
	"github.com/iotaledger/iota.go/trinary"
)

const (
	nonceBytes = 8 // len(uint64)
)

// encodeNonce encodes nonce as 48 trits using the b1t6 encoding.
func encodeNonce(dst trinary.Trits, nonce uint64) {
	var nonceBuf [nonceBytes]byte
	binary.LittleEndian.PutUint64(nonceBuf[:], nonce)
	b1t6.Encode(dst, nonceBuf[:])
}

func cpuBenchmarkWorker(powDigest []byte, startNonce uint64, done *uint32, counter *uint64) error {
	// use batched Curl hashing
	c := bct.NewCurlP81()
	var l, h [legacy.HashTrinarySize]uint

	// allocate exactly one Curl block for each batch index and fill it with the encoded digest
	buf := make([]trinary.Trits, bct.MaxBatchSize)
	for i := range buf {
		buf[i] = make(trinary.Trits, legacy.HashTrinarySize)
		b1t6.Encode(buf[i], powDigest)
	}

	digestTritsLen := b1t6.EncodedLen(len(powDigest))
	for nonce := startNonce; atomic.LoadUint32(done) == 0; nonce += bct.MaxBatchSize {
		// add the nonce to each trit buffer
		for i := range buf {
			nonceBuf := buf[i][digestTritsLen:]
			encodeNonce(nonceBuf, nonce+uint64(i))
		}

		// process the batch
		c.Reset()
		if err := c.Absorb(buf, legacy.HashTrinarySize); err != nil {
			return err
		}
		c.CopyState(l[:], h[:]) // the first 243 entries of the state correspond to the resulting hashes
		atomic.AddUint64(counter, bct.MaxBatchSize)
	}

	return nil
}
