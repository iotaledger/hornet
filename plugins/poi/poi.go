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
	var auditPathHead hornet.MessageID

	addMessage := func(messageID hornet.MessageID) error {
		cachedMsg := deps.Storage.CachedMessageOrNil(messageID) // message +1
		if cachedMsg == nil {
			return fmt.Errorf("%w message ID: %s", common.ErrMessageNotFound, messageID.ToHex())
		}
		defer cachedMsg.Release(true) // message -1

		auditPathHead = messageID

		if bytes.Equal(milestoneMessageID, messageID) || bytes.Equal(targetMessageID, messageID) {
			// do not add the milestone or the target message itself
			return nil
		}

		auditPathMessages = append(auditPathMessages, cachedMsg.Message().Message())

		return nil
	}

	targetFound := false
	if err := dag.TraverseParentsOfMessage(
		ctx,
		deps.Storage,
		milestoneMessageID,
		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex()
			if !referenced {
				// this should never happen
				return false, errors.New("indirect parents of milestone not referenced by a milestone")
			}

			// if the message is referenced by an older milestone, there is no need to traverse its parents
			if at < msIndex {
				return false, nil
			}

			return true, nil
		},
		// consumer
		func(cachedMsgMeta *storage.CachedMetadata) error {
			defer cachedMsgMeta.Release(true) // meta -1

			if !targetFound && bytes.Equal(cachedMsgMeta.Metadata().MessageID(), targetMessageID) {
				targetFound = true
				return addMessage(cachedMsgMeta.Metadata().MessageID())
			}

			if targetFound {
				// we need to check if the parents of this message are part of the path
				for _, parent := range cachedMsgMeta.Metadata().Parents() {
					if bytes.Equal(auditPathHead, parent) {
						// parent found
						return addMessage(cachedMsgMeta.Metadata().MessageID())
					}
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

	if !bytes.Equal(auditPathHead, milestoneMessageID) {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "path to milestone %d not found", msIndex)
	}

	return auditPathMessages, nil
}

func checkAuditPath(ctx context.Context, auditPathMessages []*iotago.Message, milestoneMessage *iotago.Message, targetMessage *iotago.Message) (bool, error) {

	storedMilestoneMsg, err := storage.NewMessage(milestoneMessage, serializer.DeSeriModePerformValidation)
	if err != nil {
		return false, errors.WithMessage(restapi.ErrInvalidParameter, "invalid milestone message")
	}

	// Verify the contained Milestone signatures
	msPayload := deps.MilestoneManager.VerifyMilestone(storedMilestoneMsg)
	if msPayload == nil {
		return false, errors.WithMessage(restapi.ErrInvalidParameter, "invalid milestone payload")
	}

	// Hash the contained message to get the ID
	targetMsgID, err := targetMessage.ID()
	if err != nil {
		return false, err
	}

	// walk the audit path starting with the target message
	auditPathHead := *targetMsgID

	for _, auditPathMessage := range auditPathMessages {
		currentMsgFound := false

		for _, parentMsgID := range auditPathMessage.Parents {
			if parentMsgID == auditPathHead {
				currentMsgFound = true
				msgID, err := auditPathMessage.ID()
				if err != nil {
					return false, err
				}

				auditPathHead = *msgID
				break
			}
		}

		if !currentMsgFound {
			// audit path invalid
			return false, nil
		}
	}

	// check the milestone parents
	for _, parentMsgID := range milestoneMessage.Parents {
		if parentMsgID == auditPathHead {
			// milestone points to the audit path head
			// => proof valid
			return true, nil
		}
	}

	return false, nil
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
