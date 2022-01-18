package indexer

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
)

func (i *Indexer) ImportTransaction() *ImportTransaction {
	return newImportTransaction(i.db)
}

type ImportTransaction struct {
	tx *gorm.DB
}

func newImportTransaction(db *gorm.DB) *ImportTransaction {
	return &ImportTransaction{
		tx: db.Begin(),
	}
}

func (i *ImportTransaction) AddOutput(output *utxo.Output) error {
	if err := processOutput(output, i.tx); err != nil {
		i.tx.Rollback()
		return err
	}
	return nil
}

func (i *ImportTransaction) Finalize(ledgerIndex milestone.Index) error {
	// Update the ledger index
	status := &status{
		ID:          1,
		LedgerIndex: ledgerIndex,
	}
	i.tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&status)

	return i.tx.Commit().Error
}

func (i *ImportTransaction) Cancel() error {
	return i.tx.Rollback().Error
}
