package coordinator

import (
	"bufio"
	"encoding/binary"
	"os"

	"github.com/iotaledger/iota.go/trinary"
)

type MerkleTree struct {
	Depth  int
	Root   trinary.Hash
	Layers map[int]*MerkleTreeLayer
}

func (mt *MerkleTree) Marshal(buf *bufio.Writer) (err error) {

	/*
	 4 bytes uint32 			depth
	 4 bytes uint32  		    lengthLayers
	*/

	if err := binary.Write(buf, binary.LittleEndian, uint32(mt.Depth)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(mt.Layers))); err != nil {
		return err
	}

	for layer := 0; layer < len(mt.Layers); layer++ {
		merkleTreeLayer := mt.Layers[layer]

		if err := merkleTreeLayer.Marshal(buf); err != nil {
			return err
		}
	}

	return nil
}

func (mt *MerkleTree) Unmarshal(buf *bufio.Reader) error {

	/*
	 4 bytes uint32 			depth
	 4 bytes uint32  		    lengthLayers
	*/

	var depth uint32
	if err := binary.Read(buf, binary.LittleEndian, &depth); err != nil {
		return err
	}
	mt.Depth = int(depth)

	var lengthLayers uint32
	if err := binary.Read(buf, binary.LittleEndian, &lengthLayers); err != nil {
		return err
	}

	mt.Layers = make(map[int]*MerkleTreeLayer)

	for i := 0; i < int(lengthLayers); i++ {
		mtl := &MerkleTreeLayer{}

		if err := mtl.Unmarshal(buf); err != nil {
			return err
		}

		mt.Layers[mtl.Level] = mtl
	}

	mt.Root = mt.Layers[0].Hashes[0]

	return nil
}

type MerkleTreeLayer struct {
	Level  int
	Hashes []trinary.Hash
}

func (mtl *MerkleTreeLayer) Marshal(buf *bufio.Writer) error {

	if err := binary.Write(buf, binary.LittleEndian, uint32(mtl.Level)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(mtl.Hashes))); err != nil {
		return err
	}

	for _, hash := range mtl.Hashes {
		if err := binary.Write(buf, binary.LittleEndian, trinary.MustTrytesToBytes(hash)[:49]); err != nil {
			return err
		}
	}

	return nil
}

func (mtl *MerkleTreeLayer) Unmarshal(buf *bufio.Reader) error {

	var level uint32
	if err := binary.Read(buf, binary.LittleEndian, &level); err != nil {
		return err
	}
	mtl.Level = int(level)

	var hashesCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &hashesCount); err != nil {
		return err
	}

	hashBuf := make([]byte, 49)
	for i := 0; i < int(hashesCount); i++ {
		if err := binary.Read(buf, binary.LittleEndian, hashBuf); err != nil {
			return err
		}

		mtl.Hashes = append(mtl.Hashes, trinary.MustBytesToTrytes(hashBuf, 81))
	}

	return nil
}

// storeMerkleTreeFile stores the merkle tree in a file
func storeMerkleTreeFile(filePath string, merkleTree *MerkleTree) error {

	outputFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// write into the file with an 8kB buffer
	fileBufWriter := bufio.NewWriterSize(outputFile, 8192)

	if err := merkleTree.Marshal(fileBufWriter); err != nil {
		return err
	}

	if err := fileBufWriter.Flush(); err != nil {
		return err
	}

	return nil
}

// loadMerkleTreeFile load the merkle tree from a file
func loadMerkleTreeFile(filePath string) (*MerkleTree, error) {

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := bufio.NewReader(file)

	merkleTree := &MerkleTree{}
	if err := merkleTree.Unmarshal(buf); err != nil {
		return nil, err
	}

	return merkleTree, nil
}
