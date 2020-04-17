package toolset

import (
	"fmt"
	"os"
	"strings"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/coordinator"
)

func merkleTree(args []string) {

	if len(args) < 1 {
		fmt.Println("Not enough arguments for 'merkle'")
		os.Exit(0)
	}

	if strings.ToLower(args[0]) == "create" {
		merkleTreeCreate(args[1:])
	}

	if strings.ToLower(args[0]) == "convert" {
		merkleTreeConvert(args[1:])
	}
}

func merkleTreeCreate(args []string) {

	if len(args) > 0 {
		fmt.Println("Too many arguments for 'merkle create'")
		os.Exit(0)
	}

	seed, err := coordinator.LoadSeedFromEnvironment()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}

	merkleFilePath := config.NodeConfig.GetString(config.CfgCoordinatorMerkleTreeFilePath)
	secLvl := config.NodeConfig.GetInt(config.CfgCoordinatorSecurityLevel)
	depth := config.NodeConfig.GetInt(config.CfgCoordinatorMerkleTreeDepth)

	if _, err := os.Stat(merkleFilePath); !os.IsNotExist(err) {
		// merkle tree file already exists
		fmt.Println("Error: Merkle tree file already exists. ", merkleFilePath)
		os.Exit(1)
	}

	coordinator.InitLogger("Coordinator")
	if err = coordinator.CreateMerkleTreeFile(merkleFilePath, seed, secLvl, depth); err != nil {
		println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func merkleTreeConvert(args []string) {

	if len(args) < 1 {
		fmt.Println("Not enough arguments for 'merkle convert'")
		os.Exit(0)
	}

	if len(args) > 1 {
		fmt.Println("Too many arguments for 'merkle convert'")
		os.Exit(0)
	}

	inputDir := args[0]
	merkleFilePath := config.NodeConfig.GetString(config.CfgCoordinatorMerkleTreeFilePath)

	dirInfo, err := os.Stat(inputDir)
	if os.IsNotExist(err) {
		fmt.Println("Error: Input directory does not exist. ", inputDir)
		os.Exit(1)
	}
	if !dirInfo.IsDir() {
		fmt.Println("Error: Input directory is not a directory. ", inputDir)
		os.Exit(1)
	}

	if _, err := os.Stat(merkleFilePath); !os.IsNotExist(err) {
		// merkle tree file already exists
		fmt.Println("Error: Merkle tree file already exists. ", merkleFilePath)
		os.Exit(1)
	}

	coordinator.InitLogger("Coordinator")
	if err := coordinator.ConvertMerkleTreeFiles(inputDir, merkleFilePath); err != nil {
		println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
