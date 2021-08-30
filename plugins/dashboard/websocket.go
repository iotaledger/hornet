package dashboard

import (
	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/websockethub"
)

const (
	// MsgTypeSyncStatus is the type of the SyncStatus message.
	MsgTypeSyncStatus byte = iota
	// MsgTypePublicNodeStatus is the type of the PublicNodeStatus message.
	MsgTypePublicNodeStatus = 1
	// MsgTypeNodeStatus is the type of the NodeStatus message.
	MsgTypeNodeStatus = 2
	// MsgTypeMPSMetric is the type of the messages per second (MPS) metric message.
	MsgTypeMPSMetric = 3
	// MsgTypeTipSelMetric is the type of the TipSelMetric message.
	MsgTypeTipSelMetric = 4
	// MsgTypeMs is the type of the Ms message.
	MsgTypeMs = 5
	// MsgTypePeerMetric is the type of the PeerMetric message.
	MsgTypePeerMetric = 6
	// MsgTypeConfirmedMsMetrics is the type of the ConfirmedMsMetrics message.
	MsgTypeConfirmedMsMetrics = 7
	// MsgTypeVertex is the type of the Vertex message for the visualizer.
	MsgTypeVertex = 8
	// MsgTypeSolidInfo is the type of the SolidInfo message for the visualizer.
	MsgTypeSolidInfo = 9
	// MsgTypeConfirmedInfo is the type of the ConfirmedInfo message for the visualizer.
	MsgTypeConfirmedInfo = 10
	// MsgTypeMilestoneInfo is the type of the MilestoneInfo message for the visualizer.
	MsgTypeMilestoneInfo = 11
	// MsgTypeTipInfo is the type of the TipInfo message for the visualizer.
	MsgTypeTipInfo = 12
	// MsgTypeDatabaseSizeMetric is the type of the database Size message for the metrics.
	MsgTypeDatabaseSizeMetric = 13
	// MsgTypeDatabaseCleanupEvent is the type of the database cleanup message for the metrics.
	MsgTypeDatabaseCleanupEvent = 14
	// MsgTypeSpamMetrics is the type of the SpamMetric message.
	MsgTypeSpamMetrics = 15
	// MsgTypeAvgSpamMetrics is the type of the AvgSpamMetric message.
	MsgTypeAvgSpamMetrics = 16
)

func websocketRoute(ctx echo.Context) error {
	defer func() {
		if r := recover(); r != nil {
			Plugin.LogErrorf("recovered from panic within WS handle func: %s", r)
		}
	}()

	publicTopics := []byte{
		MsgTypeSyncStatus,
		MsgTypePublicNodeStatus,
		MsgTypeMPSMetric,
		MsgTypeMs,
		MsgTypeConfirmedMsMetrics,
		MsgTypeVertex,
		MsgTypeSolidInfo,
		MsgTypeConfirmedInfo,
		MsgTypeMilestoneInfo,
		MsgTypeTipInfo,
	}

	isProtectedTopic := func(topic byte) bool {
		for _, publicTopic := range publicTopics {
			if topic == publicTopic {
				return false
			}
		}
		return true
	}

	// this function sends the initial values for some topics
	sendInitValue := func(client *websockethub.Client, initValuesSent map[byte]struct{}, topic byte) {
		if _, sent := initValuesSent[topic]; sent {
			return
		}
		initValuesSent[topic] = struct{}{}

		switch topic {
		case MsgTypeSyncStatus:
			client.Send(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})

		case MsgTypePublicNodeStatus:
			client.Send(&Msg{Type: MsgTypePublicNodeStatus, Data: currentPublicNodeStatus()})

		case MsgTypeNodeStatus:
			client.Send(&Msg{Type: MsgTypeNodeStatus, Data: currentNodeStatus()})

		case MsgTypeConfirmedMsMetrics:
			client.Send(&Msg{Type: MsgTypeConfirmedMsMetrics, Data: cachedMilestoneMetrics})

		case MsgTypeDatabaseSizeMetric:
			client.Send(&Msg{Type: MsgTypeDatabaseSizeMetric, Data: cachedDBSizeMetrics})

		case MsgTypeDatabaseCleanupEvent:
			client.Send(&Msg{Type: MsgTypeDatabaseCleanupEvent, Data: lastDBCleanup})

		case MsgTypeMs:
			start := deps.Storage.LatestMilestoneIndex()
			for i := start - 10; i <= start; i++ {
				if milestoneMessageID := getMilestoneMessageID(i); milestoneMessageID != nil {
					client.Send(&Msg{Type: MsgTypeMs, Data: &LivefeedMilestone{MessageID: milestoneMessageID.ToHex(), Index: i}})
				} else {
					break
				}
			}
		}
	}

	topicsLock := syncutils.RWMutex{}
	registeredTopics := make(map[byte]struct{})
	initValuesSent := make(map[byte]struct{})

	hub.ServeWebsocket(ctx.Response(), ctx.Request(),
		// onCreate gets called when the client is created
		func(client *websockethub.Client) {
			client.FilterCallback = func(_ *websockethub.Client, data interface{}) bool {
				msg, ok := data.(*Msg)
				if !ok {
					return false
				}

				topicsLock.RLock()
				_, registered := registeredTopics[msg.Type]
				topicsLock.RUnlock()
				return registered
			}
			client.ReceiveChan = make(chan *websockethub.WebsocketMsg, 100)

			go func() {
				for {
					select {
					case <-client.ExitSignal:
						// client was disconnected
						return

					case msg, ok := <-client.ReceiveChan:
						if !ok {
							// client was disconnected
							return
						}

						if msg.MsgType == websockethub.BinaryMessage {
							if len(msg.Data) < 2 {
								continue
							}

							cmd := msg.Data[0]
							topic := msg.Data[1]

							if cmd == WebsocketCmdRegister {

								if isProtectedTopic(topic) {
									// Check for the presence of a JWT and verify it
									if len(msg.Data) < 3 {
										// Dot not allow unsecure subscriptions to protected topics
										continue
									}
									token := string(msg.Data[2:])
									if !jwtAuth.VerifyJWT(token, func(claims *jwt.AuthClaims) bool {
										return claims.Dashboard
									}) {
										// Dot not allow unsecure subscriptions to protected topics
										continue
									}
								}

								// register topic fo this client
								topicsLock.Lock()
								registeredTopics[topic] = struct{}{}
								topicsLock.Unlock()

								sendInitValue(client, initValuesSent, topic)

							} else if cmd == WebsocketCmdUnregister {
								// unregister topic fo this client
								topicsLock.Lock()
								delete(registeredTopics, topic)
								topicsLock.Unlock()
							}
						}
					}
				}
			}()
		},

		// onConnect gets called when the client was registered
		func(_ *websockethub.Client) {
			Plugin.LogInfo("WebSocket client connection established")
		})

	return nil
}
