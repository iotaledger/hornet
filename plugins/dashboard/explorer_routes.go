package dashboard

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/muxxer/iota.go/consts"
	"github.com/muxxer/iota.go/guards"
	"github.com/muxxer/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

const (
	MaxTransactionsForAddressResults = 100
	MaxChildrenResults               = 100
	MaxTagResults                    = 100
	MaxBundleResults                 = 100
)

type ExplorerTx struct {
	Hash                          trinary.Hash   `json:"hash"`
	SignatureMessageFragment      trinary.Trytes `json:"signature_message_fragment"`
	Address                       trinary.Hash   `json:"address"`
	Value                         int64          `json:"value"`
	ObsoleteTag                   trinary.Trytes `json:"obsolete_tag"`
	Timestamp                     uint64         `json:"timestamp"`
	CurrentIndex                  uint64         `json:"current_index"`
	LastIndex                     uint64         `json:"last_index"`
	Bundle                        trinary.Hash   `json:"bundle"`
	Trunk                         trinary.Hash   `json:"trunk"`
	Branch                        trinary.Hash   `json:"branch"`
	Tag                           trinary.Trytes `json:"tag"`
	Nonce                         trinary.Trytes `json:"nonce"`
	AttachmentTimestamp           int64          `json:"attachment_timestamp"`
	AttachmentTimestampLowerBound int64          `json:"attachment_timestamp_lower_bound"`
	AttachmentTimestampUpperBound int64          `json:"attachment_timestamp_upper_bound"`
	Confirmed                     struct {
		State       bool            `json:"state"`
		Conflicting bool            `json:"conflicting"`
		Milestone   milestone.Index `json:"milestone_index"`
	} `json:"confirmed"`
	Children       []string        `json:"children"`
	Solid          bool            `json:"solid"`
	MWM            int             `json:"mwm"`
	Previous       trinary.Hash    `json:"previous"`
	Next           trinary.Hash    `json:"next"`
	BundleComplete bool            `json:"bundle_complete"`
	IsMilestone    bool            `json:"is_milestone"`
	MilestoneIndex milestone.Index `json:"milestone_index"`
}

func createExplorerTx(cachedMsg *tangle.CachedMessage) (*ExplorerTx, error) {

	defer cachedMsg.Release(true) // msg -1

	originTx := cachedMsg.GetMessage().Tx
	confirmed, by := cachedMsg.GetMetadata().GetConfirmed()
	conflicting := cachedMsg.GetMetadata().IsConflicting()
	t := &ExplorerTx{
		Hash:                          originTx.Hash,
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
			State       bool            `json:"state"`
			Conflicting bool            `json:"conflicting"`
			Milestone   milestone.Index `json:"milestone_index"`
		}{confirmed, conflicting, by},
		Solid: cachedMsg.GetMetadata().IsSolid(),
	}

	// Children
	t.Children = tangle.GetChildrenMessageIDs(cachedMsg.GetMessage().GetMessageID(), MaxChildrenResults).Hex()

	// compute mwm
	trits, err := trinary.BytesToTrits(cachedMsg.GetMessage().GetMessageID())
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

	// get previous/next hash
	var cachedMessage *tangle.CachedMessage
	if cachedMsg.GetMessage().IsTail() {
		cachedMessage = tangle.GetCachedMessageOrNil(cachedMsg.GetMessage().GetMessageID()) // message +1
	} else {
		cachedMessages := tangle.GetBundlesOfTransactionOrNil(cachedMsg.GetMessage().GetMessageID(), true) // message +1
		if cachedMessages != nil {
			cachedMessage = cachedMessages[0]

			// Release unused bundles
			for i := 1; i < len(cachedMessages); i++ {
				cachedMessages[i].Release(true) // message -1
			}
		}
	}

	if cachedMessage != nil {
		t.BundleComplete = true
		cachedMsgs := cachedMessage.GetMessage().GetTransactions() // msg +1
		for _, cachedMessageTx := range cachedMsgs {
			if cachedMessageTx.GetTransaction().Tx.CurrentIndex+1 == t.CurrentIndex {
				t.Previous = cachedMessageTx.GetTransaction().Tx.Hash
			} else if cachedMessageTx.GetTransaction().Tx.CurrentIndex-1 == t.CurrentIndex {
				t.Next = cachedMessageTx.GetTransaction().Tx.Hash
			}
		}
		cachedMsgs.Release(true) // msg -1

		// check whether milestone
		if cachedMessage.GetMessage().IsMilestone() {
			t.IsMilestone = true
			t.MilestoneIndex = cachedMessage.GetMessage().GetMilestoneIndex()
		}
		cachedMessage.Release(true) // message -1
	}

	return t, nil
}

