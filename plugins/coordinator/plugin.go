package coordinator

import (
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/plugins/urts"
	"github.com/iotaledger/hive.go/configuration"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"

	tanglecore "github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/mselection"
	"github.com/gohornet/hornet/pkg/model/tangle"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

func init() {
	flag.CommandLine.MarkHidden("cooBootstrap")
	flag.CommandLine.MarkHidden("cooStartIndex")
	Plugin = &node.Plugin{
		Status: node.Disabled,
		Pluggable: node.Pluggable{
			Name:      "Coordinator",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	log    *logger.Logger

	bootstrap  = flag.Bool("cooBootstrap", false, "bootstrap the network")
	startIndex = flag.Uint32("cooStartIndex", 0, "index of the first milestone at bootstrap")

	maxTrackedMessages int
	belowMaxDepth      milestone.Index

	nextCheckpointSignal chan struct{}
	nextMilestoneSignal  chan struct{}

	coo      *coordinator.Coordinator
	selector *mselection.HeaviestSelector

	lastCheckpointIndex     int
	lastCheckpointMessageID *hornet.MessageID
	lastMilestoneMessageID  *hornet.MessageID

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure
	onIssuedCheckpoint   *events.Closure
	onIssuedMilestone    *events.Closure

	ErrDatabaseTainted = errors.New("database is tainted. delete the coordinator database and start again with a local snapshot")

	deps dependencies
)

type dependencies struct {
	dig.In
	Tangle           *tangle.Tangle
	PoWHandler       *powpackage.Handler
	MessageProcessor *gossip.MessageProcessor
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	// set the node as synced at startup, so the coo plugin can select tips
	tanglecore.SetUpdateSyncedAtStartup(true)

	var err error
	coo, err = initCoordinator(*bootstrap, *startIndex, deps.PoWHandler)
	if err != nil {
		log.Panic(err)
	}

	configureEvents()
}

func initCoordinator(bootstrap bool, startIndex uint32, powHandler *powpackage.Handler) (*coordinator.Coordinator, error) {

	if deps.Tangle.IsDatabaseTainted() {
		return nil, ErrDatabaseTainted
	}

	privateKeys, err := utils.LoadEd25519PrivateKeysFromEnvironment("COO_PRV_KEYS")
	if err != nil {
		return nil, err
	}

	// the last trit of the seed will be ignored, so it is important security information when that happens
	lastTrits := trinary.MustTrytesToTrits(string(seed[consts.HashTrytesSize-1]))
	if lastTrits[consts.TritsPerTryte-1] != 0 {
		// print warning and set the 243rd trit to zero for consistency and to prevent warnings during key derivation
		log.Warn("The trit at index 243 of the coordinator seed is non-zero. " +
			"The value of this trit will be ignored by the key derivation.")
		lastTrits[consts.TritsPerTryte-1] = 0
		seed = seed[:consts.HashTrytesSize-1] + trinary.MustTritsToTrytes(lastTrits)
	}

	// use the heaviest branch tip selection for the milestones
	selector = mselection.New(
		deps.NodeConfig.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold),
		deps.NodeConfig.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint),
		deps.NodeConfig.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint),
		time.Duration(deps.NodeConfig.Int(CfgCoordinatorTipselectHeaviestBranchSelectionDeadlineMilliseconds))*time.Millisecond,
	)

	nextCheckpointSignal = make(chan struct{})

	// must be a buffered channel, otherwise signal gets
	// lost if checkpoint is generated at the same time
	nextMilestoneSignal = make(chan struct{}, 1)

	maxTrackedMessages = deps.NodeConfig.Int(CfgCoordinatorCheckpointsMaxTrackedMessages)

	belowMaxDepth = milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelBelowMaxDepth))

	if len(privateKeys) == 0 {
		return nil, errors.New("no private keys given")
	}

	for _, privateKey := range privateKeys {
		if len(privateKey) != ed25519.PrivateKeySize {
			return nil, errors.New("wrong private key length")
		}
	}

	inMemoryEd25519MilestoneSignerProvider := coordinator.NewInMemoryEd25519MilestoneSignerProvider(privateKeys, deps.Tangle.KeyManager(), deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount))

	coo, err := coordinator.New(
		deps.Tangle,
		inMemoryEd25519MilestoneSignerProvider,
		deps.NodeConfig.String(CfgCoordinatorStateFilePath),
		deps.NodeConfig.Int(CfgCoordinatorIntervalSeconds),
		powHandler,
		sendMessage,
		coordinator.MilestoneMerkleTreeHashFuncWithName(deps.NodeConfig.String(protocfg.CfgProtocolMilestoneMerkleTreeHashFunc)),
	)
	if err != nil {
		return nil, err
	}

	if err := coo.InitState(bootstrap, milestone.Index(startIndex)); err != nil {
		return nil, err
	}

	// don't issue milestones or checkpoints in case the node is running hot
	coo.AddBackPressureFunc(tangleplugin.IsReceiveTxWorkerPoolBusy)

	return coo, nil
}

