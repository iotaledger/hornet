package coordinator

import (
	"fmt"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"
	"golang.org/x/net/context"

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
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
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
	_ = flag.CommandLine.MarkHidden(CfgCoordinatorBootstrap)
	_ = flag.CommandLine.MarkHidden(CfgCoordinatorStartIndex)

	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
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
	deps   dependencies

	bootstrap  = flag.Bool(CfgCoordinatorBootstrap, false, "bootstrap the network")
	startIndex = flag.Uint32(CfgCoordinatorStartIndex, 0, "index of the first milestone at bootstrap")

	maxTrackedMessages int

	nextCheckpointSignal chan struct{}
	nextMilestoneSignal  chan struct{}

	heaviestSelectorLock syncutils.RWMutex

	lastCheckpointIndex     int
	lastCheckpointMessageID hornet.MessageID
	lastMilestoneMessageID  hornet.MessageID

	// Closures
	onMessageSolid                   *events.Closure
	onConfirmedMilestoneIndexChanged *events.Closure
	onIssuedCheckpoint               *events.Closure
	onIssuedMilestone                *events.Closure
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
	ShutdownHandler  *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	type selectorDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps selectorDeps) *mselection.HeaviestSelector {
		// use the heaviest branch tip selection for the milestones
		return mselection.New(
			deps.NodeConfig.Int(CfgCoordinatorTipselectMinHeaviestBranchUnreferencedMessagesThreshold),
			deps.NodeConfig.Int(CfgCoordinatorTipselectMaxHeaviestBranchTipsPerCheckpoint),
			deps.NodeConfig.Int(CfgCoordinatorTipselectRandomTipsPerCheckpoint),
			deps.NodeConfig.Duration(CfgCoordinatorTipselectHeaviestBranchSelectionTimeout),
		)
	}); err != nil {
		Plugin.Panic(err)
	}

	type coordinatorDeps struct {
		dig.In
		Storage                 *storage.Storage
		Tangle                  *tangle.Tangle
		PoWHandler              *pow.Handler
		MigratorService         *migrator.MigratorService `optional:"true"`
		UTXOManager             *utxo.Manager
		NodeConfig              *configuration.Configuration `name:"nodeConfig"`
		NetworkID               uint64                       `name:"networkId"`
		MilestonePublicKeyCount int                          `name:"milestonePublicKeyCount"`
	}

	if err := c.Provide(func(deps coordinatorDeps) *coordinator.Coordinator {

		initCoordinator := func() (*coordinator.Coordinator, error) {

			signingProvider, err := initSigningProvider(
				deps.NodeConfig.String(CfgCoordinatorSigningProvider),
				deps.NodeConfig.String(CfgCoordinatorSigningRemoteAddress),
				deps.Storage.KeyManager(),
				deps.MilestonePublicKeyCount,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize signing provider: %s", err)
			}

			quorumGroups, err := initQuorumGroups(deps.NodeConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize coordinator quorum: %s", err)
			}

			if deps.NodeConfig.Bool(CfgCoordinatorQuorumEnabled) {
				Plugin.LogInfo("running Coordinator with quorum enabled")
			}

			if deps.MigratorService == nil {
				Plugin.LogInfo("running Coordinator without migration enabled")
			}

			coo, err := coordinator.New(
				deps.Storage,
				deps.NetworkID,
				signingProvider,
				deps.MigratorService,
				deps.UTXOManager,
				deps.PoWHandler,
				sendMessage,
				coordinator.WithLogger(Plugin.Logger()),
				coordinator.WithStateFilePath(deps.NodeConfig.String(CfgCoordinatorStateFilePath)),
				coordinator.WithMilestoneInterval(deps.NodeConfig.Duration(CfgCoordinatorInterval)),
				coordinator.WithPoWWorkerCount(deps.NodeConfig.Int(CfgCoordinatorPoWWorkerCount)),
				coordinator.WithQuorum(deps.NodeConfig.Bool(CfgCoordinatorQuorumEnabled), quorumGroups, deps.NodeConfig.Duration(CfgCoordinatorQuorumTimeout)),
				coordinator.WithSigningRetryAmount(deps.NodeConfig.Int(CfgCoordinatorSigningRetryAmount)),
				coordinator.WithSigningRetryTimeout(deps.NodeConfig.Duration(CfgCoordinatorSigningRetryTimeout)),
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
			Plugin.Panic(err)
		}
		return coo
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	databaseTainted, err := deps.Storage.IsDatabaseTainted()
	if err != nil {
		Plugin.Panic(err)
	}

	if databaseTainted {
		Plugin.Panic(ErrDatabaseTainted)
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
		deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("coordinator plugin hit a critical error: %s", err))
		return true
	}

	if err := common.IsSoftError(err); err != nil {
		Plugin.LogWarn(err)
		deps.Coordinator.Events.SoftError.Trigger(err)
		return false
	}

	// this should not happen! errors should be defined as a soft or critical error explicitly
	Plugin.Panicf("coordinator plugin hit an unknown error type: %s", err)
	return true
}

func run() {

	// create a background worker that signals to issue new milestones
	if err := Plugin.Daemon().BackgroundWorker("Coordinator[MilestoneTicker]", func(shutdownSignal <-chan struct{}) {

		ticker := timeutil.NewTicker(func() {
			// issue next milestone
			select {
			case nextMilestoneSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}, deps.Coordinator.Interval(), shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityCoordinator); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}

	// create a background worker that issues milestones
	if err := Plugin.Daemon().BackgroundWorker("Coordinator", func(shutdownSignal <-chan struct{}) {
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
				if trackedMessagesCount := deps.Selector.TrackedMessagesCount(); trackedMessagesCount < maxTrackedMessages {
					continue
				}

				func() {
					// this lock is necessary, otherwise a checkpoint could be issued
					// while a milestone gets confirmed. In that case the checkpoint could
					// contain messages that are already below max depth.
					heaviestSelectorLock.RLock()
					defer heaviestSelectorLock.RUnlock()

					tips, err := deps.Selector.SelectTips(0)
					if err != nil {
						// issuing checkpoint failed => not critical
						if !errors.Is(err, mselection.ErrNoTipsAvailable) {
							Plugin.LogWarn(err)
						}
						return
					}

					// issue a checkpoint
					checkpointMessageID, err := deps.Coordinator.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, tips)
					if err != nil {
						// issuing checkpoint failed => not critical
						Plugin.LogWarn(err)
						return
					}
					lastCheckpointIndex++
					lastCheckpointMessageID = checkpointMessageID
				}()

			case <-nextMilestoneSignal:
				var milestoneTips hornet.MessageIDs

				// issue a new checkpoint right in front of the milestone
				checkpointTips, err := deps.Selector.SelectTips(1)
				if err != nil {
					// issuing checkpoint failed => not critical
					if !errors.Is(err, mselection.ErrNoTipsAvailable) {
						Plugin.LogWarn(err)
					}
				} else {
					if len(checkpointTips) > MilestoneMaxAdditionalTipsLimit {
						// issue a checkpoint with all the tips that wouldn't fit into the milestone (more than MilestoneMaxAdditionalTipsLimit)
						checkpointMessageID, err := deps.Coordinator.IssueCheckpoint(lastCheckpointIndex, lastCheckpointMessageID, checkpointTips[MilestoneMaxAdditionalTipsLimit:])
						if err != nil {
							// issuing checkpoint failed => not critical
							Plugin.LogWarn(err)
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
					if errors.Is(err, common.ErrNodeNotSynced) {
						// Coordinator is not synchronized, trigger the solidifier manually
						deps.Tangle.TriggerSolidifier()
					}

					// reset the checkpoints
					lastCheckpointMessageID = lastMilestoneMessageID
					lastCheckpointIndex = 0

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
	}, shutdown.PriorityCoordinator); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}

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

	var err error

	msgSolidEventChan := deps.Tangle.RegisterMessageSolidEvent(msg.MessageID())

	var milestoneConfirmedEventChan chan struct{}

	if len(msIndex) > 0 {
		milestoneConfirmedEventChan = deps.Tangle.RegisterMilestoneConfirmedEvent(msIndex[0])
	}

	defer func() {
		if err != nil {
			deps.Tangle.DeregisterMessageSolidEvent(msg.MessageID())
			if len(msIndex) > 0 {
				deps.Tangle.DeregisterMilestoneConfirmedEvent(msIndex[0])
			}
		}
	}()

	if err = deps.MessageProcessor.Emit(msg); err != nil {
		return err
	}

	// wait until the message is solid
	if err = utils.WaitForChannelClosed(context.Background(), msgSolidEventChan); err != nil {
		return err
	}

	if len(msIndex) > 0 {
		// if it was a milestone, also wait until the milestone was confirmed
		if err = utils.WaitForChannelClosed(context.Background(), milestoneConfirmedEventChan); err != nil {
			return err
		}
	}

	return nil
}

// isBelowMaxDepth checks the below max depth criteria for the given message.
func isBelowMaxDepth(cachedMsgMeta *storage.CachedMetadata) bool {
	defer cachedMsgMeta.Release(true)

	cmi := deps.Storage.ConfirmedMilestoneIndex()

	_, ocri := dag.ConeRootIndexes(deps.Storage, cachedMsgMeta.Retain(), cmi) // meta +1

	// if the OCRI to CMI delta is over belowMaxDepth, then the tip is invalid.
	return (cmi - ocri) > milestone.Index(deps.BelowMaxDepth)
}

// Events returns the events of the coordinator
func Events() *coordinator.Events {
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
		if trackedMessagesCount := deps.Selector.OnNewSolidMessage(cachedMsgMeta.Metadata()); trackedMessagesCount >= maxTrackedMessages {
			Plugin.LogDebugf("Coordinator Tipselector: trackedMessagesCount: %d", trackedMessagesCount)

			// issue next checkpoint
			select {
			case nextCheckpointSignal <- struct{}{}:
			default:
				// do not block if already another signal is waiting
			}
		}
	})

	onConfirmedMilestoneIndexChanged = events.NewClosure(func(_ milestone.Index) {
		heaviestSelectorLock.Lock()
		defer heaviestSelectorLock.Unlock()

		// the selector needs to be reset after the milestone was confirmed, otherwise
		// it could contain tips that are already below max depth.
		deps.Selector.Reset()

		// the checkpoint also needs to be reset, otherwise
		// a checkpoint could have been issued in the meantime,
		// which could contain messages that are already below max depth.
		lastCheckpointMessageID = lastMilestoneMessageID
		lastCheckpointIndex = 0
	})

	onIssuedCheckpoint = events.NewClosure(func(checkpointIndex int, tipIndex int, tipsTotal int, messageID hornet.MessageID) {
		Plugin.LogInfof("checkpoint (%d) message issued (%d/%d): %v", checkpointIndex+1, tipIndex+1, tipsTotal, messageID.ToHex())
	})

	onIssuedMilestone = events.NewClosure(func(index milestone.Index, messageID hornet.MessageID) {
		Plugin.LogInfof("milestone issued (%d): %v", index, messageID.ToHex())
	})
}

func attachEvents() {
	deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
	deps.Coordinator.Events.IssuedCheckpointMessage.Attach(onIssuedCheckpoint)
	deps.Coordinator.Events.IssuedMilestone.Attach(onIssuedMilestone)
}

func detachEvents() {
	deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
	deps.Coordinator.Events.IssuedMilestone.Detach(onIssuedMilestone)
}
