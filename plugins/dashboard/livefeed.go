package dashboard

import (
	"time"

	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

func runLiveFeed() {

	newMessageNoValueRateLimiter := time.NewTicker(time.Second / 10)
	newMessageValueRateLimiter := time.NewTicker(time.Second / 20)

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *storage.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedMsg.ConsumeMessage(func(msg *storage.Message) {
			if !deps.Storage.IsNodeSyncedWithThreshold() {
				return
			}

			if !msg.IsTransaction() {
				select {
				case <-newMessageNoValueRateLimiter.C:
					hub.BroadcastMsg(&Msg{Type: MsgTypeTxZeroValue, Data: &LivefeedMessage{MessageID: msg.GetMessageID().Hex(), Value: 0}})
				default:
				}
			} else {
				select {
				case <-newMessageValueRateLimiter.C:
					// ToDo: Value
					hub.BroadcastMsg(&Msg{Type: MsgTypeTxValue, Data: &LivefeedMessage{MessageID: msg.GetMessageID().Hex(), Value: 0}})
				default:
				}
			}
		})
	})

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		if milestoneMessageID := getMilestoneMessageID(msIndex); milestoneMessageID != nil {
			hub.BroadcastMsg(&Msg{Type: MsgTypeMs, Data: &LivefeedMilestone{MessageID: milestoneMessageID.Hex(), Index: msIndex}})
		}
	})

	Plugin.Daemon().BackgroundWorker("Dashboard[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		tangle.Events.ReceivedNewMessage.Attach(onReceivedNewMessage)
		defer tangle.Events.ReceivedNewMessage.Detach(onReceivedNewMessage)
		tangle.Events.LatestMilestoneIndexChanged.Attach(onLatestMilestoneIndexChanged)
		defer tangle.Events.LatestMilestoneIndexChanged.Detach(onLatestMilestoneIndexChanged)

		<-shutdownSignal

		log.Info("Stopping Dashboard[TxUpdater] ...")
		newMessageNoValueRateLimiter.Stop()
		newMessageValueRateLimiter.Stop()
		log.Info("Stopping Dashboard[TxUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
