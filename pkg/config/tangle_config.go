package config

const (
	// the path to the database folder
	CfgDatabasePath = "db.path"
	// whether to auto. set LSM as LSMI
	CfgCompassLoadLSMIAsLMI = "compass.loadLSMIAsLMI"
)

func init() {
	NodeConfig.SetDefault(CfgDatabasePath, "mainnetdb")
	NodeConfig.SetDefault(CfgCompassLoadLSMIAsLMI, false)
}
