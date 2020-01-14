package spa

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/guards"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
	"github.com/pkg/errors"
)

type ExplorerTx struct {
	Hash                          Hash   `json:"hash"`
	SignatureMessageFragment      Trytes `json:"signature_message_fragment"`
	Address                       Hash   `json:"address"`
	Value                         int64  `json:"value"`
	ObsoleteTag                   Trytes `json:"obsolete_tag"`
	Timestamp                     uint64 `json:"timestamp"`
	CurrentIndex                  uint64 `json:"current_index"`
	LastIndex                     uint64 `json:"last_index"`
	Bundle                        Hash   `json:"bundle"`
	Trunk                         Hash   `json:"trunk"`
	Branch                        Hash   `json:"branch"`
	Tag                           Trytes `json:"tag"`
	Nonce                         Trytes `json:"nonce"`
	AttachmentTimestamp           int64  `json:"attachment_timestamp"`
	AttachmentTimestampLowerBound int64  `json:"attachment_timestamp_lower_bound"`
	AttachmentTimestampUpperBound int64  `json:"attachment_timestamp_upper_bound"`
	Confirmed                     struct {
		State     bool                           `json:"state"`
		Milestone milestone_index.MilestoneIndex `json:"milestone_index"`
	} `json:"confirmed"`
	Solid          bool                           `json:"solid"`
	MWM            int                            `json:"mwm"`
	Previous       Hash                           `json:"previous"`
	Next           Hash                           `json:"next"`
	BundleComplete bool                           `json:"bundle_complete"`
	IsMilestone    bool                           `json:"is_milestone"`
	MilestoneIndex milestone_index.MilestoneIndex `json:"milestone_index"`
}

func createExplorerTx(hash Hash, tx *tangle.CachedTransaction) (*ExplorerTx, error) {
	
	tx.RegisterConsumer() //+1
	defer tx.Release()    //-1

	originTx := tx.GetTransaction().Tx
	confirmed, by := tx.GetTransaction().GetConfirmed()
	t := &ExplorerTx{
		Hash:                          hash,
		SignatureMessageFragment:      originTx.SignatureMessageFragment,
		Address:                       originTx.Address,
		ObsoleteTag:                   originTx.ObsoleteTag,
		Timestamp:                     originTx.Timestamp,
		CurrentIndex:                  originTx.CurrentIndex,
		Value:                         originTx.Value,
		LastIndex:                     originTx.LastIndex,
		Bundle:                        originTx.Bundle,
		Trunk:                         originTx.TrunkTransaction,
		Branch:                        originTx.BranchTransaction,
		Tag:                           originTx.Tag,
		Nonce:                         originTx.Nonce,
		AttachmentTimestamp:           originTx.AttachmentTimestamp,
		AttachmentTimestampLowerBound: originTx.AttachmentTimestampLowerBound,
		AttachmentTimestampUpperBound: originTx.AttachmentTimestampUpperBound,
		Confirmed: struct {
			State     bool                           `json:"state"`
			Milestone milestone_index.MilestoneIndex `json:"milestone_index"`
		}{confirmed, by},
		Solid: tx.GetTransaction().IsSolid(),
	}

	// compute mwm
	trits, err := TrytesToTrits(hash)
	if err != nil {
		return nil, err
	}
	var mwm int
	for i := len(trits) - 1; i >= 0; i-- {
		if trits[i] == 0 {
			mwm++
			continue
		}
		break
	}
	t.MWM = mwm

	// compute previous and next in bundle
	bucket, err := tangle.GetBundleBucket(t.Bundle)
	if err != nil {
		return nil, ErrInternalError
	}

	// get previous/next hash
	var bndl *tangle.Bundle
	if tx.GetTransaction().IsTail() {
		bndl = bucket.GetBundleOfTailTransaction(hash)
	} else {
		bndls := bucket.GetBundlesOfTransaction(hash)
		if len(bndls) > 0 {
			bndl = bndls[0]
		}
	}

	if bndl != nil {
		t.BundleComplete = bndl.IsComplete()
		transactions := bndl.GetTransactions() //+1
		for _, bndlTx := range transactions {
			if bndlTx.GetTransaction().Tx.CurrentIndex+1 == t.CurrentIndex {
				t.Previous = bndlTx.GetTransaction().Tx.Hash
			} else if bndlTx.GetTransaction().Tx.CurrentIndex-1 == t.CurrentIndex {
				t.Next = bndlTx.GetTransaction().Tx.Hash
			}
		}
		transactions.Release() //-1

		// check whether milestone
		if bndl.IsMilestone() {
			t.IsMilestone = true
			t.MilestoneIndex = bndl.GetMilestoneIndex()
		}
	}

	return t, nil
}

type ExplorerAdress struct {
	Balance uint64        `json:"balance"`
	Txs     []*ExplorerTx `json:"txs"`
	Spent   bool          `json:"spent"`
}

