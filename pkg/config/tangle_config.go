package config

const (
	// the path to the database folder
	CfgDatabasePath = "db.path"
	// ignore the check for corrupted databases (should only be used for debug reasons)
	CfgDatabaseDebug = "db.debug"
)

func init() {
	NodeConfig.SetDefault(CfgDatabasePath, "mainnetdb")
}
