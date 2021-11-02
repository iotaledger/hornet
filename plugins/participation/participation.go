package participation

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/partitipation"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
)

// ParticipationEventIDFromHex creates a ParticipationEventID from a hex string representation.
func ParticipationEventIDFromHex(hexString string) (partitipation.ParticipationEventID, error) {

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return partitipation.NullParticipationEventID, err
	}

	if len(b) != partitipation.ParticipationEventIDLength {
		return partitipation.ParticipationEventID{}, fmt.Errorf("unknown referendumID length (%d)", len(b))
	}

	var referendumID partitipation.ParticipationEventID
	copy(referendumID[:], b)
	return referendumID, nil
}

func parseReferendumIDParam(c echo.Context) (partitipation.ParticipationEventID, error) {

	referendumIDHex := strings.ToLower(c.Param(ParameterParticipationEventID))
	if referendumIDHex == "" {
		return partitipation.NullParticipationEventID, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterParticipationEventID)
	}

	referendumID, err := ParticipationEventIDFromHex(referendumIDHex)
	if err != nil {
		return partitipation.NullParticipationEventID, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid partitipation ID: %s, error: %s", referendumIDHex, err)
	}

	return referendumID, nil
}

func getReferendums(_ echo.Context) (*ParticipationEventsResponse, error) {
	referendumIDs := deps.ParticipationManager.ParticipationEventIDs()

	hexReferendumIDs := []string{}
	for _, id := range referendumIDs {
		hexReferendumIDs = append(hexReferendumIDs, hex.EncodeToString(id[:]))
	}

	return &ParticipationEventsResponse{ParticipationEventIDs: hexReferendumIDs}, nil
}

func createReferendum(c echo.Context) (*CreateReferendumResponse, error) {

	//TODO: add support for binary representation too?

	referendum := &partitipation.ParticipationEvent{}
	if err := c.Bind(referendum); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request! Error: %s", err)
	}

	referendumID, err := deps.ParticipationManager.StoreReferendum(referendum)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid partitipation, error: %s", err)
	}

	return &CreateReferendumResponse{
		ParticipationEventID: hex.EncodeToString(referendumID[:]),
	}, nil
}

func getReferendum(c echo.Context) (*partitipation.ParticipationEvent, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	referendum := deps.ParticipationManager.Referendum(referendumID)
	if referendum == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "partitipation not found: %s", hex.EncodeToString(referendumID[:]))
	}

	return referendum, nil
}

func deleteReferendum(c echo.Context) error {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil
	}

	return deps.ParticipationManager.DeleteReferendum(referendumID)
}

func getReferendumStatus(c echo.Context) (*partitipation.ReferendumStatus, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	status, err := deps.ParticipationManager.ReferendumStatus(referendumID)
	if err != nil {
		if errors.Is(err, partitipation.ErrReferendumNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "partitipation not found: %s", hex.EncodeToString(referendumID[:]))
		}
		return nil, err
	}
	return status, nil
}

func getOutputStatus(c echo.Context) (*OutputStatusResponse, error) {

	outputIDParam := strings.ToLower(c.Param(ParameterOutputID))

	outputIDBytes, err := hex.DecodeString(outputIDParam)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	if len(outputIDBytes) != utxo.OutputIDLength {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid output ID: %s, error: %s", outputIDParam, err)
	}

	var outputID iotago.UTXOInputID
	copy(outputID[:], outputIDBytes)

	trackedVotes, err := deps.ParticipationManager.VotesForOutputID(&outputID)
	if err != nil {
		return nil, err
	}

	if len(trackedVotes) == 0 {
		return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", hex.EncodeToString(outputIDBytes))
	}

	response := &OutputStatusResponse{
		ReferendumVotes: make(map[string]*TrackedVote),
	}

	for _, trackedVote := range trackedVotes {
		t := &TrackedVote{
			MessageID:           trackedVote.MessageID.ToHex(),
			Amount:              trackedVote.Amount,
			StartMilestoneIndex: trackedVote.StartIndex,
			EndMilestoneIndex:   trackedVote.EndIndex,
		}
		response.ReferendumVotes[hex.EncodeToString(trackedVote.ParticipationEventID[:])] = t
	}

	return response, nil

}
