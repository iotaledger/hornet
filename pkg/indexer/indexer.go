package indexer

import (
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/kvstore/utils"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrNotFound = errors.New("output not found for given filter")

	tables = []interface{}{
		&status{},
		&extendedOutput{},
		&nft{},
		&foundry{},
		&alias{},
	}
)

type Indexer struct {
	db *gorm.DB
}

func NewIndexer(dbPath string) (*Indexer, error) {

	if err := utils.CreateDirectory(dbPath, 0700); err != nil {
		return nil, err
	}

	dbFile := filepath.Join(dbPath, "indexer.db")

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Create the tables and indexes if needed
	if err := db.AutoMigrate(tables...); err != nil {
		return nil, err
	}

	return &Indexer{
		db: db,
	}, nil
}

func processSpent(spent *utxo.Spent, tx *gorm.DB) error {
	switch spent.OutputType() {
	case iotago.OutputExtended:
		return tx.Where("output_id = ?", spent.OutputID()[:]).Delete(&extendedOutput{}).Error
	case iotago.OutputAlias:
		return tx.Where("output_id = ?", spent.OutputID()[:]).Delete(&alias{}).Error
	case iotago.OutputNFT:
		return tx.Where("output_id = ?", spent.OutputID()[:]).Delete(&nft{}).Error
	case iotago.OutputFoundry:
		return tx.Where("output_id = ?", spent.OutputID()[:]).Delete(&foundry{}).Error
	}
	return nil
}

func processOutput(output *utxo.Output, tx *gorm.DB) error {
	switch iotaOutput := output.Output().(type) {
	case *iotago.ExtendedOutput:
		features, err := iotaOutput.FeatureBlocks().Set()
		if err != nil {
			return err
		}

		address, err := addressBytesForAddress(iotaOutput.Address)
		if err != nil {
			return err
		}
		extended := &extendedOutput{
			OutputID:       make(outputIDBytes, iotago.OutputIDLength),
			Address:        address,
			Amount:         iotaOutput.Amount,
			MilestoneIndex: output.MilestoneIndex(),
		}
		copy(extended.OutputID, output.OutputID()[:])

		if senderBlock := features.SenderFeatureBlock(); senderBlock != nil {
			extended.Sender, err = addressBytesForAddress(senderBlock.Address)
			if err != nil {
				return err
			}
		}

		if tagBlock := features.IndexationFeatureBlock(); tagBlock != nil {
			copy(extended.Tag, tagBlock.Tag)
		}

		if dustReturn := features.DustDepositReturnFeatureBlock(); dustReturn != nil {
			extended.DustReturn = &dustReturn.Amount
		}

		if timelockMs := features.TimelockMilestoneIndexFeatureBlock(); timelockMs != nil {
			idx := milestone.Index(timelockMs.MilestoneIndex)
			extended.TimelockMilestone = &idx
		}

		if timelockTs := features.TimelockUnixFeatureBlock(); timelockTs != nil {
			time := time.Unix(int64(timelockTs.UnixTime), 0)
			extended.TimelockTime = &time
		}

		if expirationMs := features.ExpirationMilestoneIndexFeatureBlock(); expirationMs != nil {
			idx := milestone.Index(expirationMs.MilestoneIndex)
			extended.ExpirationMilestone = &idx
		}

		if expirationTs := features.ExpirationUnixFeatureBlock(); expirationTs != nil {
			time := time.Unix(int64(expirationTs.UnixTime), 0)
			extended.ExpirationTime = &time
		}
		if err := tx.Create(extended).Error; err != nil {
			return err
		}

	case *iotago.AliasOutput:
		aliasID := iotaOutput.AliasID
		if aliasID.Empty() {
			// Use implicit AliasID
			aliasID = iotago.AliasIDFromOutputID(*output.OutputID())
		}

		features, err := iotaOutput.FeatureBlocks().Set()
		if err != nil {
			return err
		}

		alias := &alias{
			AliasID:        make(aliasIDBytes, iotago.AliasIDLength),
			OutputID:       make(outputIDBytes, iotago.OutputIDLength),
			Amount:         iotaOutput.Amount,
			MilestoneIndex: output.MilestoneIndex(),
		}
		copy(alias.AliasID, aliasID[:])
		copy(alias.OutputID, output.OutputID()[:])

		alias.StateController, err = addressBytesForAddress(iotaOutput.StateController)
		if err != nil {
			return err
		}

		alias.GovernanceController, err = addressBytesForAddress(iotaOutput.GovernanceController)
		if err != nil {
			return err
		}

		if issuerBlock := features.IssuerFeatureBlock(); issuerBlock != nil {
			alias.Issuer, err = addressBytesForAddress(issuerBlock.Address)
			if err != nil {
				return err
			}
		}

		if senderBlock := features.SenderFeatureBlock(); senderBlock != nil {
			alias.Sender, err = addressBytesForAddress(senderBlock.Address)
			if err != nil {
				return err
			}
		}

		if err := tx.Create(alias).Error; err != nil {
			return err
		}

	case *iotago.NFTOutput:
		features, err := iotaOutput.FeatureBlocks().Set()
		if err != nil {
			return err
		}

		nftID := iotaOutput.NFTID
		if nftID.Empty() {
			// Use implicit NFTID
			nftAddr := iotago.NFTAddressFromOutputID(*output.OutputID())
			nftID = nftAddr.NFTID()
		}

		address, err := addressBytesForAddress(iotaOutput.Address)
		if err != nil {
			return err
		}
		nft := &nft{
			NFTID:          make(nftIDBytes, iotago.NFTIDLength),
			OutputID:       make(outputIDBytes, iotago.OutputIDLength),
			Address:        address,
			Amount:         iotaOutput.Amount,
			MilestoneIndex: output.MilestoneIndex(),
		}
		copy(nft.NFTID, nftID[:])
		copy(nft.OutputID, output.OutputID()[:])

		if issuerBlock := features.IssuerFeatureBlock(); issuerBlock != nil {
			nft.Issuer, err = addressBytesForAddress(issuerBlock.Address)
			if err != nil {
				return err
			}
		}

		if senderBlock := features.SenderFeatureBlock(); senderBlock != nil {
			nft.Sender, err = addressBytesForAddress(senderBlock.Address)
			if err != nil {
				return err
			}
		}

		if tagBlock := features.IndexationFeatureBlock(); tagBlock != nil {
			copy(nft.Tag, tagBlock.Tag)
		}

		if dustReturn := features.DustDepositReturnFeatureBlock(); dustReturn != nil {
			nft.DustReturn = &dustReturn.Amount
		}

		if timelockMs := features.TimelockMilestoneIndexFeatureBlock(); timelockMs != nil {
			idx := milestone.Index(timelockMs.MilestoneIndex)
			nft.TimelockMilestone = &idx
		}

		if timelockTs := features.TimelockUnixFeatureBlock(); timelockTs != nil {
			time := time.Unix(int64(timelockTs.UnixTime), 0)
			nft.TimelockTime = &time
		}

		if expirationMs := features.ExpirationMilestoneIndexFeatureBlock(); expirationMs != nil {
			idx := milestone.Index(expirationMs.MilestoneIndex)
			nft.ExpirationMilestone = &idx
		}

		if expirationTs := features.ExpirationUnixFeatureBlock(); expirationTs != nil {
			time := time.Unix(int64(expirationTs.UnixTime), 0)
			nft.ExpirationTime = &time
		}
		if err := tx.Create(nft).Error; err != nil {
			return err
		}

	case *iotago.FoundryOutput:
		foundryID, err := iotaOutput.ID()
		if err != nil {
			return err
		}

		address, err := addressBytesForAddress(iotaOutput.Address)
		if err != nil {
			return err
		}
		foundry := &foundry{
			FoundryID:      foundryID[:],
			OutputID:       make(outputIDBytes, iotago.OutputIDLength),
			Amount:         iotaOutput.Amount,
			Address:        address,
			MilestoneIndex: output.MilestoneIndex(),
		}
		copy(foundry.OutputID, output.OutputID()[:])

		if err := tx.Create(foundry).Error; err != nil {
			return err
		}

	default:
		panic("Unknown output type")
	}

	return nil
}

