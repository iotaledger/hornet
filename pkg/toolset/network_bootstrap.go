package toolset

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/ioutils"
	"github.com/iotaledger/hive.go/serializer/v2"
	databasecore "github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/core/protocfg"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
	"github.com/iotaledger/iota.go/v3/keymanager"
	"github.com/iotaledger/iota.go/v3/signingprovider"
)

// CoordinatorState is the JSON representation of a coordinator state.
type CoordinatorState struct {
	LatestMilestoneIndex   iotago.MilestoneIndex `json:"latestMilestoneIndex"`
	LatestMilestoneBlockID string                `json:"latestMilestoneBlockId"`
	LatestMilestoneID      string                `json:"latestMilestoneId"`
	LatestMilestoneTime    int64                 `json:"latestMilestoneTime"`
}

func networkBootstrap(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	configFilePathFlag := fs.String(FlagToolConfigFilePath, "", "the path to the config file")
	genesisSnapshotPathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the genesis snapshot file")
	databasePathFlag := fs.String(FlagToolDatabasePath, "", "the path to the coordinator database")
	cooStatePathFlag := fs.String(FlagToolCoordinatorStatePath, "", "the path to the coordinator state file")
	databaseEngineFlag := fs.String(FlagToolDatabaseEngine, string(DefaultValueDatabaseEngine), "database engine (optional, values: pebble, rocksdb)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolBootstrapPrivateTangle)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s --%s %s --%s %s",
			ToolBootstrapPrivateTangle,
			FlagToolConfigFilePath,
			"config.json",
			FlagToolSnapshotPath,
			"genesis_snapshot.bin",
			FlagToolDatabasePath,
			"privatedb",
			FlagToolCoordinatorStatePath,
			"coordinator.state",
			FlagToolDatabaseEngine,
			DefaultValueDatabaseEngine))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*configFilePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolConfigFilePath)
	}
	if len(*genesisSnapshotPathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPath)
	}
	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}
	if len(*cooStatePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolCoordinatorStatePath)
	}

	configFilePath := *configFilePathFlag
	if _, err := os.Stat(configFilePath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) does not exist", FlagToolConfigFilePath, configFilePath)
	}

	genesisSnapshotPath := *genesisSnapshotPathFlag
	if _, err := os.Stat(genesisSnapshotPath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) does not exist", FlagToolSnapshotPath, genesisSnapshotPath)
	}

	databasePath := *databasePathFlag
	tangleDatabasePath := filepath.Join(databasePath, databasecore.TangleDatabaseDirectoryName)
	if _, err := os.Stat(tangleDatabasePath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("tangle database path (%s) already exists", tangleDatabasePath)
	}

	utxoDatabasePath := filepath.Join(databasePath, databasecore.UTXODatabaseDirectoryName)
	if _, err := os.Stat(utxoDatabasePath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("UTXO database path (%s) already exists", utxoDatabasePath)
	}

	cooStatePath := *cooStatePathFlag
	if _, err := os.Stat(cooStatePath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) already exists", FlagToolCoordinatorStatePath, cooStatePath)
	}

	dbEngine, err := hivedb.EngineFromStringAllowed(*databaseEngineFlag, database.AllowedEnginesStorage...)
	if err != nil {
		return err
	}

	keyManager, milestonePublicKeyCount, err := getKeyManagerAndMilestonePublicKeyCountFromConfigFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to load milestone public key manager from config file: %w", err)
	}

	signer, err := initSigningProvider(keyManager, milestonePublicKeyCount)
	if err != nil {
		return fmt.Errorf("failed to load milestone signing provider: %w", err)
	}

	println("creating databases ...")
	tangleStore, err := createTangleStorage(
		"",
		tangleDatabasePath,
		utxoDatabasePath,
		dbEngine,
	)
	if err != nil {
		return fmt.Errorf("failed to create databases: %w", err)
	}

	defer func() {
		println("\nshutdown storage ...")
		if err := tangleStore.Shutdown(); err != nil {
			panic(err)
		}
	}()

	// load the genesis ledger state into the storage (SEP and ledger state only)
	println("loading genesis snapshot ...")
	if err := loadGenesisSnapshot(context.Background(), tangleStore, genesisSnapshotPath, false, 0); err != nil {
		return fmt.Errorf("failed to load genesis snapshot: %w", err)
	}

	// create first milestone to bootstrap the network
	println("create first milestone ...")
	cooState, err := createInitialMilestone(tangleStore, signer)
	if err != nil {
		return fmt.Errorf("failed to create initial milestone: %w", err)
	}

	println("store coordinator state ...")
	if err := ioutils.WriteJSONToFile(cooStatePath, cooState, 0660); err != nil {
		return fmt.Errorf("failed to store coordinator state: %w", err)
	}

	fmt.Println("network bootstrap successful!")

	return nil
}

func getKeyManagerAndMilestonePublicKeyCountFromConfigFile(filePath string) (*keymanager.KeyManager, int, error) {

	if err := loadConfigFile(filePath, map[string]any{
		"protocol": protocfg.ParamsProtocol,
	}); err != nil {
		return nil, 0, err
	}

	keyManager, err := protocfg.KeyManagerWithConfigPublicKeyRanges(protocfg.ParamsProtocol.PublicKeyRanges)
	if err != nil {
		return nil, 0, err
	}

	return keyManager, protocfg.ParamsProtocol.MilestonePublicKeyCount, nil
}

