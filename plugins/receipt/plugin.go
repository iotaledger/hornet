package receipt

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/iota.go/api"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusEnabled,
		Component: &app.Component{
			Name:      "Receipts",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
	}
}

var (
	Plugin *app.Plugin

	deps dependencies
)

type dependencies struct {
	dig.In
	ReceiptService *migrator.ReceiptService
	Tangle         *tangle.Tangle
	AppConfig      *configuration.Configuration `name:"appConfig"`
}

// provide provides the ReceiptService as a singleton.
func provide(c *dig.Container) error {

	type validatorDeps struct {
		dig.In
		AppConfig *configuration.Configuration `name:"appConfig"`
	}

	if err := c.Provide(func(deps validatorDeps) *migrator.Validator {
		iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
			URI:    deps.AppConfig.String(CfgReceiptsValidatorAPIAddress),
			Client: &http.Client{Timeout: deps.AppConfig.Duration(CfgReceiptsValidatorAPITimeout)},
		})
		if err != nil {
			Plugin.LogPanicf("failed to initialize API: %s", err)
		}
		return migrator.NewValidator(
			iotaAPI,
			deps.AppConfig.String(CfgReceiptsValidatorCoordinatorAddress),
			deps.AppConfig.Int(CfgReceiptsValidatorCoordinatorMerkleTreeDepth),
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}

	type serviceDeps struct {
		dig.In
		AppConfig   *configuration.Configuration `name:"appConfig"`
		Validator   *migrator.Validator
		UTXOManager *utxo.Manager
	}

	if err := c.Provide(func(deps serviceDeps) *migrator.ReceiptService {
		return migrator.NewReceiptService(
			deps.Validator,
			deps.UTXOManager,
			deps.AppConfig.Bool(CfgReceiptsValidatorValidate),
			deps.AppConfig.Bool(CfgReceiptsBackupEnabled),
			deps.AppConfig.Bool(CfgReceiptsValidatorIgnoreSoftErrors),
			deps.AppConfig.String(CfgReceiptsBackupPath),
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {

	deps.Tangle.Events.NewReceipt.Attach(events.NewClosure(func(r *iotago.ReceiptMilestoneOpt) {
		if deps.ReceiptService.ValidationEnabled {
			Plugin.LogInfof("receipt passed validation against %s", deps.AppConfig.String(CfgReceiptsValidatorAPIAddress))
		}
		Plugin.LogInfof("new receipt processed (migrated_at %d, final %v, entries %d),", r.MigratedAt, r.Final, len(r.Funds))
	}))
	Plugin.LogInfof("storing receipt backups in %s", deps.AppConfig.String(CfgReceiptsBackupPath))
	if err := deps.ReceiptService.Init(); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}