type SearchResult struct {
	Tx        *ExplorerTx     `json:"tx"`
	Address   *ExplorerAdress `json:"address"`
	Bundles   [][]*ExplorerTx `json:"bundles"`
	Milestone *ExplorerTx     `json:"milestone"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {

	routeGroup.GET("/tx/:hash", func(c echo.Context) error {
		t, err := findTransaction(c.Param("hash"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/bundle/:hash", func(c echo.Context) error {
		bndls, err := findBundles(c.Param("hash"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, bndls)
	})

	routeGroup.GET("/addr/:hash", func(c echo.Context) error {
		addr, err := findAddress(c.Param("hash"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, addr)
	})

	routeGroup.GET("/milestone/:index", func(c echo.Context) error {
		indexStr := c.Param("index")
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return errors.Wrapf(ErrInvalidParameter, "%s is not a valid index", indexStr)
		}
		msTailTx, err := findMilestone(milestone_index.MilestoneIndex(index))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, msTailTx)
	})

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := c.Param("search")
		result := &SearchResult{}

		// milestone query
		index, err := strconv.Atoi(search)
		if err == nil {
			msTailTx, err := findMilestone(milestone_index.MilestoneIndex(index))
			if err == nil {
				result.Milestone = msTailTx
			}
			return c.JSON(http.StatusOK, result)
		}

		if len(search) < 81 {
			return errors.Wrapf(ErrInvalidParameter, "search hash invalid: %s", search)
		}

		// auto. remove checksum
		search = search[:81]

		wg := sync.WaitGroup{}
		wg.Add(3)
		go func() {
			defer wg.Done()
			tx, err := findTransaction(search)
			if err == nil {
				result.Tx = tx
			}
		}()

		go func() {
			defer wg.Done()
			addr, err := findAddress(search)
			if err == nil {
				result.Address = addr
			}
		}()

		go func() {
			defer wg.Done()
			bundles, err := findBundles(search)
			if err == nil {
				result.Bundles = bundles
			}
		}()
		wg.Wait()

		return c.JSON(http.StatusOK, result)
	})
}

func findMilestone(index milestone_index.MilestoneIndex) (*ExplorerTx, error) {
	bndl, err := tangle.GetMilestone(index)
	if err != nil {
		return nil, err
	}
	if bndl == nil {
		return nil, errors.Wrapf(ErrNotFound, "milestone %d unknown", index)
	}
	tail := bndl.GetTail() //+1
	tx, err := createExplorerTx(tail.GetTransaction().GetHash(), tail)
	tail.Release() //-1
	return tx, err
}

func findTransaction(hash Hash) (*ExplorerTx, error) {
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	tx := tangle.GetCachedTransaction(hash) //+1
	if !tx.Exists() {
		tx.Release() //-1
		return nil, errors.Wrapf(ErrNotFound, "tx %s unknown", hash)
	}

	t, err := createExplorerTx(hash, tx)
	tx.Release() //-1
	return t, err
}

func findBundles(hash Hash) ([][]*ExplorerTx, error) {
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	bucket, err := tangle.GetBundleBucket(hash)
	if err != nil {
		return nil, ErrInternalError
	}

	bndls := bucket.Bundles()
	if len(bndls) == 0 {
		return nil, errors.Wrapf(ErrNotFound, "bundle %s unknown", hash)
	}

	expBndls := [][]*ExplorerTx{}
	for _, bndl := range bndls {
		sl := []*ExplorerTx{}
		transactions := bndl.GetTransactions() //+1
		for _, tx := range transactions {
			expTx, err := createExplorerTx(tx.GetTransaction().GetHash(), tx)
			if err != nil {
				transactions.Release() //-1
				return nil, err
			}
			sl = append(sl, expTx)
		}
		transactions.Release() //-1
		expBndls = append(expBndls, sl)
	}
	return expBndls, nil
}

func findAddress(hash Hash) (*ExplorerAdress, error) {
	if len(hash) > 81 {
		hash = hash[:81]
	}
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	txHashes, err := tangle.ReadTransactionHashesForAddressFromDatabase(hash, 100)
	if err != nil {
		return nil, ErrInternalError
	}

	txs := make([]*ExplorerTx, 0, len(txHashes))
	if len(txHashes) != 0 {
		for i := 0; i < len(txHashes); i++ {
			txHash := txHashes[i]
			tx := tangle.GetCachedTransaction(txHash) //+1
			if !tx.Exists() {
				tx.Release() //-1
				return nil, errors.Wrapf(ErrNotFound, "tx %s not found but associated to address %s", txHash, hash)
			}
			expTx, err := createExplorerTx(tx.GetTransaction().GetHash(), tx)
			tx.Release() //-1
			if err != nil {
				return nil, err
			}
			txs = append(txs, expTx)
		}
	}

	balance, _, err := tangle.GetBalanceForAddress(hash)
	if err != nil {
		return nil, err
	}

	if len(txHashes) == 0 && balance == 0 {
		return nil, errors.Wrapf(ErrNotFound, "address %s not found", hash)
	}

	return &ExplorerAdress{Balance: balance, Txs: txs, Spent: tangle.WasAddressSpentFrom(hash)}, nil
}
