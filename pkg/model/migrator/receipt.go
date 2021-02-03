package migrator

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/iotaledger/iota.go/encoding/t5b1"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	receiptFilePattern = "%d.%d.bin"
)

var (
	// Returned when the state of the ReceiptService is invalid.
	ErrInvalidReceiptServiceState = errors.New("invalid receipt service state")
)

// ReceiptPersistFunc is a function which persists a receipt.
type ReceiptPersistFunc func(r *iotago.Receipt) error

// ReceiptValidateFunc is a function which validates a receipt.
type ReceiptValidateFunc func(r *iotago.Receipt) error

// ReceiptService is in charge of persisting and validating a batch of receipts.
type ReceiptService struct {
	// Whether the service is configured to validate receipts.
	ValidationEnabled bool
	validator         *Validator
	// the path under which the receipts are stored
	path string
}

// NewReceiptService creates a new ReceiptService.
func NewReceiptService(v *Validator, validationEnabled bool, receiptsPath string) *ReceiptService {
	return &ReceiptService{
		ValidationEnabled: validationEnabled,
		validator:         v,
		path:              receiptsPath,
	}
}

// Init initializes the ReceiptService and returns the amount of receipts currently stored.
func (rs *ReceiptService) Init() error {
	if err := os.MkdirAll(rs.path, 0666); err != nil {
		return err
	}
	return nil
}

// NumReceiptsStored returns the number of receipts stored.
func (rs *ReceiptService) NumReceiptsStored() (int, error) {
	fileInfos, err := rs.receiptFileInfos()
	if err != nil {
		return 0, err
	}

	var receiptsCount int
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		msIndex, _ := receiptFileNameToIndices(fileInfo.Name())
		if msIndex == notAReceiptIndex {
			continue
		}
		receiptsCount++
	}

	return receiptsCount, nil
}

// Receipts returns all stored receipts.
func (rs *ReceiptService) Receipts() ([]*iotago.Receipt, error) {
	return rs.receipts(nil, nil)
}

// ReceiptsByMigratedAtIndex returns the receipts for the given legacy milestone index.
func (rs *ReceiptService) ReceiptsByMigratedAtIndex(migratedAtIndex uint32) ([]*iotago.Receipt, error) {
	return rs.receipts(func(msIndex int) bool {
		return uint32(msIndex) == migratedAtIndex
	}, nil)
}

// receipts returns the receipts which pass the filters.
// filters can be nil to retrieve all receipts.
func (rs *ReceiptService) receipts(msIndexFilter func(msIndex int) bool, receiptFilter func(r *iotago.Receipt) bool) ([]*iotago.Receipt, error) {
	fileInfos, err := rs.receiptFileInfos()
	if err != nil {
		return nil, err
	}

	receipts := make([]*iotago.Receipt, 0)
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		msIndex, _ := receiptFileNameToIndices(fileInfo.Name())
		if msIndex == notAReceiptIndex {
			continue
		}

		if msIndexFilter != nil && !msIndexFilter(msIndex) {
			continue
		}

		receiptBytes, err := ioutil.ReadFile(path.Join(rs.path, fileInfo.Name()))
		if err != nil {
			return nil, fmt.Errorf("unable to read receipt '%s': %w", fileInfo.Name(), err)
		}

		r := &iotago.Receipt{}
		if _, err := r.Deserialize(receiptBytes, iotago.DeSeriModePerformValidation); err != nil {
			return nil, fmt.Errorf("invalid stored receipt '%s': %w", fileInfo.Name(), err)
		}

		if receiptFilter != nil && !receiptFilter(r) {
			continue
		}

		receipts = append(receipts, r)
	}
	return receipts, nil
}

// Store stores the given receipt.
func (rs *ReceiptService) store(r *iotago.Receipt, receiptIndex int) error {
	receiptFileName := path.Join(rs.path, fmt.Sprintf(receiptFilePattern, r.MigratedAt, receiptIndex))
	receiptBytes, err := r.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(receiptFileName, receiptBytes, 0666); err != nil {
		return fmt.Errorf("unable to persist receipt onto disk: %w", err)
	}
	return nil
}

const notAReceiptIndex = -1

func receiptFileNameToIndices(fileName string) (msIndex int, receiptIndex int) {
	split := strings.Split(fileName, ".")
	if len(split) != 3 {
		return notAReceiptIndex, notAReceiptIndex
	}

	var err error
	msIndex, err = strconv.Atoi(split[0])
	if err != nil {
		return notAReceiptIndex, notAReceiptIndex
	}

	receiptIndex, err = strconv.Atoi(split[1])
	if err != nil {
		return notAReceiptIndex, notAReceiptIndex
	}

	return msIndex, receiptIndex
}

