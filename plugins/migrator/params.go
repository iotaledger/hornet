package migrator

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/model/migrator"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgMigratorStateFilePath configures the path to the state file of the migrator.
	CfgMigratorStateFilePath = "migrator.stateFilePath"
	// CfgMigratorReceiptMaxEntries defines the max amount of entries to embed within a receipt.
	CfgMigratorReceiptMaxEntries = "migrator.receiptMaxEntries"
	// CfgMigratorQueryCooldownPeriod configures the cooldown period for the service to ask for new data
	// from the legacy node in case the migrator encounters an error.
	CfgMigratorQueryCooldownPeriod = "migrator.queryCooldownPeriod"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgMigratorStateFilePath, "migrator.state", "path to the state file of the migrator")
			fs.Int(CfgMigratorReceiptMaxEntries, migrator.SensibleMaxEntriesCount, "the max amount of entries to embed within a receipt")
			fs.Duration(CfgMigratorQueryCooldownPeriod, 5*time.Second, "the cooldown period of the service to ask for new data")
			return fs
		}(),
	},
	Masked: nil,
}
