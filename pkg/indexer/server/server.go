package indexer_server

import (
	"github.com/gohornet/hornet/pkg/indexer"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/labstack/echo/v4"
)

type IndexerServer struct {
	Indexer                 *indexer.Indexer
	Bech32HRP               iotago.NetworkPrefix
	RestAPILimitsMaxResults int
}

func NewIndexerServer(indexer *indexer.Indexer, group *echo.Group, prefix iotago.NetworkPrefix, maxPageSize int) *IndexerServer {
	s := &IndexerServer{
		Indexer:                 indexer,
		Bech32HRP:               prefix,
		RestAPILimitsMaxResults: maxPageSize,
	}
	s.configureRoutes(group)
	return s
}
