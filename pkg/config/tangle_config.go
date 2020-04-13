package config

const (
	// the path to the database folder
	CfgDatabasePath = "db.path"
)

func init() {
	NodeConfig.SetDefault(CfgDatabasePath, "mainnetdb")
}
