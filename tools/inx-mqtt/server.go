package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/mqtt"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	grpcListenToLatestMilestone    = "INX.ListenToLatestMilestone"
	grpcListenToConfirmedMilestone = "INX.ListenToConfirmedMilestone"
	grpcListenToMessages           = "INX.ListenToMessages"
	grpcListenToSolidMessages      = "INX.ListenToSolidMessages"
	grpcListenToReferencedMessages = "INX.ListenToReferencedMessages"
	grpcListenToLedgerUpdates      = "INX.ListenToLedgerUpdates"
	grpcListenToMigrationReceipts  = "INX.ListenToMigrationReceipts"
)

type topicSubcription struct {
	Count int
	Func  func()
}

type Server struct {
	MQTTBroker     *mqttpkg.Broker
	Client         inx.INXClient
	ProtocolParams *inx.ProtocolParameters

	grpcSubscriptionsLock sync.Mutex
	grpcSubscriptions     map[string]*topicSubcription
}

func NewServer(client inx.INXClient) (*Server, error) {

	protocolParams, err := client.ReadProtocolParameters(context.Background(), &inx.NoParams{})
	if err != nil {
		return nil, err
	}

	s := &Server{
		Client:            client,
		ProtocolParams:    protocolParams,
		grpcSubscriptions: make(map[string]*topicSubcription),
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {

	broker, err := mqtt.NewBroker(
		MQTTBindAddress,
		MQTTWSPort,
		MQTTWSPath,
		100,
		func(topic []byte) {
			s.onSubscribeTopic(ctx, string(topic))
		}, func(topic []byte) {
			s.onUnsubscribeTopic(string(topic))
		},
		10000)
	if err != nil {
		return err
	}

	s.MQTTBroker = broker
	broker.Start()

	return nil
}

func (s *Server) onSubscribeTopic(ctx context.Context, topic string) {
	if topic == topicMilestonesLatest {
		go s.fetchAndPublishMilestoneTopics(ctx)
		s.startListenIfNeeded(ctx, grpcListenToLatestMilestone, s.listenToLatestMilestone)
	} else if topic == topicMilestonesConfirmed {
		go s.fetchAndPublishMilestoneTopics(ctx)
		s.startListenIfNeeded(ctx, grpcListenToConfirmedMilestone, s.listenToConfirmedMilestone)
	} else if topic == topicMessages {
		s.startListenIfNeeded(ctx, grpcListenToMessages, s.listenToMessages)
	} else if strings.HasPrefix(topic, "messages/") {
		if messageID := messageIDFromTopic(topic); messageID != nil {
			go s.fetchAndPublishMessageMetadata(ctx, messageID)
		}
		s.startListenIfNeeded(ctx, grpcListenToSolidMessages, s.listenToSolidMessages)
		s.startListenIfNeeded(ctx, grpcListenToReferencedMessages, s.listenToReferencedMessages)
	} else if strings.HasPrefix(topic, "outputs/") || strings.HasPrefix(topic, "transactions/") {
		if transactionID := transactionIDFromTopic(topic); transactionID != nil {
			go s.fetchAndPublishTransactionInclusion(ctx, transactionID)
		}
		if outputID := outputIDFromTopic(topic); outputID != nil {
			go s.fetchAndPublishOutput(ctx, outputID)
		}
		s.startListenIfNeeded(ctx, grpcListenToLedgerUpdates, s.listenToLedgerUpdates)
	}
}

func (s *Server) onUnsubscribeTopic(topic string) {
	if topic == topicMilestonesLatest {
		s.stopListenIfNeeded(grpcListenToLatestMilestone)
	} else if topic == topicMilestonesConfirmed {
		s.stopListenIfNeeded(grpcListenToConfirmedMilestone)
	} else if topic == topicMessages {
		s.stopListenIfNeeded(grpcListenToMessages)
	} else if strings.HasPrefix(topic, "messages/") {
		s.stopListenIfNeeded(grpcListenToSolidMessages)
		s.stopListenIfNeeded(grpcListenToReferencedMessages)
	} else if strings.HasPrefix(topic, "outputs/") || strings.HasPrefix(topic, "transactions/") {
		s.stopListenIfNeeded(grpcListenToLedgerUpdates)
	}
}

func (s *Server) stopListenIfNeeded(identifier string) {
	s.grpcSubscriptionsLock.Lock()
	defer s.grpcSubscriptionsLock.Unlock()

	sub, ok := s.grpcSubscriptions[identifier]
	if ok {
		if sub.Count <= 1 {
			sub.Func()
			delete(s.grpcSubscriptions, identifier)
		} else {
			sub.Count--
		}
	}
}

func (s *Server) startListenIfNeeded(ctx context.Context, identifier string, listenFunc func(context.Context)) {
	s.grpcSubscriptionsLock.Lock()
	defer s.grpcSubscriptionsLock.Unlock()

	sub, ok := s.grpcSubscriptions[identifier]
	if !ok {
		c, cancel := context.WithCancel(ctx)
		go func() {
			fmt.Printf("Listen to %s\n", identifier)
			listenFunc(c)
			s.grpcSubscriptionsLock.Lock()
			fmt.Printf("Finished listen to %s\n", identifier)
			delete(s.grpcSubscriptions, identifier)
			s.grpcSubscriptionsLock.Unlock()
		}()
		s.grpcSubscriptions[identifier] = &topicSubcription{
			Count: 1,
			Func:  cancel,
		}
	} else {
		sub.Count++
	}
}

func (s *Server) listenToLatestMilestone(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := s.Client.ListenToLatestMilestone(c, &inx.NoParams{})
	if err != nil {
		panic(err)
	}
	for {
		milestone, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			break
		}
		s.PublishMilestoneOnTopic(topicMilestonesLatest, milestone)
	}
}

func (s *Server) listenToConfirmedMilestone(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := s.Client.ListenToConfirmedMilestone(c, &inx.NoParams{})
	if err != nil {
		panic(err)
	}
	for {
		milestone, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			break
		}
		s.PublishMilestoneOnTopic(topicMilestonesConfirmed, milestone)
	}
}

