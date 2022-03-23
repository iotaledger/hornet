package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/mqtt"
	mqttpkg "github.com/gohornet/hornet/pkg/mqtt"
	inx "github.com/iotaledger/inx/go"
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
	Count      int
	CancelFunc func()
	Identifier int
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
		config.String(CfgMQTTBindAddress),
		config.Int(CfgMQTTWSPort),
		"/",
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
	switch topic {
	case topicMilestonesLatest:
		go s.fetchAndPublishMilestoneTopics(ctx)
		s.startListenIfNeeded(ctx, grpcListenToLatestMilestone, s.listenToLatestMilestone)
	case topicMilestonesConfirmed:
		go s.fetchAndPublishMilestoneTopics(ctx)
		s.startListenIfNeeded(ctx, grpcListenToConfirmedMilestone, s.listenToConfirmedMilestone)
	case topicMessages:
		s.startListenIfNeeded(ctx, grpcListenToMessages, s.listenToMessages)
	case topicReceipts:
		s.startListenIfNeeded(ctx, grpcListenToMigrationReceipts, s.listenToMigrationReceipts)
	default:
		if strings.HasPrefix(topic, "messages/") {
			if messageID := messageIDFromTopic(topic); messageID != nil {
				go s.fetchAndPublishMessageMetadata(ctx, *messageID)
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
}

func (s *Server) onUnsubscribeTopic(topic string) {
	switch topic {
	case topicMilestonesLatest:
		s.stopListenIfNeeded(grpcListenToLatestMilestone)
	case topicMilestonesConfirmed:
		s.stopListenIfNeeded(grpcListenToConfirmedMilestone)
	case topicMessages:
		s.stopListenIfNeeded(grpcListenToMessages)
	case topicReceipts:
		s.stopListenIfNeeded(grpcListenToMigrationReceipts)
	default:
		if strings.HasPrefix(topic, "messages/") {
			s.stopListenIfNeeded(grpcListenToSolidMessages)
			s.stopListenIfNeeded(grpcListenToReferencedMessages)
		} else if strings.HasPrefix(topic, "outputs/") || strings.HasPrefix(topic, "transactions/") {
			s.stopListenIfNeeded(grpcListenToLedgerUpdates)
		}
	}
}

func (s *Server) stopListenIfNeeded(grpcCall string) {
	s.grpcSubscriptionsLock.Lock()
	defer s.grpcSubscriptionsLock.Unlock()

	sub, ok := s.grpcSubscriptions[grpcCall]
	if ok {
		sub.Count--
		if sub.Count == 0 {
			sub.CancelFunc()
			delete(s.grpcSubscriptions, grpcCall)
		}
	}
}

func (s *Server) startListenIfNeeded(ctx context.Context, grpcCall string, listenFunc func(context.Context) error) {
	s.grpcSubscriptionsLock.Lock()
	defer s.grpcSubscriptionsLock.Unlock()

	sub, ok := s.grpcSubscriptions[grpcCall]
	if ok {
		sub.Count++
		return
	}

	c, cancel := context.WithCancel(ctx)
	subscriptionIdentifier := rand.Int()
	s.grpcSubscriptions[grpcCall] = &topicSubcription{
		Count:      1,
		CancelFunc: cancel,
		Identifier: subscriptionIdentifier,
	}
	go func() {
		fmt.Printf("Listen to %s\n", grpcCall)
		err := listenFunc(c)
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Printf("Finished listen to %s with error: %s\n", grpcCall, err.Error())
		} else {
			fmt.Printf("Finished listen to %s\n", grpcCall)
		}
		s.grpcSubscriptionsLock.Lock()
		sub, ok := s.grpcSubscriptions[grpcCall]
		if ok && sub.Identifier == subscriptionIdentifier {
			// Only delete if it was not already replaced by a new one.
			delete(s.grpcSubscriptions, grpcCall)
		}
		s.grpcSubscriptionsLock.Unlock()
	}()
}

func (s *Server) listenToLatestMilestone(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := s.Client.ListenToLatestMilestone(c, &inx.NoParams{})
	if err != nil {
		return err
	}
	for {
		milestone, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToLatestMilestone: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		s.PublishMilestoneOnTopic(topicMilestonesLatest, milestone)
	}
	return nil
}

func (s *Server) listenToConfirmedMilestone(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := s.Client.ListenToConfirmedMilestone(c, &inx.NoParams{})
	if err != nil {
		return err
	}
	for {
		milestone, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToConfirmedMilestone: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		s.PublishMilestoneOnTopic(topicMilestonesConfirmed, milestone)
	}
	return nil
}

func (s *Server) listenToMessages(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.MessageFilter{}
	stream, err := s.Client.ListenToMessages(c, filter)
	if err != nil {
		return err
	}
	for {
		message, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToMessages: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		s.PublishMessage(message.GetMessage())
	}
	return nil
}

func (s *Server) listenToSolidMessages(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.MessageFilter{}
	stream, err := s.Client.ListenToSolidMessages(c, filter)
	if err != nil {
		return err
	}
	for {
		messageMetadata, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToSolidMessages: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		s.PublishMessageMetadata(messageMetadata)
	}
	return nil
}

func (s *Server) listenToReferencedMessages(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.MessageFilter{}
	stream, err := s.Client.ListenToReferencedMessages(c, filter)
	if err != nil {
		return err
	}
	for {
		messageMetadata, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToReferencedMessages: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		s.PublishMessageMetadata(messageMetadata)
	}
	return nil
}

func (s *Server) listenToLedgerUpdates(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	filter := &inx.LedgerUpdateRequest{}
	stream, err := s.Client.ListenToLedgerUpdates(c, filter)
	if err != nil {
		return err
	}
	for {
		ledgerUpdate, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToLedgerUpdates: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		index := milestone.Index(ledgerUpdate.GetMilestoneIndex())
		created := ledgerUpdate.GetCreated()
		consumed := ledgerUpdate.GetConsumed()
		for _, o := range created {
			s.PublishOutput(index, o)
		}
		for _, o := range consumed {
			s.PublishSpent(index, o)
		}
	}
	return nil
}

func (s *Server) listenToMigrationReceipts(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := s.Client.ListenToMigrationReceipts(c, &inx.NoParams{})
	if err != nil {
		return err
	}
	for {
		receipt, err := stream.Recv()
		if err != nil {
			if err == io.EOF || status.Code(err) == codes.Canceled {
				break
			}
			fmt.Printf("listenToMigrationReceipts: %s\n", err.Error())
			break
		}
		if c.Err() != nil {
			break
		}
		s.PublishReceipt(receipt)
	}
	return nil
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

func (s *Server) fetchAndPublishMessageMetadata(ctx context.Context, messageID iotago.MessageID) {
	fmt.Printf("fetchAndPublishMessageMetadata: %s\n", iotago.MessageIDToHexString(messageID))
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

func (s *Server) fetchAndPublishTransactionInclusionWithMessage(ctx context.Context, transactionID *iotago.TransactionID, messageID iotago.MessageID) {
	resp, err := s.Client.ReadMessage(ctx, inx.NewMessageId(messageID))
	if err != nil {
		return
	}
	s.PublishTransactionIncludedMessage(transactionID, resp)
}
