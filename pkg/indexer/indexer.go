package indexer

import (
	"github.com/gohornet/hornet/pkg/whiteflag"
)

type Indexer struct {
}

func NewIndexer() (*Indexer, error) {
	return &Indexer{}, nil
}

func (i *Indexer) ApplyNewConfirmation(confirmation *whiteflag.Confirmation) error {
	return nil
}

func (i *Indexer) CloseDatabase() error {
	return nil
}