func (s *Server) listenToMessages(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.MessageFilter{}
	stream, err := s.Client.ListenToMessages(c, filter)
	if err != nil {
		panic(err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			break
		}
		s.PublishMessage(message.GetMessage())
	}
	cancel()
}

func (s *Server) listenToSolidMessages(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.MessageFilter{}
	stream, err := s.Client.ListenToSolidMessages(c, filter)
	if err != nil {
		panic(err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			break
		}
		s.PublishMessageMetadata(message)
	}
}

func (s *Server) listenToReferencedMessages(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.MessageFilter{}
	stream, err := s.Client.ListenToReferencedMessages(c, filter)
	if err != nil {
		panic(err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			break
		}
		s.PublishMessageMetadata(message)
	}
}

func (s *Server) listenToLedgerUpdates(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.LedgerUpdateRequest{}
	stream, err := s.Client.ListenToLedgerUpdates(c, filter)
	if err != nil {
		panic(err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err.Error())
			cancel()
			break
		}
		index := milestone.Index(message.GetMilestoneIndex())
		created := message.GetCreated()
		consumed := message.GetConsumed()
		for _, o := range created {
			s.PublishOutput(index, o)
		}
		for _, o := range consumed {
			s.PublishSpent(index, o)
		}
	}
}

func (s *Server) fetchAndPublishMilestoneTopics(ctx context.Context) {
	fmt.Println("fetchAndPublishMilestoneTopics")
	resp, err := s.Client.ReadNodeStatus(ctx, &inx.NoParams{})
	if err != nil {
		return
	}
	s.PublishMilestoneOnTopic(topicMilestonesLatest, resp.GetLatestMilestone())
	s.PublishMilestoneOnTopic(topicMilestonesConfirmed, resp.GetConfirmedMilestone())
}

func (s *Server) fetchAndPublishMessageMetadata(ctx context.Context, messageID hornet.MessageID) {
	fmt.Printf("fetchAndPublishMessageMetadata: %s\n", messageID.ToHex())
	resp, err := s.Client.ReadMessageMetadata(ctx, inx.NewMessageId(messageID))
	if err != nil {
		return
	}
	s.PublishMessageMetadata(resp)
}

func (s *Server) fetchAndPublishOutput(ctx context.Context, outputID *iotago.OutputID) {
	fmt.Printf("fetchAndPublishOutput: %s\n", outputID.ToHex())
	resp, err := s.Client.ReadOutput(ctx, inx.NewOutputId(outputID))
	if err != nil {
		return
	}
	s.PublishOutput(milestone.Index(resp.GetLedgerIndex()), resp.GetOutput())
}

func (s *Server) fetchAndPublishTransactionInclusion(ctx context.Context, transactionID *iotago.TransactionID) {
	fmt.Printf("fetchAndPublishTransactionInclusion: %s\n", transactionID.ToHex())
	outputID := &iotago.OutputID{}
	copy(outputID[:], transactionID[:])

	resp, err := s.Client.ReadOutput(ctx, inx.NewOutputId(outputID))
	if err != nil {
		return
	}
	s.fetchAndPublishTransactionInclusionWithMessage(ctx, transactionID, resp.GetOutput().UnwrapMessageID())
}

func (s *Server) fetchAndPublishTransactionInclusionWithMessage(ctx context.Context, transactionID *iotago.TransactionID, messageID hornet.MessageID) {
	resp, err := s.Client.ReadMessage(ctx, inx.NewMessageId(messageID))
	if err != nil {
		return
	}
	s.PublishTransactionIncludedMessage(transactionID, resp)
}
