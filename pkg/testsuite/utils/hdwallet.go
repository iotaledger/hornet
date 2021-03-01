package utils

import (
	"bytes"
	"fmt"

	"github.com/wollac/iota-crypto-demo/pkg/bip32path"
	"github.com/wollac/iota-crypto-demo/pkg/slip10"

	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

const (
	pathString = "44'/4218'/0'/%d'"
)

type HDWallet struct {
	name  string
	seed  []byte
	index uint64
	utxo  []*utxo.Output
}

func NewHDWallet(name string, seed []byte, index uint64) *HDWallet {
	return &HDWallet{
		name:  name,
		seed:  seed,
		index: index,
		utxo:  make([]*utxo.Output, 0),
	}
}

func (hd *HDWallet) BookSpents(spentOutputs []*utxo.Output) {
	for _, spent := range spentOutputs {
		hd.BookSpent(spent)
	}
}

func (hd *HDWallet) BookSpent(spentOutput *utxo.Output) {
	newUtxo := make([]*utxo.Output, 0)
	for _, u := range hd.utxo {
		if bytes.Equal(u.OutputID()[:], spentOutput.OutputID()[:]) {
			fmt.Printf("%s spent %s\n", hd.name, u.OutputID().ToHex())
			continue
		}
		newUtxo = append(newUtxo, u)
	}
	hd.utxo = newUtxo
}

func (hd *HDWallet) Name() string {
	return hd.name
}

func (hd *HDWallet) Balance() uint64 {
	var balance uint64
	for _, u := range hd.utxo {
		balance += u.Amount()
	}
	return balance
}

func (hd *HDWallet) BookOutput(output *utxo.Output) {
	if output != nil {
		fmt.Printf("%s book %s\n", hd.name, output.OutputID().ToHex())
		hd.utxo = append(hd.utxo, output)
	}
}

// KeyPair calculates an ed25519 key pair by using slip10.
func (hd *HDWallet) KeyPair() (ed25519.PrivateKey, ed25519.PublicKey) {

	path, err := bip32path.ParsePath(fmt.Sprintf(pathString, hd.index))
	if err != nil {
		panic(err)
	}

	curve := slip10.Ed25519()
	key, err := slip10.DeriveKeyFromPath(hd.seed, curve, path)
	if err != nil {
		panic(err)
	}

	pubKey, privKey := slip10.Ed25519Key(key)
	return ed25519.PrivateKey(privKey), ed25519.PublicKey(pubKey)
}

func (hd *HDWallet) Outputs() []*utxo.Output {
	return hd.utxo
}

// Address calculates an ed25519 address by using slip10.
func (hd *HDWallet) Address() *iotago.Ed25519Address {
	_, pubKey := hd.KeyPair()
	addr := iotago.AddressFromEd25519PubKey(pubKey)
	return &addr
}

func (hd *HDWallet) PrintStatus() {
	var status string
	status += fmt.Sprintf("Name: %s\n", hd.name)
	status += fmt.Sprintf("Address: %s\n", hd.Address().Bech32(iotago.PrefixTestnet))
	status += fmt.Sprintf("Balance: %d\n", hd.Balance())
	status += "Outputs: \n"
	for _, utxo := range hd.utxo {
		var outputType string
		switch utxo.OutputType() {
		case iotago.OutputSigLockedSingleOutput:
			outputType = "SingleOutput"
		case iotago.OutputSigLockedDustAllowanceOutput:
			outputType = "DustAllowance"
		default:
			outputType = fmt.Sprintf("%d", utxo.OutputType())
		}
		status += fmt.Sprintf("\t%s [%s] = %d\n", utxo.OutputID().ToHex(), outputType, utxo.Amount())
	}
	fmt.Printf("%s\n", status)
}