// retrieves the file infos from files within the receipts folder.
func (rs *ReceiptService) receiptFileInfos() ([]os.FileInfo, error) {
	fileInfos, err := ioutil.ReadDir(rs.path)
	if err != nil {
		return nil, fmt.Errorf("unable to query files within receipt folder: %w", err)
	}
	return fileInfos, nil
}

// returns the latest legacy milestone index and the index of receipt.
func (rs *ReceiptService) latestIndex() (msIndex int, receiptIndex int, err error) {
	fileInfos, err := rs.receiptFileInfos()
	if err != nil {
		return 0, 0, err
	}

	var hMsIndex, hReceiptIndex int
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		msIndex, receiptIndex := receiptFileNameToIndices(fileInfo.Name())
		if msIndex == notAReceiptIndex {
			continue
		}

		if msIndex > hMsIndex {
			hMsIndex = msIndex
			hReceiptIndex = 0
			continue
		}

		if msIndex == hMsIndex && receiptIndex > hReceiptIndex {
			hReceiptIndex = receiptIndex
		}
	}

	return hMsIndex, hReceiptIndex, nil
}

// Store stores the given receipt to disk.
func (rs *ReceiptService) Store(r *iotago.Receipt) error {
	hMsIndex, hReceiptIndex, err := rs.latestIndex()
	if err != nil {
		return fmt.Errorf("unable to determine latest receipt: %w", err)
	}

	var receiptIndex int
	if int(r.MigratedAt) == hMsIndex {
		receiptIndex = hReceiptIndex + 1
	}

	return rs.store(r, receiptIndex)
}

// Validate validates the given receipt against data fetched from a legacy node.
// If the receipt has the final flag set to true, then the entire batch of receipts with the same migrated_at index
// are collected and it is checked whether they migrated all the funds of the given white-flag confirmation.
func (rs *ReceiptService) Validate(r *iotago.Receipt) error {
	if !rs.ValidationEnabled {
		panic("receipt service is not configured to validate receipts")
	}

	hMsIndex, _, err := rs.latestIndex()
	if err != nil {
		return fmt.Errorf("unable to determine latest receipt: %w", err)
	}

	hMsIndexUint32 := uint32(hMsIndex)
	if r.MigratedAt < hMsIndexUint32 {
		return fmt.Errorf("%w: current latest stored receipt has milestone index %d but new receipt has index %d", ErrInvalidReceiptServiceState, hMsIndex, r.MigratedAt)
	}

	return rs.validateAgainstWhiteFlagData(r)
}

func (rs *ReceiptService) validateAgainstWhiteFlagData(r *iotago.Receipt) error {
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
func (rs *ReceiptService) validateNonFinalReceipt(r *iotago.Receipt, wfEntries []*iotago.MigratedFundsEntry) error {
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
func addReceiptEntriesToMap(r *iotago.Receipt, m map[string]*iotago.MigratedFundsEntry) error {
	for _, seri := range r.Funds {
		migFundEntry := seri.(*iotago.MigratedFundsEntry)
		k := string(migFundEntry.TailTransactionHash[:])
		if _, has := m[k]; has {
			return fmt.Errorf("multiple receipts contain the same tail tx hash: %d/final(%v)", r.MigratedAt, r.Final)
		}
		m[k] = migFundEntry
	}
	return nil
}

// validates a complete batch of receipts for a given migrated_at index against the data retrieved from legacy nodes.
func (rs *ReceiptService) validateCompleteReceiptBatch(finalReceipt *iotago.Receipt, wfEntries []*iotago.MigratedFundsEntry) error {
	receipts := []*iotago.Receipt{finalReceipt}

	// collect migrated funds from previous receipt
	receiptsWithSameIndex, err := rs.ReceiptsByMigratedAtIndex(finalReceipt.MigratedAt)
	if err != nil {
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

	entryBytes, err := entry.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return fmt.Errorf("unable to deserialize entry: %w", err)
	}

	targetEntryBytes, err := targetEntry.Serialize(iotago.DeSeriModePerformValidation)
	if err != nil {
		return fmt.Errorf("unable to deserialize target entry: %w", err)
	}

	if !bytes.Equal(targetEntryBytes, entryBytes) {
		return fmt.Errorf("target entry does is not equal the entry within the set: %w", err)
	}

	return nil
}
