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

	switch strings.ToLower(args[0]) {
	case "create":
		merkleTreeCreate(args[1:])
	case "convert":
		merkleTreeConvert(args[1:])
	case "list":
		merkleListTools()
	default:
		fmt.Printf("Tool '%s' is no merkle tool. 'merkle list' will list all available merkle tools.\n", args[0])
		os.Exit(0)
	}
}

func merkleTreeCreate(args []string) {

	if len(args) > 0 {
		fmt.Printf("Too many arguments for 'merkle create'. Got %d, expected 0\n", len(args))
		os.Exit(0)
	}

	seed, err := coordinator.LoadSeedFromEnvironment()
	if err != nil {
		fmt.Println(err.Error())
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
		fmt.Println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func merkleTreeConvert(args []string) {

	if len(args) < 1 {
		fmt.Printf("Not enough arguments for 'merkle convert'. Got %d, expected 1\n", len(args))
		os.Exit(0)
	}

	if len(args) > 1 {
		fmt.Printf("Too many arguments for 'merkle convert'. Got %d, expected 1\n", len(args))
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
		fmt.Println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func merkleListTools() {
	fmt.Println("Available merkle tools:")
	fmt.Println("create: Calculate an new merkle tree")
	fmt.Println("convert <inputDir>: Convert a Compass created merkle tree into a HORNET compatible format")
	os.Exit(0)
}
