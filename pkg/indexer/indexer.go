package indexer

import (
	"github.com/pkg/errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"path/filepath"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
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
	case iotago.OutputBasic:
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
	case *iotago.BasicOutput:
		features, err := iotaOutput.FeatureBlocks().Set()
		if err != nil {
			return err
		}

		conditions, err := iotaOutput.UnlockConditions().Set()
		if err != nil {
			return err
		}

		extended := &extendedOutput{
			OutputID:  make(outputIDBytes, iotago.OutputIDLength),
			Amount:    iotaOutput.Amount,
			CreatedAt: unixTime(output.MilestoneTimestamp()),
		}
		copy(extended.OutputID, output.OutputID()[:])

		if senderBlock := features.SenderFeatureBlock(); senderBlock != nil {
			extended.Sender, err = addressBytesForAddress(senderBlock.Address)
			if err != nil {
				return err
			}
		}

		if tagBlock := features.TagFeatureBlock(); tagBlock != nil {
			extended.Tag = make([]byte, len(tagBlock.Tag))
			copy(extended.Tag, tagBlock.Tag)
		}

		if addressUnlock := conditions.Address(); addressUnlock != nil {
			extended.Address, err = addressBytesForAddress(addressUnlock.Address)
			if err != nil {
				return err
			}
		}

		if dustReturn := conditions.DustDepositReturn(); dustReturn != nil {
			extended.DustReturn = &dustReturn.Amount
			extended.DustReturnAddress, err = addressBytesForAddress(dustReturn.ReturnAddress)
			if err != nil {
				return err
			}
		}

		if timelock := conditions.Timelock(); timelock != nil {
			if timelock.MilestoneIndex > 0 {
				idx := milestone.Index(timelock.MilestoneIndex)
				extended.TimelockMilestone = &idx
			}
			if timelock.UnixTime > 0 {
				time := unixTime(timelock.UnixTime)
				extended.TimelockTime = &time
			}
		}

		if expiration := conditions.Expiration(); expiration != nil {
			if expiration.MilestoneIndex > 0 {
				idx := milestone.Index(expiration.MilestoneIndex)
				extended.ExpirationMilestone = &idx
			}
			if expiration.UnixTime > 0 {
				time := unixTime(expiration.UnixTime)
				extended.ExpirationTime = &time
			}
			extended.ExpirationReturnAddress, err = addressBytesForAddress(expiration.ReturnAddress)
			if err != nil {
				return err
			}
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

		conditions, err := iotaOutput.UnlockConditions().Set()
		if err != nil {
			return err
		}

		alias := &alias{
			AliasID:   make(aliasIDBytes, iotago.AliasIDLength),
			OutputID:  make(outputIDBytes, iotago.OutputIDLength),
			Amount:    iotaOutput.Amount,
			CreatedAt: unixTime(output.MilestoneTimestamp()),
		}
		copy(alias.AliasID, aliasID[:])
		copy(alias.OutputID, output.OutputID()[:])

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

		if stateController := conditions.StateControllerAddress(); stateController != nil {
			alias.StateController, err = addressBytesForAddress(stateController.Address)
			if err != nil {
				return err
			}
		}

		if governor := conditions.GovernorAddress(); governor != nil {
			alias.Governor, err = addressBytesForAddress(governor.Address)
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

		conditions, err := iotaOutput.UnlockConditions().Set()
		if err != nil {
			return err
		}

		nftID := iotaOutput.NFTID
		if nftID.Empty() {
			// Use implicit NFTID
			nftAddr := iotago.NFTAddressFromOutputID(*output.OutputID())
			nftID = nftAddr.NFTID()
		}

		nft := &nft{
			NFTID:     make(nftIDBytes, iotago.NFTIDLength),
			OutputID:  make(outputIDBytes, iotago.OutputIDLength),
			Amount:    iotaOutput.Amount,
			CreatedAt: unixTime(output.MilestoneTimestamp()),
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

		if tagBlock := features.TagFeatureBlock(); tagBlock != nil {
			nft.Tag = make([]byte, len(tagBlock.Tag))
			copy(nft.Tag, tagBlock.Tag)
		}

		if addressUnlock := conditions.Address(); addressUnlock != nil {
			nft.Address, err = addressBytesForAddress(addressUnlock.Address)
			if err != nil {
				return err
			}
		}

		if dustReturn := conditions.DustDepositReturn(); dustReturn != nil {
			nft.DustReturn = &dustReturn.Amount
			nft.DustReturnAddress, err = addressBytesForAddress(dustReturn.ReturnAddress)
			if err != nil {
				return err
			}
		}

		if timelock := conditions.Timelock(); timelock != nil {
			if timelock.MilestoneIndex > 0 {
				idx := milestone.Index(timelock.MilestoneIndex)
				nft.TimelockMilestone = &idx
			}
			if timelock.UnixTime > 0 {
				time := unixTime(timelock.UnixTime)
				nft.TimelockTime = &time
			}
		}

		if expiration := conditions.Expiration(); expiration != nil {
			if expiration.MilestoneIndex > 0 {
				idx := milestone.Index(expiration.MilestoneIndex)
				nft.ExpirationMilestone = &idx
			}
			if expiration.UnixTime > 0 {
				time := unixTime(expiration.UnixTime)
				nft.ExpirationTime = &time
			}
			nft.ExpirationReturnAddress, err = addressBytesForAddress(expiration.ReturnAddress)
			if err != nil {
				return err
			}
		}

		if err := tx.Create(nft).Error; err != nil {
			return err
		}

	case *iotago.FoundryOutput:
		conditions, err := iotaOutput.UnlockConditions().Set()
		if err != nil {
			return err
		}

		foundryID, err := iotaOutput.ID()
		if err != nil {
			return err
		}

		foundry := &foundry{
			FoundryID: foundryID[:],
			OutputID:  make(outputIDBytes, iotago.OutputIDLength),
			Amount:    iotaOutput.Amount,
			CreatedAt: unixTime(output.MilestoneTimestamp()),
		}
		copy(foundry.OutputID, output.OutputID()[:])

		if addressUnlock := conditions.Address(); addressUnlock != nil {
			foundry.Address, err = addressBytesForAddress(addressUnlock.Address)
			if err != nil {
				return err
			}
		}

		if err := tx.Create(foundry).Error; err != nil {
			return err
		}

	default:
		panic("Unknown output type")
	}

	return nil
}

func (i *Indexer) UpdatedLedger(msIndex milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) error {

	tx := i.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return err
	}

	spentOutputs := make(map[string]struct{})
	for _, spent := range newSpents {
		spentOutputs[string(spent.OutputID()[:])] = struct{}{}
		if err := processSpent(spent, tx); err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, output := range newOutputs {
		if _, wasSpentInSameMilestone := spentOutputs[string(output.OutputID()[:])]; wasSpentInSameMilestone {
			// We only care about the end-result of the confirmation, so outputs that were already spent in the same milestone can be ignored
			continue
		}
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