// loadEd25519PrivateKeysFromEnvironment loads ed25519 private keys from the given environment variable.
func loadEd25519PrivateKeysFromEnvironment(name string) ([]ed25519.PrivateKey, error) {

	keys, exists := os.LookupEnv(name)
	if !exists {
		return nil, fmt.Errorf("environment variable '%s' not set", name)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("environment variable '%s' not set", name)
	}

	privateKeysSplitted := strings.Split(keys, ",")
	privateKeys := make([]ed25519.PrivateKey, len(privateKeysSplitted))
	for i, key := range privateKeysSplitted {
		privateKey, err := crypto.ParseEd25519PrivateKeyFromString(key)
		if err != nil {
			return nil, fmt.Errorf("environment variable '%s' contains an invalid private key '%s'", name, key)

		}
		privateKeys[i] = privateKey
	}

	return privateKeys, nil
}

func initSigningProvider(keyManager *keymanager.KeyManager, milestonePublicKeyCount int) (signingprovider.MilestoneSignerProvider, error) {

	privateKeys, err := loadEd25519PrivateKeysFromEnvironment("COO_PRV_KEYS")
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

	return signingprovider.NewInMemoryEd25519MilestoneSignerProvider(privateKeys, keyManager, milestonePublicKeyCount), nil
}

// createMilestone creates a signed milestone block.
func createMilestone(
	signer signingprovider.MilestoneSignerProvider,
	protocolVersion byte,
	index iotago.MilestoneIndex,
	timestamp uint32,
	parents iotago.BlockIDs,
	previousMilestoneID iotago.MilestoneID,
	mutations *whiteflag.WhiteFlagMutations) (*iotago.Block, error) {

	msPayload := iotago.NewMilestone(index, timestamp, protocolVersion, previousMilestoneID, parents, mutations.InclusionMerkleRoot, mutations.AppliedMerkleRoot)

	iotaBlock, err := builder.
		NewBlockBuilder().
		ProtocolVersion(protocolVersion).
		Parents(parents).
		Payload(msPayload).
		Build()
	if err != nil {
		return nil, err
	}

	milestoneIndexSigner := signer.MilestoneIndexSigner(index)
	pubKeys := milestoneIndexSigner.PublicKeys()

	if err := msPayload.Sign(pubKeys, milestoneIndexSigner.SigningFunc()); err != nil {
		return nil, err
	}

	if err = msPayload.VerifySignatures(signer.PublicKeysCount(), milestoneIndexSigner.PublicKeysSet()); err != nil {
		return nil, err
	}

	if _, err := iotaBlock.Serialize(serializer.DeSeriModePerformValidation, nil); err != nil {
		return nil, err
	}

	return iotaBlock, nil
}

// createInitialMilestone creates a milestone block and stores it to the given storage.
func createInitialMilestone(dbStorage *storage.Storage, signer signingprovider.MilestoneSignerProvider) (*CoordinatorState, error) {

	if err := checkSnapshotInfo(dbStorage); err != nil {
		return nil, err
	}
	snapshotInfo := dbStorage.SnapshotInfo()

	var index iotago.MilestoneIndex = 1
	parents := iotago.BlockIDs{iotago.EmptyBlockID()}
	timestamp := time.Now()
	previousMilestoneID := iotago.MilestoneID{}

	mutations, err := whiteflag.ComputeWhiteFlagMutations(
		context.Background(),
		dbStorage.UTXOManager(),
		dag.NewParentsTraverser(dbStorage),
		dbStorage.CachedBlock,
		index,
		uint32(timestamp.Unix()),
		parents,
		previousMilestoneID,
		snapshotInfo.GenesisMilestoneIndex(),
		whiteflag.DefaultWhiteFlagTraversalCondition,
	)
	if err != nil {
		return nil, err
	}

	protoParams, err := dbStorage.CurrentProtocolParameters()
	if err != nil {
		return nil, err
	}

	milestoneBlock, err := createMilestone(signer, protoParams.Version, index, uint32(timestamp.Unix()), parents, previousMilestoneID, mutations)
	if err != nil {
		return nil, fmt.Errorf("failed to create milestone: %w", err)
	}

	milestonePayload, ok := milestoneBlock.Payload.(*iotago.Milestone)
	if !ok {
		return nil, fmt.Errorf("wrong milestone payload type: (expected *iotago.Milestone, got %T)", milestoneBlock.Payload)
	}

	milestoneID, err := milestonePayload.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to compute milestone ID: %w", err)
	}

	block, err := storage.NewBlock(milestoneBlock, serializer.DeSeriModePerformValidation, protoParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create milestone block: %w", err)
	}

	cachedBlock, _ := dbStorage.StoreBlockIfAbsent(block) // block +1
	defer cachedBlock.Release(true)                       // block -1

	for _, parent := range block.Parents() {
		dbStorage.StoreChild(parent, cachedBlock.Block().BlockID()).Release(true) // child +-0
	}

	// Mark milestone block as milestone in the database (needed for whiteflag to find last milestone)
	cachedBlock.Metadata().SetMilestone(true)

	cachedMilestone, _ := dbStorage.StoreMilestoneIfAbsent(milestonePayload) // milestone +1
	defer cachedMilestone.Release(true)                                      // milestone -1

	if err := dbStorage.UTXOManager().ApplyConfirmation(index, utxo.Outputs{}, utxo.Spents{}, nil, nil); err != nil {
		return nil, fmt.Errorf("applying confirmation failed: %w", err)
	}

	latestMilestoneBlockID, err := block.Block().ID()
	if err != nil {
		return nil, err
	}

	return &CoordinatorState{
		LatestMilestoneIndex:   index,
		LatestMilestoneBlockID: latestMilestoneBlockID.ToHex(),
		LatestMilestoneID:      milestoneID.ToHex(),
		LatestMilestoneTime:    timestamp.UnixNano(),
	}, nil
}
