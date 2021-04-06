package coordinator

import (
	"fmt"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"
	"golang.org/x/net/context"

	"github.com/gohornet/hornet/core/gracefulshutdown"
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/mselection"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	powpackage "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

const (
	// whether to bootstrap the network
	CfgCoordinatorBootstrap = "cooBootstrap"
	// the index of the first milestone at bootstrap
	CfgCoordinatorStartIndex = "cooStartIndex"
	// the maximum limit of additional tips that fit into a milestone (besides the last milestone and checkpoint hash)
	MilestoneMaxAdditionalTipsLimit = 6
)

var (
	ErrDatabaseTainted = errors.New("database is tainted. delete the coordinator database and start again with a snapshot")
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
			Provide:   provide,
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

	nextCheckpointSignal chan struct{}
	nextMilestoneSignal  chan struct{}

	lastCheckpointIndex     int
	lastCheckpointMessageID hornet.MessageID
	lastMilestoneMessageID  hornet.MessageID

	// Closures
	onMessageSolid     *events.Closure
	onIssuedCheckpoint *events.Closure
	onIssuedMilestone  *events.Closure

	deps dependencies
)

type dependencies struct {
	dig.In
	Storage          *storage.Storage
	Tangle           *tangle.Tangle
	MessageProcessor *gossip.MessageProcessor
	NodeConfig       *configuration.Configuration `name:"nodeConfig"`
	BelowMaxDepth    int                          `name:"belowMaxDepth"`
	Coordinator      *coordinator.Coordinator
	Selector         *mselection.HeaviestSelector
}

func provide(c *dig.Container) {
	log = logger.NewLogger(Plugin.Name)

	type selectordeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps selectordeps) *mselection.HeaviestSelector {
		// use the heaviest branch tip selection for the milestones
		return mselection.New(
			deps.NodeConfig.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold),
			deps.NodeConfig.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint),
			deps.NodeConfig.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint),
			deps.NodeConfig.Duration(CfgCoordinatorTipselectHeaviestBranchSelectionTimeout),
		)
	}); err != nil {
		panic(err)
	}

	type coordinatordeps struct {
		dig.In
		Storage         *storage.Storage
		Tangle          *tangle.Tangle
		PoWHandler      *powpackage.Handler
		MigratorService *migrator.MigratorService `optional:"true"`
		UTXOManager     *utxo.Manager
		NodeConfig      *configuration.Configuration `name:"nodeConfig"`
		NetworkID       uint64                       `name:"networkId"`
	}

	if err := c.Provide(func(deps coordinatordeps) *coordinator.Coordinator {

		initCoordinator := func() (*coordinator.Coordinator, error) {

			signingProvider, err := initSigningProvider(
				deps.NodeConfig.String(CfgCoordinatorSigningProvider),
				deps.NodeConfig.String(CfgCoordinatorSigningRemoteAddress),
				deps.Storage.KeyManager(),
				deps.NodeConfig.Int(protocfg.CfgProtocolMilestonePublicKeyCount),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize signing provider: %s", err)
			}

			quorumGroups, err := initQuorumGroups(deps.NodeConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize coordinator quorum: %s", err)
			}

			if deps.NodeConfig.Bool(CfgCoordinatorQuorumEnabled) {
				log.Info("running Coordinator with quorum enabled")
			}

			if deps.MigratorService == nil {
				log.Info("running Coordinator without migration enabled")
			}

			coo, err := coordinator.New(
				deps.Storage,
				deps.NetworkID,
				signingProvider,
				deps.MigratorService,
				deps.UTXOManager,
				deps.PoWHandler,
				sendMessage,
				coordinator.WithLogger(log),
				coordinator.WithStateFilePath(deps.NodeConfig.String(CfgCoordinatorStateFilePath)),
				coordinator.WithMilestoneInterval(deps.NodeConfig.Duration(CfgCoordinatorInterval)),
				coordinator.WithPowWorkerCount(deps.NodeConfig.Int(CfgCoordinatorPoWWorkerCount)),
				coordinator.WithQuorum(deps.NodeConfig.Bool(CfgCoordinatorQuorumEnabled), quorumGroups, deps.NodeConfig.Duration(CfgCoordinatorQuorumTimeout)),
			)
			if err != nil {
				return nil, err
			}

			if err := coo.InitState(*bootstrap, milestone.Index(*startIndex)); err != nil {
				return nil, err
			}

			// don't issue milestones or checkpoints in case the node is running hot
			coo.AddBackPressureFunc(deps.Tangle.IsReceiveTxWorkerPoolBusy)

			return coo, nil
		}

		coo, err := initCoordinator()
		if err != nil {
			log.Panic(err)
		}
		return coo
	}); err != nil {
		panic(err)
	}
}

func configure() {

	if deps.Storage.IsDatabaseTainted() {
		panic(ErrDatabaseTainted)
	}

	nextCheckpointSignal = make(chan struct{})

	// must be a buffered channel, otherwise signal gets
	// lost if checkpoint is generated at the same time
	nextMilestoneSignal = make(chan struct{}, 1)

	maxTrackedMessages = deps.NodeConfig.Int(CfgCoordinatorCheckpointsMaxTrackedMessages)

	// set the node as synced at startup, so the coo plugin can select tips
	deps.Tangle.SetUpdateSyncedAtStartup(true)

	configureEvents()
}

