package tangle

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sync"

	"github.com/gohornet/hornet/packages/parameter"
	"github.com/pkg/errors"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

const (
	spentAddressesFileName = "spent_addresses_v3.gz.bin"
)

var (
	spentAddressesLock sync.RWMutex
)

func ReadLockSpentAddresses() {
	spentAddressesLock.RLock()
}

func ReadUnlockSpentAddresses() {
	spentAddressesLock.RUnlock()
}

func WriteLockSpentAddresses() {
	spentAddressesLock.Lock()
}

func WriteUnlockSpentAddresses() {
	spentAddressesLock.Unlock()
}

func loadSpentAddressesCuckooFilter() *cuckoo.Filter {
	dbPath := parameter.NodeConfig.GetString("db.path")
	filePath := path.Join(dbPath, spentAddressesFileName)

	// return a new filter if the file doesn't exist
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			// create a new one as the file doesn't exist
			return cuckoo.NewFilter(CuckooFilterSize)
		}
		log.Panicf("unable to stat %s: %v", filePath, err)
	}

	// file exists
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0660)
	if err != nil {
		log.Panicf("unable to read %s: %v", filePath, err)
	}
	defer f.Close()
	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		log.Panicf("unable to create gzip reader for %s: %v", filePath, err)
	}
	defer gzipReader.Close()
	cfBytes, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		log.Panicf("unable to read %s: %v", filePath, err)
	}
	cf, err := cuckoo.Decode(cfBytes)
	if err != nil {
		panic(err)
	}
	return cf
}

// Serializes the cuckoo filter holding the spent addresses to its byte representation.
// The spent addresses lock should be held while calling this function.
func SerializedSpentAddressesCuckooFilter() []byte {
	return SpentAddressesCuckooFilter.Encode()
}

// Imports the given cuckoo filter by persisting its serialized representation to the disk in the database folder.
func ImportSpentAddressesCuckooFilter(cf *cuckoo.Filter) error {
	spentAddressesLock.Lock()
	defer spentAddressesLock.Unlock()

	dbPath := parameter.NodeConfig.GetString("db.path")
	filePath := path.Join(dbPath, spentAddressesFileName)
	tempFilePath := path.Join(dbPath, fmt.Sprintf("tmp_%s", spentAddressesFileName))

	cfBytes := cf.Encode()

	// remove temp file just in case it already exists for whatever reason
	os.Remove(tempFilePath)

	file, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		return err
	}

	gzipWriter := gzip.NewWriter(file)
	if _, err := gzipWriter.Write(cfBytes); err != nil {
		gzipWriter.Close()
		file.Close()
		return err
	}

	gzipWriter.Close()
	file.Close()

	if err := os.Rename(tempFilePath, filePath); err != nil {
		return errors.Wrap(err, "can't rename temp file to target")
	}

	os.Remove(tempFilePath)
	return nil
}

// Stores the package local cuckoo filter to the disk in the database folder.
func StoreSpentAddressesCuckooFilterInDatabase() error {
	return ImportSpentAddressesCuckooFilter(SpentAddressesCuckooFilter)
}

// CountSpentAddressesEntries returns the amount of spent addresses.
// ReadLockSpentAddresses must be held while entering this function.
func CountSpentAddressesEntries() int32 {
	spentAddressesLock.RLock()
	defer spentAddressesLock.RUnlock()
	return int32(SpentAddressesCuckooFilter.Count())
}
