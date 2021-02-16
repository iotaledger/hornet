package receipt

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/iota.go/api"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/logger"
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
	}
	if err := c.Provide(func(deps serviceDependencies) (*migrator.ReceiptService, error) {
		return migrator.NewReceiptService(
			deps.Validator,
			deps.NodeConfig.Bool(CfgReceiptsValidatorValidate),
			deps.NodeConfig.String(CfgReceiptsFolderPath),
		), nil
	}); err != nil {
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)
	if err := deps.ReceiptService.Init(); err != nil {
		panic(err)
	}

	numReceipts, err := deps.ReceiptService.NumReceiptsStored()
	if err != nil {
		panic(err)
	}

	log.Infof("stored receipts: %d", numReceipts)
}
