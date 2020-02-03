package autopeering

import (
	"github.com/gohornet/hornet/packages/parameter"
)

const (
	CFG_ENTRY_NODES = "autopeering.entryNodes"
)

func init() {
	// "list of trusted entry nodes for auto peering"
	parameter.NodeConfig.SetDefault(CFG_ENTRY_NODES, []string{"zEiNuQMDfZ6F8QDisa1ndX32ykBTyYCxbtkO0vkaWd0=@159.69.9.6:18626"})
}
