package receipt

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/model/migrator"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/iota.go/api"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:      "Receipts",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
		},
		IsEnabled: func() bool {
			return ParamsReceipts.Enabled
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
}

// provide provides the ReceiptService as a singleton.
func provide(c *dig.Container) error {

	if err := c.Provide(func() *migrator.Validator {
		iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
			URI:    ParamsReceipts.Validator.API.Address,
			Client: &http.Client{Timeout: ParamsReceipts.Validator.API.Timeout},
		})
		if err != nil {
			Plugin.LogPanicf("failed to initialize API: %s", err)
		}

		return migrator.NewValidator(
			iotaAPI,
			ParamsReceipts.Validator.Coordinator.Address,
			ParamsReceipts.Validator.Coordinator.MerkleTreeDepth,
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}

	type serviceDeps struct {
		dig.In
		Validator   *migrator.Validator
		UTXOManager *utxo.Manager
	}

	if err := c.Provide(func(deps serviceDeps) *migrator.ReceiptService {
		return migrator.NewReceiptService(
			deps.Validator,
			deps.UTXOManager,
			ParamsReceipts.Validator.Validate,
			ParamsReceipts.Backup.Enabled,
			ParamsReceipts.Validator.IgnoreSoftErrors,
			ParamsReceipts.Backup.Path,
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}

func configure() error {

	deps.Tangle.Events.NewReceipt.Hook(events.NewClosure(func(r *iotago.ReceiptMilestoneOpt) {
		if deps.ReceiptService.ValidationEnabled {
			Plugin.LogInfof("receipt passed validation against %s", ParamsReceipts.Validator.API.Address)
		}
		Plugin.LogInfof("new receipt processed (migrated_at %d, final %v, entries %d),", r.MigratedAt, r.Final, len(r.Funds))
	}))
	Plugin.LogInfof("storing receipt backups in %s", ParamsReceipts.Backup.Path)
	if err := deps.ReceiptService.Init(); err != nil {
		Plugin.LogPanic(err)
	}

	return nil
}
