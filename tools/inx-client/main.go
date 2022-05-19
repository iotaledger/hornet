package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	inx "github.com/iotaledger/inx/go"
)

func main() {
	inxPort, err := utils.LoadStringFromEnvironment("INX_PORT")
	if err != nil {
		panic(err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%s", inxPort), opts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := inx.NewINXClient(conn)

	stream, err := client.ListenToConfirmedMilestone(context.Background(), &inx.NoParams{})
	if err != nil {
		panic(err)
	}
	for {
		milestone, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		printMilestoneCone(client, milestone)
	}
}

func printMilestoneCone(client inx.INXClient, milestone *inx.Milestone) {
	req := &inx.MilestoneRequest{
		MilestoneIndex: milestone.GetMilestoneInfo().GetMilestoneIndex(),
	}
	stream, err := client.ReadMilestoneCone(context.Background(), req)
	if err != nil {
		panic(err)
	}
	for {
		payload, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		block, err := payload.UnwrapBlock(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			panic(err)
		}
		jsonBlock, err := json.MarshalIndent(block, "", "  ")
		if err != nil {
			panic(err)
		}
		println(string(jsonBlock))
	}
}