type ExplorerTag struct {
	Txs []*ExplorerTx `json:"txs"`
}

type ExplorerAddress struct {
	Balance uint64        `json:"balance"`
	Txs     []*ExplorerTx `json:"txs"`
}

type SearchResult struct {
	Tx        *ExplorerTx      `json:"tx"`
	Tag       *ExplorerTag     `json:"tag"`
	Address   *ExplorerAddress `json:"address"`
	Bundles   [][]*ExplorerTx  `json:"bundles"`
	Milestone *ExplorerTx      `json:"milestone"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {

	routeGroup.GET("/tx/:hash", func(c echo.Context) error {
		hash := strings.ToUpper(c.Param("hash"))
		t, err := findTransaction(hash)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/bundle/:hash", func(c echo.Context) error {
		hash := strings.ToUpper(c.Param("hash"))
		bndls, err := findBundles(hash)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, bndls)
	})

	routeGroup.GET("/tag/:tag", func(c echo.Context) error {
		tag := strings.ToUpper(c.Param("tag"))
		txs, err := findTag(strings.ToUpper(tag))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, txs)
	})

	routeGroup.GET("/addr/:hash/value", func(c echo.Context) error {
		hash := strings.ToUpper(c.Param("hash"))
		addr, err := findAddress(hash, true)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, addr)
	})

	routeGroup.GET("/addr/:hash", func(c echo.Context) error {
		hash := strings.ToUpper(c.Param("hash"))
		addr, err := findAddress(hash, false)
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
		msTailTx, err := findMilestone(milestone.Index(index))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, msTailTx)
	})

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := strings.TrimSpace(strings.ToUpper(c.Param("search")))
		result := &SearchResult{}

		// milestone query
		index, err := strconv.Atoi(search)
		if err == nil {
			msTailTx, err := findMilestone(milestone.Index(index))
			if err == nil {
				result.Milestone = msTailTx
			}
			return c.JSON(http.StatusOK, result)
		}

		// check for valid trytes
		if err := trinary.ValidTrytes(search); err != nil {
			return c.JSON(http.StatusOK, result)
		}

		// tag query
		if len(search) == 27 {
			txs, err := findTag(search)
			if err == nil && len(txs.Txs) > 0 {
				result.Tag = txs
				return c.JSON(http.StatusOK, result)
			}
		}

		if len(search) < 81 {
			return c.JSON(http.StatusOK, result)
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
			addr, err := findAddress(search, false)
			if err == nil && (len(addr.Txs) > 0 || addr.Balance > 0) {
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

func findMilestone(index milestone.Index) (*ExplorerTx, error) {
	cachedMs := tangle.GetMilestoneOrNil(index) // message +1
	if cachedMs == nil {
		return nil, errors.Wrapf(ErrNotFound, "milestone %d unknown", index)
	}
	defer cachedMs.Release(true) // message -1

	cachedTailTx := cachedMs.GetMessage().GetTail() // msg +1
	defer cachedTailTx.Release(true)                // msg -1
	return createExplorerTx(cachedTailTx.Retain())  // tx pass +1
}

func findTransaction(hash trinary.Hash) (*ExplorerTx, error) {
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	cachedMsg := tangle.GetCachedMessageOrNil(hornet.HashFromHashTrytes(hash)) // msg +1
	if cachedMsg == nil {
		return nil, errors.Wrapf(ErrNotFound, "tx %s unknown", hash)
	}

	t, err := createExplorerTx(cachedMsg.Retain()) // tx pass +1
	cachedMsg.Release(true)                        // msg -1
	return t, err
}

func findTag(tag trinary.Trytes) (*ExplorerTag, error) {
	if err := trinary.ValidTrytes(tag); err != nil {
		return nil, errors.Wrapf(ErrInvalidParameter, "tag invalid: %s", tag)
	}

	if len(tag) != 27 {
		return nil, errors.Wrapf(ErrInvalidParameter, "tag invalid length: %s", tag)
	}

	txHashes := tangle.GetTagHashes(hornet.HashFromTagTrytes(tag), true, MaxTagResults)
	if len(txHashes) == 0 {
		return nil, errors.Wrapf(ErrNotFound, "tag %s unknown", tag)
	}

	txs := make([]*ExplorerTx, 0, len(txHashes))
	if len(txHashes) != 0 {
		for i := 0; i < len(txHashes); i++ {
			txHash := txHashes[i]
			cachedMsg := tangle.GetCachedMessageOrNil(txHash) // msg +1
			if cachedMsg == nil {
				return nil, errors.Wrapf(ErrNotFound, "tx %s not found but associated to tag %s", txHash.Trytes(), tag)
			}
			expTx, err := createExplorerTx(cachedMsg.Retain()) // tx pass +1
			cachedMsg.Release(true)                            // msg -1
			if err != nil {
				return nil, err
			}
			txs = append(txs, expTx)
		}
	}

	return &ExplorerTag{Txs: txs}, nil
}

func findBundles(hash trinary.Hash) ([][]*ExplorerTx, error) {
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	cachedMessages := tangle.GetBundles(hornet.HashFromHashTrytes(hash), true, MaxBundleResults) // message +1
	if len(cachedMessages) == 0 {
		return nil, errors.Wrapf(ErrNotFound, "bundle %s unknown", hash)
	}
	defer cachedMessages.Release(true) // message -1

	expBndls := [][]*ExplorerTx{}
	for _, cachedMessage := range cachedMessages {
		sl := []*ExplorerTx{}
		cachedMsgs := cachedMessage.GetMessage().GetTransactions() // msg +1
		for _, cachedMsg := range cachedMsgs {
			expTx, err := createExplorerTx(cachedMsg.Retain()) // tx pass +1
			if err != nil {
				cachedMsgs.Release(true) // msg -1
				return nil, err
			}
			sl = append(sl, expTx)
		}
		cachedMsgs.Release(true) // msg -1
		expBndls = append(expBndls, sl)
	}

	return expBndls, nil
}

func findAddress(hash trinary.Hash, valueOnly bool) (*ExplorerAddress, error) {
	if len(hash) > 81 {
		hash = hash[:81]
	}
	if !guards.IsTrytesOfExactLength(hash, consts.HashTrytesSize) {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", hash)
	}

	addr := hornet.HashFromAddressTrytes(hash)

	txHashes := tangle.GetTransactionHashesForAddress(addr, valueOnly, true, MaxTransactionsForAddressResults)

	txs := make([]*ExplorerTx, 0, len(txHashes))
	if len(txHashes) != 0 {
		for i := 0; i < len(txHashes); i++ {
			txHash := txHashes[i]
			cachedMsg := tangle.GetCachedMessageOrNil(txHash) // msg +1
			if cachedMsg == nil {
				return nil, errors.Wrapf(ErrNotFound, "tx %s not found but associated to address %s", txHash, hash)
			}
			expTx, err := createExplorerTx(cachedMsg.Retain()) // tx pass +1
			cachedMsg.Release(true)                            // msg -1
			if err != nil {
				return nil, err
			}
			txs = append(txs, expTx)
		}
	}

	balance, _, err := tangle.GetBalanceForAddress(addr)
	if err != nil {
		return nil, err
	}

	return &ExplorerAddress{
		Balance: balance,
		Txs:     txs,
	}, nil
}
