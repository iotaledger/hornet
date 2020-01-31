package autopeering

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_ENTRY_NODES = "autopeering.entryNodes"
)

func init() {
	flag.StringSlice(CFG_ENTRY_NODES, []string{"zEiNuQMDfZ6F8QDisa1ndX32ykBTyYCxbtkO0vkaWd0=@159.69.9.6:18626"}, "list of trusted entry nodes for auto peering")
}
