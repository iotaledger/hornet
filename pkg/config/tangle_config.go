package config

const (
	// the path to the database folder
	CfgDatabasePath     = "db.path"
	CfgDatabaseDebugLog = "db.debugLog"
)

func init() {
	NodeConfig.SetDefault(CfgDatabasePath, "mainnetdb")
	NodeConfig.SetDefault(CfgDatabaseDebugLog, false)
}
