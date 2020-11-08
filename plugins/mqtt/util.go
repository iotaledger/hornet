package mqtt

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/urts"
)

func publishOnTopic(topic string, payload interface{}) {
	milestoneInfoJSON, err := json.Marshal(payload)
	if err != nil {
		log.Warn(err.Error())
		return
	}

	mqttBroker.Send(topic, milestoneInfoJSON)
}

func publishMilestoneOnTopic(topic string, milestone *tangle.Milestone) {
	publishOnTopic(topic, &milestonePayload{
		Index:       uint32(milestone.Index),
		MilestoneID: hex.EncodeToString(milestone.MilestoneID[:]),
		Time:        milestone.Timestamp.Unix(),
	})
}

func publishMessageMetadata(cachedMetadata *tangle.CachedMetadata) {

	defer cachedMetadata.Release(true)

	metadata := cachedMetadata.GetMetadata()

	var referencedByMilestone *milestone.Index = nil
	referenced, referencedIndex := metadata.GetReferenced()
	if referenced {
		referencedByMilestone = &referencedIndex
	}

	messageMetadataResponse := &messageMetadataPayload{
		MessageID:                  metadata.GetMessageID().Hex(),
		Parent1:                    metadata.GetParent1MessageID().Hex(),
		Parent2:                    metadata.GetParent2MessageID().Hex(),
		Solid:                      metadata.IsSolid(),
		ReferencedByMilestoneIndex: referencedByMilestone,
	}

	if referenced {
		inclusionState := "noTransaction"

		if metadata.IsConflictingTx() {
			inclusionState = "conflicting"
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = "included"
		}

		messageMetadataResponse.LedgerInclusionState = &inclusionState
	} else if metadata.IsSolid() {
		// determine info about the quality of the tip if not referenced
		lsmi := deps.Tangle.GetSolidMilestoneIndex()
		ycri, ocri := dag.GetConeRootIndexes(deps.Tangle, cachedMetadata.Retain(), lsmi)

		// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
		shouldPromote := false
		shouldReattach := false

		if (lsmi - ocri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelBelowMaxDepth)) {
			// if the OCRI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
			shouldPromote = false
			shouldReattach = true
		} else if (lsmi - ycri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI)) {
			// if the LSMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI, then the tip is lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		} else if (lsmi - ocri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI)) {
			// if the OCRI to LSMI delta is over CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI, the tip is semi-lazy and should be promoted
			shouldPromote = true
			shouldReattach = false
		}

		messageMetadataResponse.ShouldPromote = &shouldPromote
		messageMetadataResponse.ShouldReattach = &shouldReattach
	}

	messageId := metadata.GetMessageID().Hex()
	topic := strings.ReplaceAll(topicMessageMetadata, "{messageId}", messageId)

	publishOnTopic(topic, messageMetadataResponse)
}
