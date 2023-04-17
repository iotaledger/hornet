package receipt

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/model/migrator"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/iota.go/api"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Component = &app.Component{
		Name:     "Receipts",
		DepsFunc: func(cDeps dependencies) { deps = cDeps },
		Params:   params,
		IsEnabled: func(c *dig.Container) bool {
			// do not enable in "autopeering entry node" mode
			return components.IsAutopeeringEntryNodeDisabled(c) && ParamsReceipts.Enabled
		},
		Provide:   provide,
		Configure: configure,
	}
}

var (
	Component *app.Component

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
			Component.LogPanicf("failed to initialize API: %s", err)
		}

		return migrator.NewValidator(
			iotaAPI,
			ParamsReceipts.Validator.Coordinator.Address,
			ParamsReceipts.Validator.Coordinator.MerkleTreeDepth,
		)
	}); err != nil {
		Component.LogPanic(err)
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
		Component.LogPanic(err)
	}

	return nil
}

func configure() error {
	deps.Tangle.Events.NewReceipt.Hook(func(r *iotago.ReceiptMilestoneOpt) {
		if deps.ReceiptService.ValidationEnabled {
			Component.LogInfof("receipt passed validation against %s", ParamsReceipts.Validator.API.Address)
		}
		Component.LogInfof("new receipt processed (migrated_at %d, final %v, entries %d),", r.MigratedAt, r.Final, len(r.Funds))
	})
	Component.LogInfof("storing receipt backups in %s", ParamsReceipts.Backup.Path)
	if err := deps.ReceiptService.Init(); err != nil {
		Component.LogPanic(err)
	}

	return nil
}
