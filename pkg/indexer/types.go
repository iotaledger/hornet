package indexer

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	NullOutputID = iotago.OutputID{}
)

type outputIDBytes []byte
type addressBytes []byte
type nftIDBytes []byte
type aliasIDBytes []byte
type foundryIDBytes []byte

type status struct {
	ID          uint `gorm:"primaryKey;not null"`
	LedgerIndex milestone.Index
}

type queryResult struct {
	OutputID       outputIDBytes
	MilestoneIndex milestone.Index
	LedgerIndex    milestone.Index
}

func (o outputIDBytes) ID() iotago.OutputID {
	id := iotago.OutputID{}
	copy(id[:], o)
	return id
}

type queryResults []queryResult

func (q queryResults) IDs() iotago.OutputIDs {
	outputIDs := iotago.OutputIDs{}
	for _, r := range q {
		outputIDs = append(outputIDs, r.OutputID.ID())
	}
	return outputIDs
}

func (a addressBytes) Address() (iotago.Address, error) {
	if len(a) != iotago.NFTAddressBytesLength || len(a) != iotago.AliasAddressBytesLength || len(a) != iotago.Ed25519AddressBytesLength {
		return nil, errors.New("invalid address length")
	}
	addr, err := iotago.AddressSelector(uint32(a[0]))
	if err != nil {
		return nil, err
	}
	_, err = addr.Deserialize(a, serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func addressBytesForAddress(addr iotago.Address) (addressBytes, error) {
	return addr.Serialize(serializer.DeSeriModeNoValidation, nil)
}

type IndexerResult struct {
	OutputIDs   iotago.OutputIDs
	LedgerIndex milestone.Index
	PageSize    int
	NextOffset  []byte
	Error       error
}

func errorResult(err error) *IndexerResult {
	return &IndexerResult{
		Error: err,
	}
}

func (i *Indexer) combineOutputIDFilteredQuery(query *gorm.DB, pageSize int, offset []byte) *IndexerResult {

	query = query.Select("output_id", "milestone_index").Order("milestone_index asc, output_id asc")
	if pageSize > 0 {
		if offset != nil {
			if len(offset) != 38 {
				return errorResult(errors.Errorf("Invalid offset length: %d", len(offset)))
			}
			msIndex := binary.LittleEndian.Uint32(offset[:4])
			query = query.Where("milestone_index >= ?", msIndex).Where("output_id >= hex(?)", hex.EncodeToString(offset[4:]))
		}
		query = query.Limit(pageSize + 1)
	}

	// This combines the query with a second query that checks for the current ledger_index.
	// This way we do not need to lock anything and we know the index matches the results.
	//TODO: measure performance for big datasets
	ledgerIndexQuery := i.db.Model(&status{}).Select("ledger_index")
	joinedQuery := i.db.Table("(?), (?)", query, ledgerIndexQuery)

	var results queryResults

	result := joinedQuery.Find(&results)
	if err := result.Error; err != nil {
		return errorResult(err)
	}

	ledgerIndex := milestone.Index(0)
	if len(results) > 0 {
		ledgerIndex = results[0].LedgerIndex
	}

	var nextOffset []byte
	if pageSize > 0 && len(results) > pageSize {
		lastResult := results[len(results)-1]
		results = results[:len(results)-1]
		msIndex := make([]byte, 4)
		binary.LittleEndian.PutUint32(msIndex, uint32(lastResult.MilestoneIndex))
		nextOffset = byteutils.ConcatBytes(msIndex, lastResult.OutputID[:])
	}

	return &IndexerResult{
		OutputIDs:   results.IDs(),
		LedgerIndex: ledgerIndex,
		PageSize:    pageSize,
		NextOffset:  nextOffset,
		Error:       nil,
	}
}
