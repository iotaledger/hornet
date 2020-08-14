package toolset

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/merkle"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	// printStatusInterval is the interval for printing status messages
	printStatusInterval = 2 * time.Second
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
		// Merkle tree file already exists
		return fmt.Errorf("Merkle tree file already exists. %v", merkleFilePath)
	}

	count := 1 << depth

	ts := time.Now()
	lastStatusTime := time.Now()

	calculateAddressesStartCallback := func(count uint32) {
		fmt.Printf("calculating %d addresses...\n", count)
	}

	calculateAddressesCallback := func(index uint32) {
		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			percentage, remaining := utils.EstimateRemainingTime(ts, int64(index), int64(count))
			fmt.Printf("calculated %d/%d (%0.2f%%) addresses. %v left...\n", index, count, percentage, remaining.Truncate(time.Second))
		}
	}

	calculateAddressesFinishedCallback := func(count uint32) {
		fmt.Printf("calculated %d/%d (100.00%%) addresses (took %v).\n", count, count, time.Since(ts).Truncate(time.Second))
	}

	calculateLayersCallback := func(index uint32) {
		fmt.Printf("calculating nodes for layer %d\n", index)
	}

	mt, err := merkle.CreateMerkleTree(seed, consts.SecurityLevel(secLvl), depth,
		merkle.MerkleCreateOptions{
			CalculateAddressesStartCallback:    calculateAddressesStartCallback,
			CalculateAddressesCallback:         calculateAddressesCallback,
			CalculateAddressesFinishedCallback: calculateAddressesFinishedCallback,
			CalculateLayersCallback:            calculateLayersCallback,
			Parallelism:                        runtime.NumCPU(),
		})

	if err != nil {
		return fmt.Errorf("error creating Merkle tree: %v", err)
	}

	if err := merkle.StoreMerkleTreeFile(merkleFilePath, mt); err != nil {
		return fmt.Errorf("error persisting Merkle tree: %v", err)
	}

	fmt.Printf("Merkle tree root: %v\n", mt.Root)

	fmt.Printf("successfully created Merkle tree (took %v).\n", time.Since(ts).Truncate(time.Second))

	return nil
}
