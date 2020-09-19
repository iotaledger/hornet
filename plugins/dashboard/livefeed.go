package dashboard

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"

	"github.com/gohornet/hornet/pkg/model/milestone"
	tanglemodel "github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/tangle"
)

func runLiveFeed() {

	newMessageNoValueRateLimiter := time.NewTicker(time.Second / 10)
	newMessageValueRateLimiter := time.NewTicker(time.Second / 20)

	onReceivedNewMessage := events.NewClosure(func(cachedMsg *tanglemodel.CachedMessage, latestMilestoneIndex milestone.Index, latestSolidMilestoneIndex milestone.Index) {
		cachedMsg.ConsumeMessage(func(msg *tanglemodel.Message) {
			if !tanglemodel.IsNodeSyncedWithThreshold() {
				return
			}

			if !msg.IsValue() {
				select {
				case <-newMessageNoValueRateLimiter.C:
					hub.BroadcastMsg(&Msg{Type: MsgTypeTxZeroValue, Data: &LivefeedTransaction{MessageID: msg.GetMessageID().Hex(), Value: 0}})
				default:
				}
			} else {
				select {
				case <-newMessageValueRateLimiter.C:
					// ToDo: Value
					hub.BroadcastMsg(&Msg{Type: MsgTypeTxValue, Data: &LivefeedTransaction{MessageID: msg.GetMessageID().Hex(), Value: 0}})
				default:
				}
			}
		})
	})

	onLatestMilestoneIndexChanged := events.NewClosure(func(msIndex milestone.Index) {
		if msTailTxHash := getMilestoneTailHash(msIndex); msTailTxHash != nil {
			hub.BroadcastMsg(&Msg{Type: MsgTypeMs, Data: &LivefeedMilestone{Hash: msTailTxHash.Hex(), Index: msIndex}})
		}
	})

	daemon.BackgroundWorker("Dashboard[TxUpdater]", func(shutdownSignal <-chan struct{}) {
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