func (i *Indexer) ApplyConfirmation(msIndex milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) error {

	tx := i.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return err
	}

	for _, spent := range newSpents {
		if err := processSpent(spent, tx); err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, output := range newOutputs {
		if err := processOutput(output, tx); err != nil {
			tx.Rollback()
			return err
		}
	}

	// Update the ledger index
	status := &status{
		ID:          1,
		LedgerIndex: msIndex,
	}
	tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&status)

	return tx.Commit().Error
}

func (i *Indexer) ApplyWhiteflagConfirmation(confirmation *whiteflag.Confirmation) error {
	mutationOutputs := confirmation.Mutations.NewOutputs

	var newSpents utxo.Spents
	for spentID, spent := range confirmation.Mutations.NewSpents {
		newSpents = append(newSpents, spent)
		// We only care about the end-result of the confirmation, so outputs that were already spent in the same milestone can be ignored
		delete(mutationOutputs, spentID)
	}

	var newOutputs utxo.Outputs
	for _, output := range mutationOutputs {
		newOutputs = append(newOutputs, output)
	}

	return i.ApplyConfirmation(confirmation.MilestoneIndex, newOutputs, newSpents)
}

func (i *Indexer) LedgerIndex() (milestone.Index, error) {
	status := &status{}
	if err := i.db.Take(&status).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return status.LedgerIndex, nil
}

func (i *Indexer) Clear() error {
	// Drop all tables
	for _, table := range tables {
		if err := i.db.Migrator().DropTable(table); err != nil {
			return err
		}
	}
	// Re-create tables
	return i.db.AutoMigrate(tables...)
}

func (i *Indexer) CloseDatabase() error {
	sqlDB, err := i.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
