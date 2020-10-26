package dashboard

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

const (
	MaxMessagesForAddressResults = 100
	MaxChildrenResults           = 100
	MaxIndexationResults         = 100
)

type ExplorerMessage struct {
	MessageID        string `json:"message_id"`
	Parent1MessageID string `json:"parent1_message_id"`
	Parent2MessageID string `json:"parent2_message_id"`
	Referenced       struct {
		State       bool            `json:"state"`
		Conflicting bool            `json:"conflicting"`
		Milestone   milestone.Index `json:"milestone_index"`
	} `json:"referenced"`
	Children       []string        `json:"children"`
	Solid          bool            `json:"solid"`
	MWM            int             `json:"mwm"`
	IsMilestone    bool            `json:"is_milestone"`
	MilestoneIndex milestone.Index `json:"milestone_index"`
}

func createExplorerMessage(cachedMsg *tangle.CachedMessage) (*ExplorerMessage, error) {
	defer cachedMsg.Release(true) // msg -1

	referenced, by := cachedMsg.GetMetadata().GetReferenced()
	conflicting := cachedMsg.GetMetadata().IsConflictingTx()
	t := &ExplorerMessage{
		MessageID:        cachedMsg.GetMetadata().GetMessageID().Hex(),
		Parent1MessageID: cachedMsg.GetMetadata().GetParent1MessageID().Hex(),
		Parent2MessageID: cachedMsg.GetMetadata().GetParent2MessageID().Hex(),
		Referenced: struct {
			State       bool            `json:"state"`
			Conflicting bool            `json:"conflicting"`
			Milestone   milestone.Index `json:"milestone_index"`
		}{referenced, conflicting, by},
		Solid: cachedMsg.GetMetadata().IsSolid(),
	}

	// Children
	t.Children = tangle.GetChildrenMessageIDs(cachedMsg.GetMessage().GetMessageID(), MaxChildrenResults).Hex()

	// compute mwm
	// TODO:
	/*
		trits, err := trinary.BytesToTrits(cachedMsg.GetMessage().GetMessageID().Slice())
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
	*/
	t.MWM = 14

	// check whether milestone
	ms := cachedMsg.GetMessage().GetMilestone()
	if ms != nil {
		t.IsMilestone = true
		t.MilestoneIndex = milestone.Index(ms.Index)
	}

	return t, nil
}

type ExplorerTag struct {
	Messages []*ExplorerMessage `json:"msgs"`
}

type ExplorerAddress struct {
	Balance  uint64             `json:"balance"`
	Messages []*ExplorerMessage `json:"msgs"`
}

