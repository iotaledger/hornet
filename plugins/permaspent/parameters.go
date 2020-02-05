package permaspent

import "github.com/gohornet/hornet/packages/parameter"

func init() {
	parameter.NodeConfig.SetDefault("permaspent.nodes", []string{"https://permaspent.manapotion.io"})
	parameter.NodeConfig.SetDefault("permaspent.thresholds.noResponseTolerance", 0.25)
	parameter.NodeConfig.SetDefault("permaspent.thresholds.quorum", 0.65)
}
