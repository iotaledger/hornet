package coordinator

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"

	"github.com/iotaledger/iota.go/trinary"
)

func ConvertMerkleTreeFiles(inputDir string, outputFilePath string) error {
	layersFromFile, err := loadLayers(inputDir)
	if err != nil {
		panic(err)
	}

	mt := &MerkleTree{Depth: depth}
	mt.Layers = make(map[int]*MerkleTreeLayer)

	for i, layer := range layersFromFile {
		mt.Layers[i] = &MerkleTreeLayer{Level: i, Hashes: layer}
	}

	mt.Root = mt.Layers[0].Hashes[0]

	if err := WriteMerkleTreeFile(outputFilePath, mt); err != nil {
		return err
	}

	return nil
}

func readLines(filePath string) ([]trinary.Hash, error) {

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []trinary.Hash

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		hash := scanner.Text()

		if err := trinary.ValidTrytes(hash); err != nil {
			return nil, err
		}

		result = append(result, hash)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func loadLayers(directory string) ([][]trinary.Hash, error) {
	layers := make(map[int][]trinary.Hash)
	var result [][]trinary.Hash

	validLayerFileName := regexp.MustCompile(`layer\.(\d+)\.csv`)

	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if validLayerFileName.Match([]byte(file.Name())) {
			groups := validLayerFileName.FindStringSubmatch(file.Name())
			idx, err := strconv.Atoi(groups[1])
			if err != nil {
				return nil, err
			}

			hashes, err := readLines(path.Join(directory, file.Name()))
			if err != nil {
				return nil, err
			}

			if len(hashes) != 1<<idx {
				return nil, errors.New("Wrong length")
			}
			// ToDo: failed to load layers from

			layers[idx] = hashes
		}
	}

	for idx := 0; idx <= depth; idx++ {
		layer, exists := layers[idx]
		if !exists {
			return nil, fmt.Errorf("Found a missing layer. please check: layers.%d.csv", idx)
		}
		result = append(result, layer)
	}

	return result, nil
}
