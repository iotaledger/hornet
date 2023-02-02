package poi

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/serializer"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	ProofVersion = 1
)

func createAuditPath(ctx context.Context, msIndex milestone.Index, milestoneMessageID hornet.MessageID, targetMessageID hornet.MessageID) ([]*iotago.Message, error) {
	var auditPathMessages []*iotago.Message
	auditPathMessageIDs := make(map[iotago.MessageID]struct{})

	milestonePathFound := false
	addMessage := func(messageID hornet.MessageID) error {
		cachedMsg := deps.Storage.CachedMessageOrNil(messageID) // message +1
		if cachedMsg == nil {
			return fmt.Errorf("%w message ID: %s", common.ErrMessageNotFound, messageID.ToHex())
		}
		defer cachedMsg.Release(true) // message -1

		if bytes.Equal(milestoneMessageID, messageID) {
			// do not add the milestone itself
			milestonePathFound = true
			return nil
		}

		auditPathMessages = append(auditPathMessages, cachedMsg.Message().Message())
		auditPathMessageIDs[messageID.ToArray()] = struct{}{}

		return nil
	}

	originFound := false
	if err := dag.TraverseParentsOfMessage(
		ctx,
		deps.Storage,
		milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// if the message is referenced by an older milestone, there is no need to traverse its parents
			if referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex(); !referenced || (at < msIndex) {
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error {
			defer cachedMsgMeta.Release(true) // meta -1

			if !originFound && bytes.Equal(cachedMsgMeta.Metadata().MessageID(), targetMessageID) {
				originFound = true
				return addMessage(cachedMsgMeta.Metadata().MessageID())
			}

			if originFound {
				// we need to check if the parents of this message are part of the path
				parentFound := false
				for _, parent := range cachedMsgMeta.Metadata().Parents() {
					if _, exists := auditPathMessageIDs[parent.ToArray()]; !exists {
						continue
					}

					parentFound = true

					// delete the found parent from the map, so we don't include several paths in the proof
					// the path may not be the shortest, but its always the same. (will be replaced by stardust PoI anyway)
					delete(auditPathMessageIDs, parent.ToArray())
				}

				if parentFound {
					return addMessage(cachedMsgMeta.Metadata().MessageID())
				}
			}

			return nil
		},
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		if errors.Is(err, common.ErrOperationAborted) {
			return nil, errors.WithMessage(echo.ErrInternalServerError, "operation aborted")
		} else {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
	}

	if !milestonePathFound {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "path to milestone %d not found", msIndex)
	}

	return auditPathMessages, nil
}

func checkAuditPath(ctx context.Context, auditPathMessages []*iotago.Message, milestoneMsg *iotago.Message, message *iotago.Message) (bool, error) {

	storedMilestoneMsg, err := storage.NewMessage(milestoneMsg, serializer.DeSeriModePerformValidation)
	if err != nil {
		return false, errors.WithMessage(restapi.ErrInvalidParameter, "invalid milestone message")
	}

	// Verify the contained Milestone signatures
	msPayload := deps.MilestoneManager.VerifyMilestone(storedMilestoneMsg)
	if msPayload == nil {
		return false, errors.WithMessage(restapi.ErrInvalidParameter, "invalid milestone payload")
	}

	// Hash the contained message to get the ID
	messageID, err := message.ID()
	if err != nil {
		return false, err
	}

	targetMsgID := *messageID

	// create a map for faster lookup of transactions
	auditPathMessageIDs := make(map[iotago.MessageID]*iotago.Message)
	for _, msg := range auditPathMessages {
		msgID, err := msg.ID()
		if err != nil {
			return false, err
		}

		m := msg
		auditPathMessageIDs[*msgID] = m
	}

	// walk the audit path starting with the milestone
	currentMsg := milestoneMsg
	for {
		parentFound := false
		for _, parentMsgID := range currentMsg.Parents {
			parentMsg, exists := auditPathMessageIDs[parentMsgID]
			if !exists {
				continue
			}

			if parentMsgID == targetMsgID {
				// we found the valid path to the message
				return true, nil
			}

			// delete the found parent from the map, so we don't walk the path again
			parentFound = true
			currentMsg = parentMsg
			delete(auditPathMessageIDs, parentMsgID)
			break
		}

		if !parentFound {
			// audit path invalid
			return false, nil
		}
	}
}

func createProof(c echo.Context) (*ProofRequestAndResponse, error) {

	messageID, err := restapi.ParseMessageIDParam(c)
	if err != nil {
		return nil, err
	}

	cachedMsg := deps.Storage.CachedMessageOrNil(messageID) // message +1
	if cachedMsg == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true) // message -1

	referenced, msIndex := cachedMsg.Metadata().ReferencedWithIndex()
	if !referenced {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "message %s is not referenced by a milestone", messageID.ToHex())
	}

	cachedMsgMilestone := deps.Storage.MilestoneCachedMessageOrNil(msIndex) // message +1
	if cachedMsgMilestone == nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "milestone %d not found", msIndex)
	}
	defer cachedMsgMilestone.Release(true) // milestone -1

	ms := cachedMsgMilestone.Message().Milestone()
	if ms == nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "milestone %d not found", msIndex)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	auditPathMessages, err := createAuditPath(ctx, msIndex, cachedMsgMilestone.Message().MessageID(), messageID)
	if err != nil {
		return nil, err
	}

	return &ProofRequestAndResponse{
		Version:   ProofVersion,
		Milestone: cachedMsgMilestone.Message().Message(),
		Message:   cachedMsg.Message().Message(),
		Proof:     auditPathMessages,
	}, nil
}

func validateProof(c echo.Context) (*ValidateProofResponse, error) {

	req := &ProofRequestAndResponse{}
	if err := c.Bind(req); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if req.Version != ProofVersion {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: wrong version of proof: %d, supported version: %d", req.Version, ProofVersion)
	}

	if req.Proof == nil || req.Milestone == nil || req.Message == nil {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "invalid request")
	}

	valid, err := checkAuditPath(c.Request().Context(), req.Proof, req.Milestone, req.Message)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "failed to check audit path: %s", err)

	}

	return &ValidateProofResponse{Valid: valid}, nil
}
