package tangle

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/database"

	hornetDB "github.com/gohornet/hornet/packages/database"
	"github.com/gohornet/hornet/packages/model/hornet"
)

var bundleDatabase database.Database

func configureBundleDatabase() {
	if db, err := database.Get(DBPrefixBundles, hornetDB.GetBadgerInstance()); err != nil {
		panic(err)
	} else {
		bundleDatabase = db
	}
}

func databaseKeyForBundle(bundleHash trinary.Hash, txHash trinary.Hash) []byte {
	return append(databaseKeyPrefixForBundleHash(bundleHash), trinary.MustTrytesToBytes(txHash)...)
}

func databaseKeyPrefixForBundleHash(bundleHash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(bundleHash)
}

func StoreBundleBucketsInDatabase(bundleBuckets []*BundleBucket) error {

	// Create entries for all txs in all bundles
	var entries []database.Entry
	tails := map[trinary.Hash]struct{}{}
	for _, bundleBucket := range bundleBuckets {

		for _, bundle := range bundleBucket.Bundles() {
			if !bundle.IsModified() {
				continue
			}
			tails[bundle.tailTx] = struct{}{}
			// we store the bundle metadata in the tail tx
			entry := database.Entry{
				Key:   databaseKeyForBundle(bundleBucket.GetHash(), bundle.tailTx),
				Value: []byte{},
				Meta:  bundle.GetMetadata(),
			}
			entries = append(entries, entry)
			bundle.SetModified(false)
		}

		transactions := bundleBucket.Transactions()

		for _, tx := range transactions {
			// tails were already stored
			if tx.GetTransaction().IsTail() {
				continue
			}

			entry := database.Entry{
				Key:   databaseKeyForBundle(bundleBucket.GetHash(), tx.GetTransaction().GetHash()),
				Value: []byte{},
				Meta:  byte(0),
			}
			entries = append(entries, entry)
		}

		transactions.Release()
	}

	// Now batch insert all entries
	if err := bundleDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store bundles")
	}

	return nil
}

func StoreBundleInDatabase(bundle *Bundle) error {
	if !bundle.IsModified() {
		return nil
	}

	var entries []database.Entry
	txHashes := bundle.GetTransactionHashes()
	for _, txHash := range txHashes {
		var entry database.Entry
		if txHash == bundle.tailTx {
			entry = database.Entry{
				Key:   databaseKeyForBundle(bundle.GetHash(), txHash),
				Value: []byte{},
				Meta:  bundle.GetMetadata(),
			}
		} else {
			entry = database.Entry{
				Key:   databaseKeyForBundle(bundle.GetHash(), txHash),
				Value: []byte{},
				Meta:  byte(0),
			}
		}
		entries = append(entries, entry)
	}
	if err := bundleDatabase.Apply(entries, []database.Key{}); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store bundle")
	}

	bundle.SetModified(false)

	return nil
}

func DeleteBundlesInDatabase(bundles map[string]string) error {
	var deletions []database.Key

	for bundleHash, txHash := range bundles {
		deletions = append(deletions, databaseKeyForBundle(bundleHash, txHash))
	}

	// Now batch delete all entries
	if err := bundleDatabase.Apply([]database.Entry{}, deletions); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to delete bundles")
	}

	return nil
}

func readBundleBucketFromDatabase(bundleHash trinary.Hash) (*BundleBucket, error) {

	var transactions = map[trinary.Hash]*CachedTransaction{}
	metaMap := map[trinary.Hash]bitutils.BitMask{}
	err := bundleDatabase.ForEachPrefixKeyOnly(databaseKeyPrefixForBundleHash(bundleHash), func(entry database.KeyOnlyEntry) (stop bool) {
		txHash := trinary.MustBytesToTrytes(entry.Key, 81)
		tx, _ := GetCachedTransaction(txHash)
		if tx.Exists() {
			if tx.GetTransaction().Tx.CurrentIndex == 0 {
				metaMap[tx.GetTransaction().GetHash()] = bitutils.BitMask(entry.Meta)
			}
			transactions[tx.GetTransaction().GetHash()] = tx
		} else {
			tx.Release()
		}
		return false
	})

	if err != nil {
		for _, tx := range transactions {
			tx.Release()
		}
		return nil, errors.Wrap(NewDatabaseError(err), "failed to read bundle bucket from database")
	} else if len(transactions) == 0 {
		return nil, nil
	} else {
		bucket := NewBundleBucketFromDatabase(bundleHash, transactions, metaMap)
		for _, tx := range transactions {
			tx.Release()
		}
		return bucket, nil
	}
}

func databaseContainsBundle(bundleHash trinary.Hash) (bool, error) {
	if contains, err := bundleDatabase.Contains(databaseKeyPrefixForBundleHash(bundleHash)); err != nil {
		return contains, errors.Wrap(NewDatabaseError(err), "failed to check if the bundle exists")
	} else {
		return contains, nil
	}
}
