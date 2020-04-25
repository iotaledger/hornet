package coordinator

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	ErrDepthTooSmall = errors.New("depth is set too low, must be >0")
)

// CreateMerkleTreeFile calculates a merkle tree and persists it into a file.
func CreateMerkleTreeFile(filePath string, seed trinary.Hash, securityLvl int, depth int) error {

	if depth < 1 {
		return ErrDepthTooSmall
	}

	addresses := calculateAllAddresses(seed, securityLvl, 1<<depth)
	layers := calculateAllLayers(addresses)

	mt := &MerkleTree{Depth: depth}
	mt.Layers = make(map[int]*MerkleTreeLayer)

	for i, layer := range layers {
		mt.Layers[i] = &MerkleTreeLayer{Level: i, Hashes: layer}
	}

	mt.Root = mt.Layers[0].Hashes[0]

	if err := storeMerkleTreeFile(filePath, mt); err != nil {
		return err
	}

	return nil
}

// calculateAllAddresses calculates all addresses that are used for the merkle tree of the coordinator.
func calculateAllAddresses(seed trinary.Hash, securityLvl int, count int) []trinary.Hash {
	fmt.Printf("calculating %d addresses...\n", count)

	resultLock := syncutils.Mutex{}
	result := make([]trinary.Hash, count)

	wg := sync.WaitGroup{}
	wg.Add(runtime.NumCPU())

	// calculate all addresses in parallel
	input := make(chan milestone.Index)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			defer wg.Done()

			for index := range input {
				address, err := Address(seed, index, securityLvl)
				if err != nil {
					panic(err)
				}
				resultLock.Lock()
				result[int(index)] = address
				resultLock.Unlock()
			}
		}()
	}

	ts := time.Now()
	for index := 0; index < count; index++ {
		input <- milestone.Index(index)

		if index%5000 == 0 && index != 0 {
			ratio := float64(index) / float64(count)
			total := time.Duration(float64(time.Since(ts)) / ratio)
			duration := time.Until(ts.Add(total))
			fmt.Printf("calculated %d/%d (%0.2f%%) addresses. %v left...\n", index, count, ratio*100.0, duration.Truncate(time.Second))
		}
	}

	close(input)
	wg.Wait()

	fmt.Printf("calculated %d/%d (100.00%%) addresses (took %v).\n", count, count, time.Since(ts).Truncate(time.Second))
	return result
}

// calculateAllLayers calculates all layers of the merkle tree used for coordinator signatures.
func calculateAllLayers(addresses []trinary.Hash) [][]trinary.Hash {
	depth := int64(math.Floor(math.Log2(float64(len(addresses)))))

	var layers [][]trinary.Hash

	last := addresses
	layers = append(layers, last)

	for i := depth - 1; i >= 0; i-- {
		fmt.Printf("calculating nodes for layer %d\n", i)
		last = calculateNextLayer(last)
		layers = append(layers, last)
	}

	// reverse the result
	for left, right := 0, len(layers)-1; left < right; left, right = left+1, right-1 {
		layers[left], layers[right] = layers[right], layers[left]
	}

	return layers
}

// calculateNextLayer calculates a single layer of the merkle tree used for coordinator signatures.
func calculateNextLayer(lastLayer []trinary.Hash) []trinary.Hash {

	resultLock := syncutils.Mutex{}
	result := make([]trinary.Hash, len(lastLayer)/2)

	wg := sync.WaitGroup{}
	wg.Add(runtime.NumCPU())

	// calculate all nodes in parallel
	input := make(chan int)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			defer wg.Done()

			for index := range input {
				sp := kerl.NewKerl()

				// merkle trees are calculated layer by layer by hashing two corresponding nodes of the last layer
				// https://en.wikipedia.org/wiki/Merkle_tree
				sp.AbsorbTrytes(lastLayer[index*2])
				sp.AbsorbTrytes(lastLayer[index*2+1])

				resultLock.Lock()
				result[index] = sp.MustSqueezeTrytes(consts.HashTrinarySize)
				resultLock.Unlock()
			}
		}()
	}

	for index := 0; index < len(lastLayer)/2; index++ {
		input <- index
	}

	close(input)
	wg.Wait()

	return result
}
