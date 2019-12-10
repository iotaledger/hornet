package tangle

import flag "github.com/spf13/pflag"

func init() {
	flag.String("db.path", "mainnetdb", "Path to the database folder")
	flag.Bool("light", false, "Enable the light mode for nodes with less RAM")
	flag.Bool("compass.loadLSMIAsLMI", false, "Auto. set LSM as LSMI if enabled")
}
