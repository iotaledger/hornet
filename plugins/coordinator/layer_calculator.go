package coordinator

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	depth = 10
)

func CreateCoordinatorFiles(outputDir string, seed trinary.Hash, securityLvl int, depth int) {

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		// outputDor does not exist
		os.MkdirAll(outputDir, os.ModePerm)
	}

	addresses := calculateAllAddresses(seed, securityLvl, 1<<depth)
	log.Info("Calculated all addresses.")

	layers := calculateAllLayers(addresses)

	for i, layer := range layers {
		if err := writeLayer(outputDir, i, layer); err != nil {
			panic(err)
		}
	}

	log.Infof("Successfully wrote Merkle Tree with root: %v", layers[0][0])
}

func CreateMerkleTreeFile(filePath string, seed trinary.Hash, securityLvl int, depth int) error {

	addresses := calculateAllAddresses(seed, securityLvl, 1<<depth)
	log.Info("Calculated all addresses.")

	layers := calculateAllLayers(addresses)

	mt := &MerkleTree{Depth: depth}
	mt.Layers = make(map[int]*MerkleTreeLayer)

	for i, layer := range layers {
		mt.Layers[i] = &MerkleTreeLayer{Level: i, Hashes: layer}
	}

	mt.Root = mt.Layers[0].Hashes[0]

	if err := WriteMerkleTreeFile(filePath, mt); err != nil {
		return err
	}

	fmt.Printf("Creating merkle tree successful (took %v).\n", time.Since(ts).Truncate(time.Second))

	return nil
}

func calculateAllAddresses(seed trinary.Hash, securityLvl int, count int) []trinary.Hash {
	fmt.Printf("Calculating %d addresses...\n", count)

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
				address, err := GetAddress(seed, index, securityLvl)
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
			duration := ts.Add(total).Sub(time.Now())
			fmt.Printf("Calculated %d/%d (%0.2f%%) addresses. %v left...\n", index, count, ratio*100.0, duration.Truncate(time.Second))
		}
	}

	close(input)
	wg.Wait()

	fmt.Printf("Calculated %d/%d (100.00%%) addresses (took %v).\n", count, count, time.Since(ts).Truncate(time.Second))

	return result
}

func calculateAllLayers(addresses []trinary.Hash) [][]trinary.Hash {
	depth := int64(math.Floor(math.Log2(float64(len(addresses)))))

	var layers [][]trinary.Hash

	last := addresses
	layers = append(layers, last)

	for i := depth - 1; i >= 0; i-- {
		fmt.Printf("Calculating nodes for layer %d\n", i)
		last = calculateNextLayer(last)
		layers = append(layers, last)
	}

	// reverse the result
	for left, right := 0, len(layers)-1; left < right; left, right = left+1, right-1 {
		layers[left], layers[right] = layers[right], layers[left]
	}

	return layers
}

func calculateNextLayer(lastLayer []trinary.Hash) []trinary.Hash {

	resultLock := syncutils.Mutex{}
	result := make([]trinary.Hash, len(lastLayer)/2)

	wg := sync.WaitGroup{}
	wg.Add(runtime.NumCPU())

	// calculate all layers in parallel
	input := make(chan int)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			defer wg.Done()

			for index := range input {
				sp := kerl.NewKerl()
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

func writeLayer(outputDir string, depth int, layer []trinary.Hash) error {

	filePath := path.Join(outputDir, fmt.Sprintf("layer.%d.csv", depth))

	outputFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// write into the file with an 8kB buffer
	fileBufWriter := bufio.NewWriterSize(outputFile, 8192)

	for _, element := range layer {
		fileBufWriter.WriteString(element + "\n")
	}

	if err := fileBufWriter.Flush(); err != nil {
		return err
	}

	return nil
}
