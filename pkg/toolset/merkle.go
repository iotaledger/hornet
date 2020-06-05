package toolset

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/coordinator"
)

func merkleTreeCreate(args []string) error {

	if len(args) > 0 {
		return errors.New("too many arguments for 'merkle'")
	}

	seed, err := config.LoadHashFromEnvironment("COO_SEED")
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

	ts := time.Now()
	if err = coordinator.CreateMerkleTreeFile(merkleFilePath, seed, secLvl, depth); err != nil {
		return err
	}

	fmt.Printf("successfully created merkle tree (took %v).\n", time.Since(ts).Truncate(time.Second))
	return nil
}
