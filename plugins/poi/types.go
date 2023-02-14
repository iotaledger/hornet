package poi

import (
	iotago "github.com/iotaledger/iota.go/v2"
)

type ProofRequestAndResponse struct {
	Version   byte              `json:"version"`
	Milestone *iotago.Message   `json:"milestone"`
	Message   *iotago.Message   `json:"message"`
	Proof     []*iotago.Message `json:"proof"`
}

type ValidateProofResponse struct {
	Valid bool `json:"valid"`
}