func run() {

	// create a background worker that signals to issue new milestones
	Plugin.Daemon().BackgroundWorker("Coordinator[MilestoneTicker]", func(shutdownSignal <-chan struct{}) {

		timeutil.Ticker(func() {
			// issue next milestone
			select {
			case nextMilestoneSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}, coo.GetInterval(), shutdownSignal)

	}, shutdown.PriorityCoordinator)

	// create a background worker that issues milestones
	Plugin.Daemon().BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
		// wait until all background workers of the tangle plugin are started
		tanglecore.WaitForTangleProcessorStartup()

		attachEvents()

		// bootstrap the network if not done yet
		milestoneMessageID, criticalErr := coo.Bootstrap()
		if criticalErr != nil {
			log.Panic(criticalErr)
		}

		// init the last milestone message ID
		lastMilestoneMessageID = milestoneMessageID

		// init the checkpoints
		lastCheckpointMessageID = milestoneMessageID
		lastCheckpointIndex = 0

	coordinatorLoop:
		for {
			select {
			case <-nextCheckpointSignal:
				// check the thresholds again, because a new milestone could have been issued in the meantime
				if trackedMessagesCount := selector.GetTrackedMessagesCount(); trackedMessagesCount < maxTrackedMessages {
					continue
				}

				tips, err := selector.SelectTips(0)
				if err != nil {
					// issuing checkpoint failed => not critical
					if err != mselection.ErrNoTipsAvailable {
						log.Warn(err)
					}
					continue
				}

				// issue a checkpoint
				checkpointMessageID, err := coo.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, tips)
				if err != nil {
					// issuing checkpoint failed => not critical
					log.Warn(err)
					continue
				}
				lastCheckpointIndex++
				lastCheckpointMessageID = checkpointMessageID

			case <-nextMilestoneSignal:

				// issue a new checkpoint right in front of the milestone
				tips, err := selector.SelectTips(1)
				if err != nil {
					// issuing checkpoint failed => not critical
					if err != mselection.ErrNoTipsAvailable {
						log.Warn(err)
					}
				} else {
					checkpointMessageID, err := coo.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, tips)
					if err != nil {
						// issuing checkpoint failed => not critical
						log.Warn(err)
					} else {
						// use the new checkpoint message ID
						lastCheckpointMessageID = checkpointMessageID
					}
				}

				milestoneMessageID, err, criticalErr := coo.IssueMilestone(lastMilestoneMessageID, lastCheckpointMessageID)
				if criticalErr != nil {
					log.Panic(criticalErr)
				}
				if err != nil {
					if err == tangle.ErrNodeNotSynced {
						// Coordinator is not synchronized, trigger the solidifier manually
						tanglecore.TriggerSolidifier()
					}
					log.Warn(err)
					continue
				}

				// remember the last milestone message ID
				lastMilestoneMessageID = milestoneMessageID

				// reset the checkpoints
				lastCheckpointMessageID = milestoneMessageID
				lastCheckpointIndex = 0

			case <-shutdownSignal:
				break coordinatorLoop
			}
		}

		detachEvents()
	}, shutdown.PriorityCoordinator)

}

