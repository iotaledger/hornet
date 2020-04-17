package toolset

import (
	"errors"
	"fmt"
	"os"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/coordinator"
)

func merkleTreeCreate(args []string) error {

	if len(args) > 0 {
		return errors.New("too many arguments for 'merkle'")
	}

	seed, err := coordinator.LoadSeedFromEnvironment()
	if err != nil {
		return err
	}

	merkleFilePath := config.NodeConfig.GetString(config.CfgCoordinatorMerkleTreeFilePath)
	secLvl := config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel)
	depth := config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth)

	if _, err := os.Stat(merkleFilePath); !os.IsNotExist(err) {
		// merkle tree file already exists
		return fmt.Errorf("merkle tree file already exists. %v", merkleFilePath)
	}

	if err = coordinator.CreateMerkleTreeFile(merkleFilePath, seed, secLvl, depth); err != nil {
		return err
	}

	return nil
}
