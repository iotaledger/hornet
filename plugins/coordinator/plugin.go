package coordinator

import (
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/iota.go/v2/ed25519"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"
	"golang.org/x/net/context"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/mselection"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/gohornet/hornet/plugins/urts"
)

const (
	// whether to bootstrap the network
	CfgCoordinatorBootstrap = "cooBootstrap"
	// the index of the first milestone at bootstrap
	CfgCoordinatorStartIndex = "cooStartIndex"
	// the maximum limit of additional tips that fit into a milestone (besides the last milestone and checkpoint hash)
	MilestoneMaxAdditionalTipsLimit = 6
)

func init() {
	flag.CommandLine.MarkHidden(CfgCoordinatorBootstrap)
	flag.CommandLine.MarkHidden(CfgCoordinatorStartIndex)

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

	bootstrap  = flag.Bool(CfgCoordinatorBootstrap, false, "bootstrap the network")
	startIndex = flag.Uint32(CfgCoordinatorStartIndex, 0, "index of the first milestone at bootstrap")

	maxTrackedMessages int
	belowMaxDepth      milestone.Index

	nextCheckpointSignal chan struct{}
	nextMilestoneSignal  chan struct{}

	coo      *coordinator.Coordinator
	selector *mselection.HeaviestSelector

	lastCheckpointIndex     int
	lastCheckpointMessageID hornet.MessageID
	lastMilestoneMessageID  hornet.MessageID

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure
	onIssuedCheckpoint   *events.Closure
	onIssuedMilestone    *events.Closure

	ErrDatabaseTainted = errors.New("database is tainted. delete the coordinator database and start again with a snapshot")

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage          *storage.Storage
	Tangle           *tangle.Tangle
	PoWHandler       *powpackage.Handler
	MigratorService  *migrator.MigratorService `optional:"true"`
	UTXOManager      *utxo.Manager
	MessageProcessor *gossip.MessageProcessor
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
	NetworkID        uint64                       `name:"networkId"`
}

func configure() {
	log = logger.NewLogger(Plugin.Name)

	// set the node as synced at startup, so the coo plugin can select tips
	deps.Tangle.SetUpdateSyncedAtStartup(true)

	var err error
	coo, err = initCoordinator(*bootstrap, *startIndex, deps.PoWHandler)
	if err != nil {
		log.Panic(err)
	}

	configureEvents()
}

func initCoordinator(bootstrap bool, startIndex uint32, powHandler *powpackage.Handler) (*coordinator.Coordinator, error) {

	if deps.Storage.IsDatabaseTainted() {
		return nil, ErrDatabaseTainted
	}

	// use the heaviest branch tip selection for the milestones
	selector = mselection.New(
		deps.NodeConfig.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold),
		deps.NodeConfig.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint),
		deps.NodeConfig.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint),
		time.Duration(deps.NodeConfig.Int(CfgCoordinatorTipselectHeaviestBranchSelectionTimeoutMilliseconds))*time.Millisecond,
	)

	nextCheckpointSignal = make(chan struct{})

	// must be a buffered channel, otherwise signal gets
	// lost if checkpoint is generated at the same time
	nextMilestoneSignal = make(chan struct{}, 1)

	maxTrackedMessages = deps.NodeConfig.Int(CfgCoordinatorCheckpointsMaxTrackedMessages)

	belowMaxDepth = milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelBelowMaxDepth))

	var signingProvider coordinator.MilestoneSignerProvider
	switch deps.NodeConfig.String(CfgCoordinatorSigningProvider) {
	case "local":
		privateKeys, err := utils.LoadEd25519PrivateKeysFromEnvironment("COO_PRV_KEYS")
		if err != nil {
			return nil, err
		}

		if len(privateKeys) == 0 {
			return nil, errors.New("no private keys given")
		}

		for _, privateKey := range privateKeys {
			if len(privateKey) != ed25519.PrivateKeySize {
				return nil, errors.New("wrong private key length")
			}
		}

		signingProvider = coordinator.NewInMemoryEd25519MilestoneSignerProvider(privateKeys, deps.Storage.KeyManager(), deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount))

	case "remote":
		remoteEndpoint := deps.NodeConfig.String(CfgCoordinatorSigningRemoteAddress)
		if remoteEndpoint == "" {
			return nil, errors.New("no address given for remote signing provider")
		}
		signingProvider = coordinator.NewInsecureRemoteEd25519MilestoneSignerProvider(remoteEndpoint, deps.Storage.KeyManager(), deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount))

	default:
		return nil, fmt.Errorf("unknown milestone signing provider: %s", deps.NodeConfig.String(CfgCoordinatorSigningProvider))
	}

	powParallelism := deps.NodeConfig.Int(CfgCoordinatorPoWParallelism)
	if powParallelism < 1 {
		powParallelism = 1
	}

	if deps.MigratorService == nil {
		log.Info("running Coordinator without migration enabled")
	}

	coo, err := coordinator.New(
		deps.Storage,
		deps.NetworkID,
		signingProvider,
		deps.NodeConfig.String(CfgCoordinatorStateFilePath),
		deps.NodeConfig.Int(CfgCoordinatorIntervalSeconds),
		powParallelism,
		powHandler,
		deps.MigratorService,
		deps.UTXOManager,
		sendMessage,
	)
	if err != nil {
		return nil, err
	}

	if err := coo.InitState(bootstrap, milestone.Index(startIndex)); err != nil {
		return nil, err
	}

	// don't issue milestones or checkpoints in case the node is running hot
	coo.AddBackPressureFunc(deps.Tangle.IsReceiveTxWorkerPoolBusy)

	return coo, nil
}