type SearchResult struct {
	Message   *ExplorerMessage     `json:"msg"`
	Tag       *ExplorerTag         `json:"tag"`
	Address   *ExplorerAddress     `json:"address"`
	Messages  [][]*ExplorerMessage `json:"messages"`
	Milestone *ExplorerMessage     `json:"milestone"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {

	routeGroup.GET("/msg/:hash", func(c echo.Context) error {
		hash := strings.ToUpper(c.Param("hash"))
		t, err := findTransaction(hash)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/tag/:index", func(c echo.Context) error {
		index := strings.ToUpper(c.Param("index"))
		msgs, err := findIndex(strings.ToUpper(index))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, msgs)
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
		milestoneMessage, err := findMilestone(milestone.Index(index))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, milestoneMessage)
	})

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := strings.TrimSpace(strings.ToUpper(c.Param("search")))
		result := &SearchResult{}

		// milestone query
		index, err := strconv.Atoi(search)
		if err == nil {
			milestoneMessage, err := findMilestone(milestone.Index(index))
			if err == nil {
				result.Milestone = milestoneMessage
			}
			return c.JSON(http.StatusOK, result)
		}

		// check for valid trytes
		/*
			if err := trinary.ValidTrytes(search); err != nil {
				return c.JSON(http.StatusOK, result)
			}
		*/

		// tag query
		if len(search) == 27 {
			msgs, err := findIndex(search)
			if err == nil && len(msgs.Messages) > 0 {
				result.Tag = msgs
				return c.JSON(http.StatusOK, result)
			}
		}

		if len(search) < 81 {
			return c.JSON(http.StatusOK, result)
		}

		// auto. remove checksum
		search = search[:81]

		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			msg, err := findTransaction(search)
			if err == nil {
				result.Message = msg
			}
		}()

		go func() {
			defer wg.Done()
			addr, err := findAddress(search, false)
			if err == nil && (len(addr.Messages) > 0 || addr.Balance > 0) {
				result.Address = addr
			}
		}()

		wg.Wait()

		return c.JSON(http.StatusOK, result)
	})
}

func findMilestone(index milestone.Index) (*ExplorerMessage, error) {
	cachedMsg := tangle.GetMilestoneCachedMessageOrNil(index) // message +1
	if cachedMsg == nil {
		return nil, errors.Wrapf(ErrNotFound, "milestone %d unknown", index)
	}
	defer cachedMsg.Release(true) // message -1

	return createExplorerMessage(cachedMsg.Retain()) // msg pass +1
}

func findTransaction(msgID string) (*ExplorerMessage, error) {
	if len(msgID) != 64 {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", msgID)
	}

	messageID, err := hornet.MessageIDFromHex(msgID)
	if err != nil {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", err.Error())
	}

	cachedMsg := tangle.GetCachedMessageOrNil(messageID) // msg +1
	if cachedMsg == nil {
		return nil, errors.Wrapf(ErrNotFound, "msg %s unknown", msgID)
	}

	t, err := createExplorerMessage(cachedMsg.Retain()) // msg pass +1
	cachedMsg.Release(true)                             // msg -1
	return t, err
}

func findIndex(tag string) (*ExplorerTag, error) {
	return nil, errors.New("not implemented")
	/*
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

		msgs := make([]*ExplorerMessage, 0, len(txHashes))
		if len(txHashes) != 0 {
			for i := 0; i < len(txHashes); i++ {
				txHash := txHashes[i]
				cachedMsg := tangle.GetCachedMessageOrNil(txHash) // msg +1
				if cachedMsg == nil {
					return nil, errors.Wrapf(ErrNotFound, "msg %s not found but associated to tag %s", txHash.Trytes(), tag)
				}
				expTx, err := createExplorerMessage(cachedMsg.Retain()) // msg pass +1
				cachedMsg.Release(true)                                 // msg -1
				if err != nil {
					return nil, err
				}
				msgs = append(msgs, expTx)
			}
		}

		return &ExplorerTag{Messages: msgs}, nil
	*/
}

func findAddress(messageID string, valueOnly bool) (*ExplorerAddress, error) {
	if len(messageID) != 64 {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", messageID)
	}

	_, err := hex.DecodeString(messageID)
	if err != nil {
		return nil, errors.Wrapf(ErrInvalidParameter, "hash invalid: %s", err.Error())
	}

	/*
		ToDo:
		txHashes := tangle.GetTransactionHashesForAddress(addr, valueOnly, true, MaxTransactionsForAddressResults)

		msgs := make([]*ExplorerMessage, 0, len(txHashes))
		if len(txHashes) != 0 {
			for i := 0; i < len(txHashes); i++ {
				txHash := txHashes[i]
				cachedMsg := tangle.GetCachedMessageOrNil(txHash) // msg +1
				if cachedMsg == nil {
					return nil, errors.Wrapf(ErrNotFound, "msg %s not found but associated to address %s", txHash, messageID)
				}
				expTx, err := createExplorerTx(cachedMsg.Retain()) // msg pass +1
				cachedMsg.Release(true)                            // msg -1
				if err != nil {
					return nil, err
				}
				msgs = append(msgs, expTx)
			}
		}
	*/

	/*
		// Todo
		balance, _, err := tangle.GetBalanceForAddress(addr)
		if err != nil {
			return nil, err
		}
	*/
	msgs := make([]*ExplorerMessage, 0)
	var balance uint64 = 0

	return &ExplorerAddress{
		Balance:  balance,
		Messages: msgs,
	}, nil
}