func sendMessage(msg *tangle.Message, msIndex ...milestone.Index) error {

	msgSolidEventChan := tanglecore.RegisterMessageSolidEvent(msg.GetMessageID())

	var milestoneConfirmedEventChan chan struct{}

	if len(msIndex) > 0 {
		milestoneConfirmedEventChan = tanglecore.RegisterMilestoneConfirmedEvent(msIndex[0])
	}

	if err := deps.MessageProcessor.Emit(msg); err != nil {
		return err
	}

	// wait until the message is solid
	utils.WaitForChannelClosed(msgSolidEventChan)

	if len(msIndex) > 0 {
		// if it was a milestone, also wait until the milestone was confirmed
		utils.WaitForChannelClosed(milestoneConfirmedEventChan)
	}

	return nil
}

// isBelowMaxDepth checks the below max depth criteria for the given message.
func isBelowMaxDepth(cachedMsgMeta *tangle.CachedMetadata) bool {
	defer cachedMsgMeta.Release(true)

	lsmi := deps.Tangle.GetSolidMilestoneIndex()

	_, ocri := dag.GetConeRootIndexes(deps.Tangle, cachedMsgMeta.Retain(), lsmi) // meta +1

	// if the OCRI to LSMI delta is over belowMaxDepth, then the tip is invalid.
	return (lsmi - ocri) > belowMaxDepth
}

// GetEvents returns the events of the coordinator
func GetEvents() *coordinator.Events {
	if coo == nil {
		return nil
	}
	return coo.Events
}

func configureEvents() {
	// pass all new solid messages to the selector
	onMessageSolid = events.NewClosure(func(cachedMsgMeta *tangle.CachedMetadata) {
		defer cachedMsgMeta.Release(true)

		if isBelowMaxDepth(cachedMsgMeta.Retain()) {
			// ignore tips that are below max depth
			return
		}

		// add tips to the heaviest branch selector
		if trackedMessagesCount := selector.OnNewSolidMessage(cachedMsgMeta.GetMetadata()); trackedMessagesCount >= maxTrackedMessages {
			log.Debugf("Coordinator Tipselector: trackedMessagesCount: %d", trackedMessagesCount)

			// issue next checkpoint
			select {
			case nextCheckpointSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}
	})

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		ts := time.Now()

		// do not propagate during syncing, because it is not needed at all
		if !deps.Tangle.IsNodeSyncedWithThreshold() {
			return
		}

		// propagate new cone root indexes to the future cone for heaviest branch tipselection
		dag.UpdateConeRootIndexes(deps.Tangle, confirmation.Mutations.MessagesReferenced, confirmation.MilestoneIndex)

		log.Debugf("UpdateConeRootIndexes finished, took: %v", time.Since(ts).Truncate(time.Millisecond))
	})

	onIssuedCheckpoint = events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, messageID *hornet.MessageID) {
		log.Infof("checkpoint (%d) message issued (%d/%d): %v", checkpointIndex+1, tipIndex+1, tipsTotal, messageID.Hex())
	})

	onIssuedMilestone = events.NewClosure(func(index milestone.Index, messageID *hornet.MessageID) {
		log.Infof("milestone issued (%d): %v", index, messageID.Hex())
	})
}

func attachEvents() {
	tanglecore.Events.MessageSolid.Attach(onMessageSolid)
	tanglecore.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
	coo.Events.IssuedCheckpointMessage.Attach(onIssuedCheckpoint)
	coo.Events.IssuedMilestone.Attach(onIssuedMilestone)
}

func detachEvents() {
	tanglecore.Events.MessageSolid.Detach(onMessageSolid)
	tanglecore.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
	coo.Events.IssuedMilestone.Detach(onIssuedMilestone)
}
