package test

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"testing"

	_ "golang.org/x/crypto/blake2b" // import implementation

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

func TestWhiteFlagMerkleTreeHash(t *testing.T) {

	// Test vectors taken from the example in the RFC-0012: https://github.com/Wollac/iota-crypto-demo/tree/master/examples/merkle

	var tailHashes []hornet.Hash

	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("NOBKDFGZMOWYUKDZITTWBRWA9YPSXCVFENCQFPC9GMJIAIPSSURYIOMYZLGNZXLUAQHHNBSRHNOIJDYZO"))
	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("IPATPTEZSBMFJRDCRPTCVUQWBAVCAXAVZIDEDL9TSILDFWDMIIFPZIYHKRFFZDYQNKBQBVGYSKMLCYBMR"))
	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("MXOIOFOGLIHCHMDRCWAIYCWIUCMGEZWXFJZFWBRCNSNBWIGFJXBCACPKMLLANYNXSGYKANYFTVGTLFXXX"))
	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("EXZTJAXJMZJBBIZGUTMBOEUQDNVHJPXCLFUXNLPLSBATDMKYUZOFMHCOBWUABYDMNGMKIXLIUFXNVY9PN"))
	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("SJXYVFUDCDPPAOALVXDQUKAWLLOQO99OSJQT9TUNILQ9VLFLCZMLZAKUTIZFHOLPMGPYHKMMUUSURIOCF"))
	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("Q9GHMAITEZCWKFIESJARYQYMF9XWFPQTTFRXULLHQDWEZLYBSFYHSLPXEHBORDDFYZRFYFGDCM9VJKEFR"))
	tailHashes = append(tailHashes, trinary.MustTrytesToBytes("GMNECTSPSLSPPEITCHBXSN9KZD9OZPVPOET9TVQJDZMFGN9SGPRPMUQARNXUVKMWAFAKLKWBZLWZCTPCP"))

	hash := whiteflag.NewHasher(crypto.BLAKE2b_512).TreeHash(tailHashes)

	expectedHash, err := hex.DecodeString("d07161bdb535afb7dbb3f5b2fb198ecf715cbd9dfca133d2b48d67b1e11173c6f92bed2f4dca92c36e8d1ef279a0c19ca9e40a113e9f5526090342988f86e53a")
	require.NoError(t, err)
	require.True(t, bytes.Equal(hash, expectedHash))
}
