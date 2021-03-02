package receipt

import (
	"fmt"
	"net/http"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/iota.go/api"
	iotago "github.com/iotaledger/iota.go/v2"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
		Pluggable: node.Pluggable{
			Name:      "Receipts",
			DepsFunc:  func(cDeps pluginDependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
	}
}

var (
	Plugin *node.Plugin

	log  *logger.Logger
	deps pluginDependencies
)

type pluginDependencies struct {
	dig.In
	ReceiptService *migrator.ReceiptService
	Tangle         *tangle.Tangle
	NodeConfig     *configuration.Configuration `name:"nodeConfig"`
}

// provide provides the ReceiptService as a singleton.
func provide(c *dig.Container) {
	type validatorDependencies struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}
	if err := c.Provide(func(deps validatorDependencies) (*migrator.Validator, error) {
		iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
			URI:    deps.NodeConfig.String(CfgReceiptsValidatorAPIAddress),
			Client: &http.Client{Timeout: deps.NodeConfig.Duration(CfgReceiptsValidatorAPITimeout)},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize API: %w", err)
		}
		return migrator.NewValidator(
			iotaAPI,
			deps.NodeConfig.String(CfgReceiptsValidatorCoordinatorAddress),
			deps.NodeConfig.Int(CfgReceiptsValidatorCoordinatorMerkleTreeDepth),
		), nil
	}); err != nil {
		panic(err)
	}
	type serviceDependencies struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
		Validator  *migrator.Validator
		UTXO       *utxo.Manager
		Storage    *storage.Storage
	}
	if err := c.Provide(func(deps serviceDependencies) (*migrator.ReceiptService, error) {
		return migrator.NewReceiptService(
			deps.Validator, deps.UTXO,
			deps.NodeConfig.Bool(CfgReceiptsValidatorIgnoreSoftErrors),
			deps.NodeConfig.Bool(CfgReceiptsValidatorValidate),
			deps.NodeConfig.Bool(CfgReceiptsBackupEnabled),
			deps.NodeConfig.String(CfgReceiptsBackupFolder),
		), nil
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)
	deps.Tangle.Events.NewReceipt.Attach(events.NewClosure(func(r *iotago.Receipt) {
		if deps.ReceiptService.ValidationEnabled {
			log.Info("receipt passed validation against %s", deps.NodeConfig.String(CfgReceiptsValidatorAPIAddress))
		}
		log.Infof("new receipt processed (migrated_at %d, final %v, entries %d),", r.MigratedAt, r.Final, len(r.Funds))
	}))
	log.Infof("storing receipt backups in %s", deps.NodeConfig.String(CfgReceiptsBackupFolder))
	if err := deps.ReceiptService.Init(); err != nil {
		panic(err)
	}
}