// handleError checks for critical errors and returns true if the node should shutdown.
func handleError(err error) bool {
	if err == nil {
		return false
	}

	if err := common.IsCriticalError(err); err != nil {
		gracefulshutdown.SelfShutdown(fmt.Sprintf("coordinator plugin hit a critical error: %s", err.Error()))
		return true
	}

	if err := common.IsSoftError(err); err != nil {
		log.Warn(err)
		deps.Coordinator.Events.SoftError.Trigger(err)
		return false
	}

	// this should not happen! errors should be defined as a soft or critical error explicitly
	log.Panicf("coordinator plugin hit an unknown error type: %s", err)
	return true
}

func run() {

	// create a background worker that signals to issue new milestones
	Plugin.Daemon().BackgroundWorker("Coordinator[MilestoneTicker]", func(shutdownSignal <-chan struct{}) {

		ticker := timeutil.NewTicker(func() {
			// issue next milestone
			select {
			case nextMilestoneSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}, deps.Coordinator.GetInterval(), shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityCoordinator)

	// create a background worker that issues milestones
	Plugin.Daemon().BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
		// wait until all background workers of the tangle plugin are started
		deps.Tangle.WaitForTangleProcessorStartup()

		attachEvents()

		// bootstrap the network if not done yet
		milestoneMessageID, err := deps.Coordinator.Bootstrap()
		if handleError(err) {
			// critical error => stop worker
			detachEvents()
			return
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
				if trackedMessagesCount := deps.Selector.GetTrackedMessagesCount(); trackedMessagesCount < maxTrackedMessages {
					continue
				}

				tips, err := deps.Selector.SelectTips(0)
				if err != nil {
					// issuing checkpoint failed => not critical
					if err != mselection.ErrNoTipsAvailable {
						log.Warn(err)
					}
					continue
				}

				// issue a checkpoint
				checkpointMessageID, err := deps.Coordinator.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, tips)
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
				checkpointTips, err := deps.Selector.SelectTips(1)
				if err != nil {
					// issuing checkpoint failed => not critical
					if err != mselection.ErrNoTipsAvailable {
						log.Warn(err)
					}
				} else {
					if len(checkpointTips) > MilestoneMaxAdditionalTipsLimit {
						// issue a checkpoint with all the tips that wouldn't fit into the milestone (more than MilestoneMaxAdditionalTipsLimit)
						checkpointMessageID, err := deps.Coordinator.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, checkpointTips[MilestoneMaxAdditionalTipsLimit:])
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

				milestoneMessageID, err := deps.Coordinator.IssueMilestone(milestoneTips)
				if handleError(err) {
					// critical error => quit loop
					break coordinatorLoop
				}
				if err != nil {
					// non-critical errors
					if err == common.ErrNodeNotSynced {
						// Coordinator is not synchronized, trigger the solidifier manually
						deps.Tangle.TriggerSolidifier()
					}
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

func initSigningProvider(signingProviderType string, remoteEndpoint string, keyManager *keymanager.KeyManager, milestonePublicKeyCount int) (coordinator.MilestoneSignerProvider, error) {

	switch signingProviderType {
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

		return coordinator.NewInMemoryEd25519MilestoneSignerProvider(privateKeys, keyManager, milestonePublicKeyCount), nil

	case "remote":
		if remoteEndpoint == "" {
			return nil, errors.New("no address given for remote signing provider")
		}

		return coordinator.NewInsecureRemoteEd25519MilestoneSignerProvider(remoteEndpoint, keyManager, milestonePublicKeyCount), nil

	default:
		return nil, fmt.Errorf("unknown milestone signing provider: %s", signingProviderType)
	}
}

func initQuorumGroups(nodeConfig *configuration.Configuration) (map[string][]*coordinator.QuorumClientConfig, error) {
	// parse quorum groups config
	quorumGroups := make(map[string][]*coordinator.QuorumClientConfig)
	for _, groupName := range nodeConfig.MapKeys(CfgCoordinatorQuorumGroups) {
		configKey := CfgCoordinatorQuorumGroups + "." + groupName

		groupConfig := []*coordinator.QuorumClientConfig{}
		if err := nodeConfig.Unmarshal(configKey, &groupConfig); err != nil {
			return nil, fmt.Errorf("failed to parse group: %s, %s", configKey, err)
		}

		if len(groupConfig) == 0 {
			return nil, fmt.Errorf("invalid group: %s, no entries", configKey)
		}

		for _, entry := range groupConfig {
			if entry.BaseURL == "" {
				return nil, fmt.Errorf("invalid group: %s, missing baseURL in entry", configKey)
			}
		}

		quorumGroups[groupName] = groupConfig
	}

	return quorumGroups, nil
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

	cmi := deps.Storage.GetConfirmedMilestoneIndex()

	_, ocri := dag.GetConeRootIndexes(deps.Storage, cachedMsgMeta.Retain(), cmi) // meta +1

	// if the OCRI to CMI delta is over belowMaxDepth, then the tip is invalid.
	return (cmi - ocri) > milestone.Index(deps.BelowMaxDepth)
}

// GetEvents returns the events of the coordinator
func GetEvents() *coordinator.Events {
	if deps.Coordinator == nil {
		return nil
	}
	return deps.Coordinator.Events
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
		if trackedMessagesCount := deps.Selector.OnNewSolidMessage(cachedMsgMeta.GetMetadata()); trackedMessagesCount >= maxTrackedMessages {
			log.Debugf("Coordinator Tipselector: trackedMessagesCount: %d", trackedMessagesCount)

			// issue next checkpoint
			select {
			case nextCheckpointSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}
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
	deps.Coordinator.Events.IssuedCheckpointMessage.Attach(onIssuedCheckpoint)
	deps.Coordinator.Events.IssuedMilestone.Attach(onIssuedMilestone)
}

func detachEvents() {
	deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
	deps.Coordinator.Events.IssuedMilestone.Detach(onIssuedMilestone)
}