func run() {

	// create a background worker that signals to issue new milestones
	Plugin.Daemon().BackgroundWorker("Coordinator[MilestoneTicker]", func(shutdownSignal <-chan struct{}) {

		timeutil.NewTicker(func() {
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
		deps.Tangle.WaitForTangleProcessorStartup()

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
				var milestoneTips hornet.MessageIDs

				// issue a new checkpoint right in front of the milestone
				checkpointTips, err := selector.SelectTips(1)
				if err != nil {
					// issuing checkpoint failed => not critical
					if err != mselection.ErrNoTipsAvailable {
						log.Warn(err)
					}
				} else {
					if len(checkpointTips) > MilestoneMaxAdditionalTipsLimit {
						// issue a checkpoint with all the tips that wouldn't fit into the milestone (more than MilestoneMaxAdditionalTipsLimit)
						checkpointMessageID, err := coo.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, checkpointTips[MilestoneMaxAdditionalTipsLimit:])
						if err != nil {
							// issuing checkpoint failed => not critical
							log.Warn(err)
						} else {
							// use the new checkpoint message ID
							lastCheckpointMessageID = checkpointMessageID
						}

						// use the other tips for the milestone
						milestoneTips = checkpointTips[:MilestoneMaxAdditionalTipsLimit]
					} else {
						// do not issue a checkpoint and use the tips for the milestone instead since they fit into the milestone directly
						milestoneTips = checkpointTips
					}
				}

				milestoneTips = append(milestoneTips, hornet.MessageIDs{lastMilestoneMessageID, lastCheckpointMessageID}...)

				milestoneMessageID, err, criticalErr := coo.IssueMilestone(milestoneTips)
				if criticalErr != nil {
					log.Panic(criticalErr)
				}
				if err != nil {
					if err == common.ErrNodeNotSynced {
						// Coordinator is not synchronized, trigger the solidifier manually
						deps.Tangle.TriggerSolidifier()
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

func sendMessage(msg *storage.Message, msIndex ...milestone.Index) error {

	msgSolidEventChan := deps.Tangle.RegisterMessageSolidEvent(msg.GetMessageID())

	var milestoneConfirmedEventChan chan struct{}

	if len(msIndex) > 0 {
		milestoneConfirmedEventChan = deps.Tangle.RegisterMilestoneConfirmedEvent(msIndex[0])
	}

	if err := deps.MessageProcessor.Emit(msg); err != nil {
		deps.Tangle.DeregisterMessageSolidEvent(msg.GetMessageID())
		if len(msIndex) > 0 {
			deps.Tangle.DeregisterMilestoneConfirmedEvent(msIndex[0])
		}

		return err
	}

	// wait until the message is solid
	utils.WaitForChannelClosed(context.Background(), msgSolidEventChan)

	if len(msIndex) > 0 {
		// if it was a milestone, also wait until the milestone was confirmed
		utils.WaitForChannelClosed(context.Background(), milestoneConfirmedEventChan)
	}

	return nil
}

// isBelowMaxDepth checks the below max depth criteria for the given message.
func isBelowMaxDepth(cachedMsgMeta *storage.CachedMetadata) bool {
	defer cachedMsgMeta.Release(true)

	lsmi := deps.Storage.GetSolidMilestoneIndex()

	_, ocri := dag.GetConeRootIndexes(deps.Storage, cachedMsgMeta.Retain(), lsmi) // meta +1

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
	onMessageSolid = events.NewClosure(func(cachedMsgMeta *storage.CachedMetadata) {
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
		if !deps.Storage.IsNodeSyncedWithThreshold() {
			return
		}

		// propagate new cone root indexes to the future cone for heaviest branch tipselection
		dag.UpdateConeRootIndexes(deps.Storage, confirmation.Mutations.MessagesReferenced, confirmation.MilestoneIndex)

		log.Debugf("UpdateConeRootIndexes finished, took: %v", time.Since(ts).Truncate(time.Millisecond))
	})

	onIssuedCheckpoint = events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, messageID hornet.MessageID) {
		log.Infof("checkpoint (%d) message issued (%d/%d): %v", checkpointIndex+1, tipIndex+1, tipsTotal, messageID.ToHex())
	})

	onIssuedMilestone = events.NewClosure(func(index milestone.Index, messageID hornet.MessageID) {
		log.Infof("milestone issued (%d): %v", index, messageID.ToHex())
	})
}

func attachEvents() {
	deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
	deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
	coo.Events.IssuedCheckpointMessage.Attach(onIssuedCheckpoint)
	coo.Events.IssuedMilestone.Attach(onIssuedMilestone)
}

func detachEvents() {
	deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
	deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
	coo.Events.IssuedMilestone.Detach(onIssuedMilestone)
}
