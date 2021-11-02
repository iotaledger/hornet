package participation

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
)

// ParticipationEventIDFromHex creates a ParticipationEventID from a hex string representation.
func ParticipationEventIDFromHex(hexString string) (participation.ParticipationEventID, error) {

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return participation.NullParticipationEventID, err
	}

	if len(b) != participation.ParticipationEventIDLength {
		return participation.ParticipationEventID{}, fmt.Errorf("unknown referendumID length (%d)", len(b))
	}

	var referendumID participation.ParticipationEventID
	copy(referendumID[:], b)
	return referendumID, nil
}

func parseParticipationEventIDParam(c echo.Context) (participation.ParticipationEventID, error) {

	referendumIDHex := strings.ToLower(c.Param(ParameterParticipationEventID))
	if referendumIDHex == "" {
		return participation.NullParticipationEventID, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterParticipationEventID)
	}

	referendumID, err := ParticipationEventIDFromHex(referendumIDHex)
	if err != nil {
		return participation.NullParticipationEventID, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid participation ID: %s, error: %s", referendumIDHex, err)
	}

	return referendumID, nil
}

func getEvents(_ echo.Context) (*ParticipationEventsResponse, error) {
	eventIDs := deps.ParticipationManager.ParticipationEventIDs()

	hexEventIDs := []string{}
	for _, id := range eventIDs {
		hexEventIDs = append(hexEventIDs, hex.EncodeToString(id[:]))
	}

	return &ParticipationEventsResponse{ParticipationEventIDs: hexEventIDs}, nil
}

func createEvent(c echo.Context) (*CreateParticipationEventResponse, error) {

	//TODO: add support for binary representation too?

	event := &participation.ParticipationEvent{}
	if err := c.Bind(event); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request! Error: %s", err)
	}

	eventID, err := deps.ParticipationManager.StoreParticipationEvent(event)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid participation event, error: %s", err)
	}

	return &CreateParticipationEventResponse{
		ParticipationEventID: hex.EncodeToString(eventID[:]),
	}, nil
}

func getEvent(c echo.Context) (*participation.ParticipationEvent, error) {

	eventID, err := parseParticipationEventIDParam(c)
	if err != nil {
		return nil, err
	}

	event := deps.ParticipationManager.ParticipationEvent(eventID)
	if event == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "participation event not found: %s", hex.EncodeToString(eventID[:]))
	}

	return event, nil
}

func deleteEvent(c echo.Context) error {

	eventID, err := parseParticipationEventIDParam(c)
	if err != nil {
		return nil
	}

	return deps.ParticipationManager.DeleteParticipationEvent(eventID)
}

func getEventStatus(c echo.Context) (*participation.ParticipationEventStatus, error) {

	eventID, err := parseParticipationEventIDParam(c)
	if err != nil {
		return nil, err
	}

	status, err := deps.ParticipationManager.ParticipationEventStatus(eventID)
	if err != nil {
		if errors.Is(err, participation.ErrParticipationEventNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "participation event not found: %s", hex.EncodeToString(eventID[:]))
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

	trackedParticipations, err := deps.ParticipationManager.VotesForOutputID(&outputID)
	if err != nil {
		return nil, err
	}

	if len(trackedParticipations) == 0 {
		return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", hex.EncodeToString(outputIDBytes))
	}

	response := &OutputStatusResponse{
		Participations: make(map[string]*TrackedParticipation),
	}

	for _, trackedParticipation := range trackedParticipations {
		t := &TrackedParticipation{
			MessageID:           trackedParticipation.MessageID.ToHex(),
			Amount:              trackedParticipation.Amount,
			StartMilestoneIndex: trackedParticipation.StartIndex,
			EndMilestoneIndex:   trackedParticipation.EndIndex,
		}
		response.Participations[hex.EncodeToString(trackedParticipation.ParticipationEventID[:])] = t
	}

	return response, nil

}
