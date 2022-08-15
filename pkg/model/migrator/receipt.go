package migrator

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/iota.go/encoding/t5b1"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	receiptFilePattern = "%d.%d.json"
)

var (
	// ErrInvalidReceiptServiceState is returned when the state of the ReceiptService is invalid.
	ErrInvalidReceiptServiceState = errors.New("invalid receipt service state")
)

// ReceiptPersistFunc is a function which persists a receipt.
type ReceiptPersistFunc func(r *iotago.ReceiptMilestoneOpt) error

// ReceiptValidateFunc is a function which validates a receipt.
type ReceiptValidateFunc func(r *iotago.ReceiptMilestoneOpt) error

// ReceiptService is in charge of persisting and validating a batch of receipts.
type ReceiptService struct {
	// Whether the service is configured to back up receipts.
	BackupEnabled bool
	// Whether the service is configured to validate receipts.
	ValidationEnabled bool
	// Whether the service should ignore soft errors.
	IgnoreSoftErrors bool
	backupFolder     string
	validator        *Validator
	utxoManager      *utxo.Manager
}

// NewReceiptService creates a new ReceiptService.
func NewReceiptService(validator *Validator, utxoManager *utxo.Manager, validationEnabled bool, backupEnabled bool, ignoreSoftErrors bool, backupFolder string) *ReceiptService {
	return &ReceiptService{
		ValidationEnabled: validationEnabled,
		IgnoreSoftErrors:  ignoreSoftErrors,
		BackupEnabled:     backupEnabled,
		utxoManager:       utxoManager,
		validator:         validator,
		backupFolder:      backupFolder,
	}
}

// Init initializes the ReceiptService and returns the amount of receipts currently stored.
func (rs *ReceiptService) Init() error {
	if !rs.BackupEnabled {
		return nil
	}
	if err := os.MkdirAll(rs.backupFolder, os.ModePerm); err != nil {
		return err
	}

	return nil
}

// Backup backups the given receipt to disk.
func (rs *ReceiptService) Backup(r *utxo.ReceiptTuple) error {
	if !rs.BackupEnabled {
		panic("receipt service is not configured to backup receipts")
	}

	receiptFileName := path.Join(rs.backupFolder, fmt.Sprintf(receiptFilePattern, r.Receipt.MigratedAt, r.MilestoneIndex))
	receiptJSON, err := r.Receipt.MarshalJSON()
	if err != nil {
		return err
	}
	if err := os.WriteFile(receiptFileName, receiptJSON, os.ModePerm); err != nil {
		return common.CriticalError(fmt.Errorf("unable to persist receipt onto disk: %w", err))
	}

	return nil
}

// ValidateWithoutLocking validates the given receipt against data fetched from a legacy node.
// The UTXO ledger should be locked outside of this function.
// If the receipt has the final flag set to true, then the entire batch of receipts with the same migrated_at index
// are collected and it is checked whether they migrated all the funds of the given white-flag confirmation.
func (rs *ReceiptService) ValidateWithoutLocking(r *iotago.ReceiptMilestoneOpt) error {
	if !rs.ValidationEnabled {
		panic("receipt service is not configured to validate receipts")
	}

	highestMigratedAtIndex, err := rs.utxoManager.SearchHighestReceiptMigratedAtIndex(utxo.ReadLockLedger(false))
	if err != nil {
		return fmt.Errorf("unable to determine highest migrated at index: %w", err)
	}

	if r.MigratedAt < highestMigratedAtIndex {
		return fmt.Errorf("%w: current latest stored receipt has migrated at index %d but new receipt has index %d", ErrInvalidReceiptServiceState, highestMigratedAtIndex, r.MigratedAt)
	}

	return rs.validateAgainstWhiteFlagData(r)
}

func (rs *ReceiptService) validateAgainstWhiteFlagData(r *iotago.ReceiptMilestoneOpt) error {
	// validate
	wfEntries, err := rs.validator.QueryMigratedFunds(r.MigratedAt)
	if err != nil {
		return fmt.Errorf("unable to query migrated funds from legacy node for receipt validation: %w", err)
	}

	// we either simply check whether all the entries are contained within the legacy wf-conf
	// or if the receipt is final, check whether all funds have been migrated for the given index
	if r.Final {
		return rs.validateCompleteReceiptBatch(r, wfEntries)
	}

	return rs.validateNonFinalReceipt(r, wfEntries)
}

