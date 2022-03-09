package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	indexerpkg "github.com/gohornet/hornet/pkg/indexer"
	indexer_server "github.com/gohornet/hornet/pkg/indexer/server"
	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	APIRoute = "inx-indexer/v1"
)

func ConvertINXOutput(output *inx.LedgerOutput) (*utxo.Output, error) {
	outputID := output.UnwrapOutputID()
	messageID := output.UnwrapMessageID()
	milestoneIndex := milestone.Index(output.GetMilestoneIndexBooked())
	milestoneTimestamp := uint64(output.GetMilestoneTimestampBooked())
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
	targetTransactionID := spent.UnwrapTransactionIDSpent()
	milestoneIndex := milestone.Index(spent.GetMilestoneIndexSpent())
	milestoneTimestamp := uint64(spent.GetMilestoneTimestampSpent())

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

func fillIndexer(client inx.INXClient, indexer *indexerpkg.Indexer) error {
	importer := indexer.ImportTransaction()
	stream, err := client.ReadUnspentOutputs(context.Background(), &inx.NoParams{})
	if err != nil {
		panic(err)
	}
	var ledgerIndex milestone.Index
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		ledgerOutput := message.GetOutput()
		iotaOutput, err := ledgerOutput.UnwrapOutput(serializer.DeSeriModeNoValidation)
		if err != nil {
			return err
		}
		outputLedgerIndex := milestone.Index(message.GetLedgerIndex())
		output := utxo.CreateOutput(ledgerOutput.UnwrapOutputID(), ledgerOutput.UnwrapMessageID(), milestone.Index(ledgerOutput.GetMilestoneIndexBooked()), uint64(ledgerOutput.GetMilestoneTimestampBooked()), iotaOutput)
		if err := importer.AddOutput(output); err != nil {
			return err
		}
		if outputLedgerIndex > ledgerIndex {
			ledgerIndex = outputLedgerIndex
		}
	}
	if err := importer.Finalize(ledgerIndex); err != nil {
		return err
	}
	fmt.Printf("Imported initial ledger at index %d\n", ledgerIndex)
	return nil
}

func listenToLedgerUpdates(ctx context.Context, client inx.INXClient, indexer *indexerpkg.Indexer) error {
	ledgerIndex, err := indexer.LedgerIndex()
	if err != nil {
		return err
	}
	req := &inx.LedgerUpdateRequest{
		StartMilestoneIndex: uint32(ledgerIndex + 1),
	}
	stream, err := client.ListenToLedgerUpdates(ctx, req)
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

	port, err := utils.LoadStringFromEnvironment("INX_PORT")
	if err != nil {
		panic(err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             1 * time.Second,
		PermitWithoutStream: true,
	}))
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%s", port), opts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := inx.NewINXClient(conn)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	go func() {
		if err := e.Start(":0"); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}
	}()

	indexer, err := indexerpkg.NewIndexer(".")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer indexer.CloseDatabase()

	ledgerIndex, err := indexer.LedgerIndex()
	if err != nil {
		if errors.Is(err, indexerpkg.ErrNotFound) {
			// Indexer is empty, so import initial ledger state from the node
			fmt.Println("Indexer is empty, so import initial ledger")
			if err := fillIndexer(client, indexer); err != nil {
				fmt.Printf("Error: %s\n", err)
				return
			}
		} else {
			fmt.Printf("Error: %s\n", err)
			return
		}
	} else {
		fmt.Printf("Indexer started at ledgerIndex %d\n", ledgerIndex)
	}

	resp, err := client.ReadNodeStatus(context.Background(), &inx.NoParams{})
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	if milestone.Index(resp.GetPruningIndex()) > ledgerIndex {
		fmt.Println("Node has an newer pruning index than our current ledgerIndex, so re-import initial ledger")
		if err := indexer.Clear(); err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
		if err := fillIndexer(client, indexer); err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := listenToLedgerUpdates(ctx, client, indexer); err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		cancel()
	}()
	indexer_server.NewIndexerServer(indexer, e.Group(APIRoute), iotago.PrefixTestnet, 1000)

	apiReq := &inx.APIRouteRequest{
		Route: APIRoute,
		Host:  "localhost",
		Port:  uint32(e.Listener.Addr().(*net.TCPAddr).Port),
	}
	fmt.Println("Registering API route")
	if _, err := client.RegisterAPIRoute(context.Background(), apiReq); err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	go func() {
		<-signalChan
		done <- true
	}()
	go func() {
		select {
		case <-signalChan:
			done <- true
		case <-ctx.Done():
			done <- true
		}
	}()
	<-done
	cancel()
	e.Close()
	fmt.Println("Removing API route")
	if _, err := client.UnregisterAPIRoute(context.Background(), apiReq); err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	fmt.Println("exiting")
}
