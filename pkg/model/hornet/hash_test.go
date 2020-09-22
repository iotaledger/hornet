package hornet_test

import (
	"testing"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/stretchr/testify/assert"
)

func TestHashFromAddressTrytes(t *testing.T) {
	trytes := utils.RandomTrytesInsecure(consts.HashTrytesSize + consts.AddressChecksumTrytesSize/consts.TritsPerTryte)
	hash := hornet.HashFromAddressTrytes(trytes)

	assert.Len(t, hash, 49)
	assert.Equal(t, trytes[:consts.HashTrytesSize], hash.Trytes())
	assert.Equal(t, trinary.MustTrytesToTrits(trytes[:consts.HashTrytesSize]), hash.Trits())
}

func TestHashFromHashTrytes(t *testing.T) {
	trytes := utils.RandomTrytesInsecure(consts.HashTrytesSize)
	hash := hornet.HashFromHashTrytes(trytes)

	assert.Len(t, hash, 49)
	assert.Equal(t, trytes, hash.Trytes())
	assert.Equal(t, trinary.MustTrytesToTrits(trytes), hash.Trits())
}

func TestHashFromTagTrytes(t *testing.T) {
	trytes := utils.RandomTrytesInsecure(consts.TagTrinarySize / consts.TritsPerTryte)
	hash := hornet.HashFromTagTrytes(trytes)

	assert.Len(t, hash, 17)
	assert.Equal(t, trytes, hash.Trytes())
	assert.Equal(t, trinary.MustTrytesToTrits(trytes), hash.Trits())
}

func TestHashes_Trytes(t *testing.T) {
	hashTrytes := utils.RandomTrytesInsecure(consts.HashTrytesSize)
	tagTrytes := utils.RandomTrytesInsecure(consts.TagTrinarySize / consts.TritsPerTryte)
	var hashes = hornet.Hashes{
		hornet.HashFromHashTrytes(hashTrytes),
		hornet.HashFromTagTrytes(tagTrytes),
	}

	assert.ElementsMatch(t, []trinary.Trytes{hashTrytes, tagTrytes}, hashes.Trytes())
}
