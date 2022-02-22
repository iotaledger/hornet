package main

import (
	"context"
	"fmt"
	"io"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	indexerpkg "github.com/gohornet/hornet/pkg/indexer"
	indexer_server "github.com/gohornet/hornet/pkg/indexer/server"
	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	INXPort = 9029
)

func ConvertINXOutput(output *inx.LedgerOutput) (*utxo.Output, error) {
	outputID := output.UnwrapOutputID()
	messageID := output.UnwrapMessageID()
	milestoneIndex := milestone.Index(output.GetMilestoneIndex())
	milestoneTimestamp := uint64(output.GetMilestoneTimestamp())
	o, err := output.UnwrapOutput(serializer.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return utxo.CreateOutput(outputID, messageID, milestoneIndex, milestoneTimestamp, o), nil
}

func ConvertINXSpent(spent *inx.LedgerSpent) (*utxo.Spent, error) {
	output, err := ConvertINXOutput(spent.GetOutput())
	if err != nil {
		return nil, err
	}
	targetTransactionID := spent.UnwrapTargetTransactionID()
	milestoneIndex := milestone.Index(spent.GetSpentMilestoneIndex())
	milestoneTimestamp := uint64(spent.GetSpentMilestoneTimestamp())

	return utxo.NewSpent(output, targetTransactionID, milestoneIndex, milestoneTimestamp), nil
}

func ConvertINXOutputs(outputs []*inx.LedgerOutput) (utxo.Outputs, error) {
	var out utxo.Outputs
	for _, output := range outputs {
		o, err := ConvertINXOutput(output)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func ConvertINXSpents(spents []*inx.LedgerSpent) (utxo.Spents, error) {
	var sp utxo.Spents
	for _, spent := range spents {
		s, err := ConvertINXSpent(spent)
		if err != nil {
			return nil, err
		}
		sp = append(sp, s)
	}
	return sp, nil
}

func initializeIndexer(client inx.INXClient, indexer *indexerpkg.Indexer) error {
	// compare Indexer ledgerIndex with UTXO ledgerIndex and if it does not match, drop tables and import unspent outputs
	needsInitialImport := false

	if _, err := client.ReadLockLedger(context.Background(), &inx.NoParams{}); err != nil {
		return err
	}

	defer func() {
		client.ReadUnlockLedger(context.Background(), &inx.NoParams{})
	}()

	resp, err := client.LedgerStatus(context.Background(), &inx.NoParams{})
	if err != nil {
		panic(err)
	}
	utxoLedgerIndex := milestone.Index(resp.GetLedgerIndex())
	indexerLedgerIndex, err := indexer.LedgerIndex()
	if err != nil {
		if errors.Is(err, indexerpkg.ErrNotFound) {
			needsInitialImport = true
		} else {
			return err
		}
	} else {
		if utxoLedgerIndex != indexerLedgerIndex {
			fmt.Printf("Re-indexing UTXO ledger with index: %d\n", utxoLedgerIndex)
			indexer.Clear()
			needsInitialImport = true
		}
	}

	if needsInitialImport {
		importer := indexer.ImportTransaction()

		stream, err := client.ReadUnspentOutputs(context.Background(), &inx.NoParams{})
		if err != nil {
			panic(err)
		}
		for {
			message, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			iotaOutput, err := message.UnwrapOutput(serializer.DeSeriModeNoValidation)
			if err != nil {
				return err
			}
			output := utxo.CreateOutput(message.UnwrapOutputID(), message.UnwrapMessageID(), milestone.Index(message.GetMilestoneIndex()), uint64(message.GetMilestoneTimestamp()), iotaOutput)
			if err := importer.AddOutput(output); err != nil {
				return err
			}
		}
		if err := importer.Finalize(utxoLedgerIndex); err != nil {
			return err
		}
		fmt.Printf("Imported initial ledger at index %d\n", utxoLedgerIndex)
	}
	return nil
}

func listenToLedgerUpdates(ctx context.Context, client inx.INXClient, indexer *indexerpkg.Indexer) error {
	stream, err := client.ListenToLedgerUpdates(ctx, &inx.NoParams{})
	if err != nil {
		panic(err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		milestoneIndex := milestone.Index(message.GetMilestoneIndex())
		created, err := ConvertINXOutputs(message.GetCreated())
		if err != nil {
			return err
		}
		consumed, err := ConvertINXSpents(message.GetConsumed())
		if err != nil {
			return err
		}
		if err := indexer.UpdatedLedger(milestoneIndex, created, consumed); err != nil {
			return err
		}
		fmt.Printf("Updated ledgerIndex to %d with %d created and %d consumed outputs\n", milestoneIndex, len(created), len(consumed))
	}
	return nil
}

func main() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", INXPort), opts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := inx.NewINXClient(conn)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	indexer, err := indexerpkg.NewIndexer(".")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
	defer indexer.CloseDatabase()

	err = initializeIndexer(client, indexer)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	go listenToLedgerUpdates(ctx, client, indexer)

	indexer_server.NewIndexerServer(indexer, e.Group("indexer/v1"), iotago.PrefixTestnet, 1000)

	e.Start(":9090")
	cancel()
}
