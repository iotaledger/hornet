package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/iotaledger/hive.go/serializer/v2"
)

const (
	INXPort = 9029
)

func main() {

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", INXPort), opts...)
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
