package indexer

import (
	"encoding/binary"
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	OffsetLength = 38
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
	ID          uint `gorm:"primaryKey;notnull"`
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
			if len(offset) != OffsetLength {
				return errorResult(errors.Errorf("Invalid offset length: %d", len(offset)))
			}
			msIndex := binary.LittleEndian.Uint32(offset[:4])
			query = query.Select("output_id", "milestone_index", "hex(output_id) AS hex_output_id").Where("milestone_index >= ?", msIndex).Where("hex_output_id >= ?", strings.ToUpper(hex.EncodeToString(offset[4:])))
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
