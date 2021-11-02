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

// EventIDFromHex creates a EventID from a hex string representation.
func EventIDFromHex(hexString string) (participation.EventID, error) {

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return participation.NullEventID, err
	}

	if len(b) != participation.EventIDLength {
		return participation.EventID{}, fmt.Errorf("unknown eventID length (%d)", len(b))
	}

	var eventID participation.EventID
	copy(eventID[:], b)
	return eventID, nil
}

func parseEventIDParam(c echo.Context) (participation.EventID, error) {

	eventIDHex := strings.ToLower(c.Param(ParameterParticipationEventID))
	if eventIDHex == "" {
		return participation.NullEventID, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterParticipationEventID)
	}

	eventID, err := EventIDFromHex(eventIDHex)
	if err != nil {
		return participation.NullEventID, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid event ID: %s, error: %s", eventIDHex, err)
	}

	return eventID, nil
}

func getEvents(_ echo.Context) (*EventsResponse, error) {
	eventIDs := deps.ParticipationManager.EventIDs()

	hexEventIDs := []string{}
	for _, id := range eventIDs {
		hexEventIDs = append(hexEventIDs, hex.EncodeToString(id[:]))
	}

	return &EventsResponse{EventIDs: hexEventIDs}, nil
}

func createEvent(c echo.Context) (*CreateEventResponse, error) {

	//TODO: add support for binary representation too?

	event := &participation.Event{}
	if err := c.Bind(event); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request! Error: %s", err)
	}

	eventID, err := deps.ParticipationManager.StoreEvent(event)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid event, error: %s", err)
	}

	return &CreateEventResponse{
		eventID: hex.EncodeToString(eventID[:]),
	}, nil
}

func getEvent(c echo.Context) (*participation.Event, error) {

	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	event := deps.ParticipationManager.Event(eventID)
	if event == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "event not found: %s", hex.EncodeToString(eventID[:]))
	}

	return event, nil
}

func deleteEvent(c echo.Context) error {

	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil
	}

	return deps.ParticipationManager.DeleteEvent(eventID)
}

func getEventStatus(c echo.Context) (*participation.ParticipationEventStatus, error) {

	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	status, err := deps.ParticipationManager.ParticipationEventStatus(eventID)
	if err != nil {
		if errors.Is(err, participation.ErrEventNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "event not found: %s", hex.EncodeToString(eventID[:]))
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

	trackedParticipations, err := deps.ParticipationManager.ParticipationsForOutputID(&outputID)
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
		response.Participations[hex.EncodeToString(trackedParticipation.EventID[:])] = t
	}

	return response, nil

}