// validates the given non final receipt by checking whether the entries of migrated funds all exist
// within the given white-flag confirmation data.
func (rs *ReceiptService) validateNonFinalReceipt(r *iotago.ReceiptMilestoneOpt, wfEntries []*iotago.MigratedFundsEntry) error {
	if r.Final {
		panic("final receipt given")
	}

	receiptEntries := make(map[string]*iotago.MigratedFundsEntry)
	if err := addReceiptEntriesToMap(r, receiptEntries); err != nil {
		return err
	}

	if len(receiptEntries) > len(wfEntries) {
		return fmt.Errorf("%w: receipt has more entries than the wf-conf data", ErrInvalidReceiptServiceState)
	}

	wfEntriesMap := make(map[string]*iotago.MigratedFundsEntry)
	for _, wfEntry := range wfEntries {
		wfEntriesMap[string(wfEntry.TailTransactionHash[:])] = wfEntry
	}

	for _, receiptEntry := range receiptEntries {
		if err := compareAgainstEntries(wfEntriesMap, receiptEntry); err != nil {
			return err
		}
	}

	return nil
}

// adds the entries within the receipt to the given map by their tail tx hash.
// it returns an error in case an entry for a given tail tx already exists.
func addReceiptEntriesToMap(r *iotago.ReceiptMilestoneOpt, m map[string]*iotago.MigratedFundsEntry) error {
	for _, migFundEntry := range r.Funds {
		k := string(migFundEntry.TailTransactionHash[:])
		if _, has := m[k]; has {
			return fmt.Errorf("multiple receipts contain the same tail tx hash: %d/final(%v)", r.MigratedAt, r.Final)
		}
		m[k] = migFundEntry
	}

	return nil
}

// validates a complete batch of receipts for a given migrated_at index against the data retrieved from legacy nodes.
func (rs *ReceiptService) validateCompleteReceiptBatch(finalReceipt *iotago.ReceiptMilestoneOpt, wfEntries []*iotago.MigratedFundsEntry) error {
	receipts := []*iotago.ReceiptMilestoneOpt{finalReceipt}

	// collect migrated funds from previous receipt
	receiptsWithSameIndex := make([]*iotago.ReceiptMilestoneOpt, 0)
	if err := rs.utxoManager.ForEachReceiptTupleMigratedAt(finalReceipt.MigratedAt, func(rt *utxo.ReceiptTuple) bool {
		receiptsWithSameIndex = append(receiptsWithSameIndex, rt.Receipt)

		return true
	}, utxo.ReadLockLedger(false)); err != nil {
		return err
	}
	receipts = append(receipts, receiptsWithSameIndex...)

	receiptEntries := make(map[string]*iotago.MigratedFundsEntry)
	var finalCount int
	for _, r := range receipts {
		if len(r.Funds) == 0 {
			return fmt.Errorf("%w: receipt contains no migrated fund entries: %d/final(%v)", ErrInvalidReceiptServiceState, r.MigratedAt, r.Final)
		}
		if r.Final {
			finalCount++
		}
		if err := addReceiptEntriesToMap(r, receiptEntries); err != nil {
			return err
		}
	}

	switch {
	case finalCount == 0:
		// this should never happen
		return fmt.Errorf("%w: no final receipt within receipt batch %d/final(%v)", ErrInvalidReceiptServiceState, finalReceipt.MigratedAt, finalReceipt.Final)
	case finalCount > 1:
		return fmt.Errorf("%w: more than one (%d) final receipt within receipt batch %d", ErrInvalidReceiptServiceState, finalCount, finalReceipt.MigratedAt)
	}

	if len(wfEntries) != len(receiptEntries) {
		return fmt.Errorf("%w: mismatch between amount of entries: stored receipts have %d, wf-conf API call returns %d", ErrInvalidReceiptServiceState, len(receiptEntries), len(wfEntries))
	}

	// all white-flag conf entries must be within the receipts batch
	for _, wfEntry := range wfEntries {
		if err := compareAgainstEntries(receiptEntries, wfEntry); err != nil {
			return fmt.Errorf("failed receipt batch validation: %w", err)
		}
	}

	return nil
}

// compares the given target entry against an entry within the entries set.
// returns an error if the target entry is not within the entries set or if the entry within the set
// does not equal the target entry.
func compareAgainstEntries(entries map[string]*iotago.MigratedFundsEntry, targetEntry *iotago.MigratedFundsEntry) error {
	entry, has := entries[string(targetEntry.TailTransactionHash[:])]
	if !has {
		trytes, err := t5b1.DecodeToTrytes(targetEntry.TailTransactionHash[:])
		if err != nil {
			return fmt.Errorf("%w: non T5B1 tail tx hash within entry", ErrInvalidReceiptServiceState)
		}

		return fmt.Errorf("%w: target entry %s not in entries set", ErrInvalidReceiptServiceState, trytes)
	}

	entryBytes, err := entry.Serialize(serializer.DeSeriModePerformValidation, nil)
	if err != nil {
		return fmt.Errorf("unable to deserialize entry: %w", err)
	}

	targetEntryBytes, err := targetEntry.Serialize(serializer.DeSeriModePerformValidation, nil)
	if err != nil {
		return fmt.Errorf("unable to deserialize target entry: %w", err)
	}

	if !bytes.Equal(targetEntryBytes, entryBytes) {
		return fmt.Errorf("target entry does is not equal the entry within the set: %w", err)
	}

	return nil
}
