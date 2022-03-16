package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
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

	filter := &inx.MessageFilter{}
	stream, err := client.ListenToMessages(context.Background(), filter)
	if err != nil {
		panic(err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		msg := message.MustUnwrapMessage(serializer.DeSeriModeNoValidation)
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Rec: %s => %s\n", message.UnwrapMessageID().ToHex(), string(jsonMsg))
	}
}
