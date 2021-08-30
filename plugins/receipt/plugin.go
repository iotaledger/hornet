package receipt

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/iota.go/api"
	iotago "github.com/iotaledger/iota.go/v2"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:      "Receipts",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
	}
}

var (
	Plugin *node.Plugin

	deps dependencies
)

type dependencies struct {
	dig.In
	ReceiptService *migrator.ReceiptService
	Tangle         *tangle.Tangle
	NodeConfig     *configuration.Configuration `name:"nodeConfig"`
}

// provide provides the ReceiptService as a singleton.
func provide(c *dig.Container) {

	type validatorDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps validatorDeps) *migrator.Validator {
		iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
			URI:    deps.NodeConfig.String(CfgReceiptsValidatorAPIAddress),
			Client: &http.Client{Timeout: deps.NodeConfig.Duration(CfgReceiptsValidatorAPITimeout)},
		})
		if err != nil {
			Plugin.Panicf("failed to initialize API: %s", err)
		}
		return migrator.NewValidator(
			iotaAPI,
			deps.NodeConfig.String(CfgReceiptsValidatorCoordinatorAddress),
			deps.NodeConfig.Int(CfgReceiptsValidatorCoordinatorMerkleTreeDepth),
		)
	}); err != nil {
		Plugin.Panic(err)
	}

	type serviceDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
		Validator  *migrator.Validator
		UTXO       *utxo.Manager
		Storage    *storage.Storage
	}

	if err := c.Provide(func(deps serviceDeps) *migrator.ReceiptService {
		return migrator.NewReceiptService(
			deps.Validator, deps.UTXO,
			deps.NodeConfig.Bool(CfgReceiptsValidatorValidate),
			deps.NodeConfig.Bool(CfgReceiptsBackupEnabled),
			deps.NodeConfig.Bool(CfgReceiptsValidatorIgnoreSoftErrors),
			deps.NodeConfig.String(CfgReceiptsBackupPath),
		)
	}); err != nil {
		Plugin.Panic(err)
	}
}

func configure() {

	deps.Tangle.Events.NewReceipt.Attach(events.NewClosure(func(r *iotago.Receipt) {
		if deps.ReceiptService.ValidationEnabled {
			Plugin.LogInfof("receipt passed validation against %s", deps.NodeConfig.String(CfgReceiptsValidatorAPIAddress))
		}
		Plugin.LogInfof("new receipt processed (migrated_at %d, final %v, entries %d),", r.MigratedAt, r.Final, len(r.Funds))
	}))
	Plugin.LogInfof("storing receipt backups in %s", deps.NodeConfig.String(CfgReceiptsBackupPath))
	if err := deps.ReceiptService.Init(); err != nil {
		Plugin.Panic(err)
	}
}
