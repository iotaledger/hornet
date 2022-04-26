package participation

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// EventIDFromHex creates a EventID from a hex string representation.
func EventIDFromHex(hexString string) (participation.EventID, error) {

	b, err := iotago.DecodeHex(hexString)
	if err != nil {
		return participation.NullEventID, err
	}

	if len(b) != participation.EventIDLength {
		return participation.NullEventID, fmt.Errorf("unknown eventID length (%d)", len(b))
	}

	var eventID participation.EventID
	copy(eventID[:], b)
	return eventID, nil
}

func parseEventTypeQueryParam(c echo.Context) ([]uint32, error) {
	typeParams := c.QueryParams()["type"]

	if len(typeParams) == 0 {
		return []uint32{}, nil
	}

	var returnTypes []uint32
	for _, typeParam := range typeParams {
		intParam, err := strconv.ParseUint(typeParam, 10, 32)
		if err != nil {
			return []uint32{}, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid event type: %s, error: %s", typeParam, err)
		}
		eventType := uint32(intParam)
		switch eventType {
		case participation.BallotPayloadTypeID:
		case participation.StakingPayloadTypeID:
		default:
			return []uint32{}, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid event type: %s", typeParam)
		}
		returnTypes = append(returnTypes, eventType)
	}
	return returnTypes, nil
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

func getEvents(c echo.Context) (*EventsResponse, error) {

	eventTypes, err := parseEventTypeQueryParam(c)
	if err != nil {
		return nil, err
	}

	eventIDs := deps.ParticipationManager.EventIDs(eventTypes...)

	hexEventIDs := []string{}
	for _, id := range eventIDs {
		hexEventIDs = append(hexEventIDs, id.ToHex())
	}
	sort.Strings(hexEventIDs)

	return &EventsResponse{EventIDs: hexEventIDs}, nil
}

func createEvent(c echo.Context) (*CreateEventResponse, error) {

	event := &participation.Event{}
	if err := c.Bind(event); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if _, err := event.Serialize(serializer.DeSeriModePerformValidation, nil); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid event payload, error: %s", err)
	}

	// We need to lock the ledger here so that we don't add a new event while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	eventID, err := deps.ParticipationManager.StoreEvent(event)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid event, error: %s", err)
	}

	return &CreateEventResponse{
		EventID: eventID.ToHex(),
	}, nil
}

func getEvent(c echo.Context) (*participation.Event, error) {

	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	event := deps.ParticipationManager.Event(eventID)
	if event == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "event not found: %s", eventID.ToHex())
	}

	return event, nil
}

func deleteEvent(c echo.Context) error {

	eventID, err := parseEventIDParam(c)
	if err != nil {
		return err
	}

	// We need to lock the ledger here so that we delete an event while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	if err = deps.ParticipationManager.DeleteEvent(eventID); err != nil {
		if errors.Is(err, participation.ErrEventNotFound) {
			return errors.WithMessagef(echo.ErrNotFound, "event not found: %s", eventID.ToHex())
		}
		return errors.WithMessagef(echo.ErrInternalServerError, "deleting event failed: %s", err)
	}

	return nil
}

func parseMilestoneIndexQueryParam(c echo.Context) (milestone.Index, error) {
	milestoneIndexParam := c.QueryParam(restapi.ParameterMilestoneIndex)
	if len(milestoneIndexParam) == 0 {
		return 0, nil
	}

	intParam, err := strconv.ParseUint(milestoneIndexParam, 10, 32)
	if err != nil {
		return 0, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid milestone index: %s, error: %s", milestoneIndexParam, err)
	}
	return milestone.Index(uint32(intParam)), nil
}

func getEventStatus(c echo.Context) (*participation.EventStatus, error) {
	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	milestoneIndex, err := parseMilestoneIndexQueryParam(c)
	if err != nil {
		return nil, err
	}

	var milestoneIndexFilter []milestone.Index
	if milestoneIndex > 0 {
		milestoneIndexFilter = append(milestoneIndexFilter, milestoneIndex)
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	status, err := deps.ParticipationManager.EventStatus(eventID, milestoneIndexFilter...)
	if err != nil {
		if errors.Is(err, participation.ErrEventNotFound) {
			return nil, errors.WithMessagef(echo.ErrNotFound, "event not found: %s", eventID.ToHex())
		}
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "get event status failed: %s", err)
	}
	return status, nil
}

func getOutputStatus(c echo.Context) (*OutputStatusResponse, error) {
	outputID, err := restapi.ParseOutputIDParam(c)
	if err != nil {
		return nil, err
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	trackedParticipations, err := deps.ParticipationManager.ParticipationsForOutputID(outputID)
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "error fetching participations: %s", err)
	}

	if len(trackedParticipations) == 0 {
		return nil, errors.WithMessagef(echo.ErrNotFound, "output not found: %s", outputID.ToHex())
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
		response.Participations[trackedParticipation.EventID.ToHex()] = t
	}

	return response, nil
}

func getRewardsByBech32Address(c echo.Context) (*AddressRewardsResponse, error) {
	bech32Address, err := restapi.ParseBech32AddressParam(c, deps.ProtocolParameters.Bech32HRP)
	if err != nil {
		return nil, err
	}

	milestoneIndex, err := parseMilestoneIndexQueryParam(c)
	if err != nil {
		return nil, err
	}

	if milestoneIndex == 0 {
		milestoneIndex = deps.SyncManager.ConfirmedMilestoneIndex()
	}

	switch address := bech32Address.(type) {
	case *iotago.Ed25519Address:
		return ed25519Rewards(address, milestoneIndex)
	default:
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: unknown address type", bech32Address.String())
	}
}

func getRewardsByEd25519Address(c echo.Context) (*AddressRewardsResponse, error) {

	milestoneIndex, err := parseMilestoneIndexQueryParam(c)
	if err != nil {
		return nil, err
	}

	if milestoneIndex == 0 {
		milestoneIndex = deps.SyncManager.ConfirmedMilestoneIndex()
	}

	address, err := restapi.ParseEd25519AddressParam(c)
	if err != nil {
		return nil, err
	}
	return ed25519Rewards(address, milestoneIndex)
}

func ed25519Rewards(address *iotago.Ed25519Address, msIndex milestone.Index) (*AddressRewardsResponse, error) {
	eventIDs := deps.ParticipationManager.EventIDs(participation.StakingPayloadTypeID)

	response := &AddressRewardsResponse{
		Rewards:        make(map[string]*AddressReward),
		MilestoneIndex: msIndex,
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	for _, eventID := range eventIDs {

		event := deps.ParticipationManager.Event(eventID)
		if event == nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, "event not found")
		}

		staking := event.Staking()
		if staking == nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, "staking payload not found")
		}

		amount, err := deps.ParticipationManager.StakingRewardForAddress(eventID, address, msIndex)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "error fetching rewards: %s", err)
		}

		response.Rewards[eventID.ToHex()] = &AddressReward{
			Amount:         amount,
			Symbol:         staking.Symbol,
			MinimumReached: amount >= staking.RequiredMinimumRewards,
		}
	}

	return response, nil
}

func getRewards(c echo.Context) (*RewardsResponse, error) {
	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	event := deps.ParticipationManager.Event(eventID)

	if event == nil || event.Staking() == nil {
		return nil, errors.WithMessage(echo.ErrNotFound, "no staking event found")
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	milestoneIndex, err := parseMilestoneIndexQueryParam(c)
	if err != nil {
		return nil, err
	}

	if milestoneIndex == 0 {
		milestoneIndex = deps.SyncManager.ConfirmedMilestoneIndex()
	}

	if milestoneIndex > event.EndMilestoneIndex() {
		milestoneIndex = event.EndMilestoneIndex()
	}

	var addresses []string
	rewardsByAddress := make(map[string]uint64)
	if err := deps.ParticipationManager.ForEachAddressStakingParticipation(eventID, milestoneIndex, func(address iotago.Address, _ *participation.TrackedParticipation, rewards uint64) bool {
		addr := address.String()
		if _, has := rewardsByAddress[addr]; !has {
			addresses = append(addresses, addr)
		}
		rewardsByAddress[addr] += rewards
		return true
	}); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "error fetching rewards: %s", err)
	}

	responseHash := sha256.New()
	responseHash.Write(eventID[:])
	binary.Write(responseHash, binary.LittleEndian, uint32(milestoneIndex))
	responseHash.Write([]byte(event.Staking().Symbol))

	response := &RewardsResponse{
		Symbol:         event.Staking().Symbol,
		MilestoneIndex: milestoneIndex,
		TotalRewards:   0,
		Rewards:        make(map[string]uint64),
	}

	sort.Strings(addresses)
	for _, addr := range addresses {
		amount := rewardsByAddress[addr]
		if amount < event.Staking().RequiredMinimumRewards {
			continue
		}
		responseHash.Write([]byte(addr))
		binary.Write(responseHash, binary.LittleEndian, amount)
		response.Rewards[addr] = amount
		response.TotalRewards += amount
	}

	response.Checksum = iotago.EncodeHex(responseHash.Sum(nil))

	return response, nil
}

func getOutputsByBech32Address(c echo.Context) (*AddressOutputsResponse, error) {
	bech32Address, err := restapi.ParseBech32AddressParam(c, deps.ProtocolParameters.Bech32HRP)
	if err != nil {
		return nil, err
	}

	switch address := bech32Address.(type) {
	case *iotago.Ed25519Address:
		return ed25519Outputs(address)
	default:
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid address: %s, error: unknown address type", bech32Address.String())
	}
}

func getOutputsByEd25519Address(c echo.Context) (*AddressOutputsResponse, error) {
	address, err := restapi.ParseEd25519AddressParam(c)
	if err != nil {
		return nil, err
	}
	return ed25519Outputs(address)
}

func ed25519Outputs(address *iotago.Ed25519Address) (*AddressOutputsResponse, error) {
	eventIDs := deps.ParticipationManager.EventIDs(participation.StakingPayloadTypeID)

	response := &AddressOutputsResponse{
		Outputs: make(map[string]*OutputStatusResponse),
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	for _, eventID := range eventIDs {

		event := deps.ParticipationManager.Event(eventID)
		if event == nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, "event not found")
		}

		participations, err := deps.ParticipationManager.ParticipationsForAddress(eventID, address)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "error fetching outputs: %s", err)
		}
		for _, trackedParticipation := range participations {

			t := &TrackedParticipation{
				MessageID:           trackedParticipation.MessageID.ToHex(),
				Amount:              trackedParticipation.Amount,
				StartMilestoneIndex: trackedParticipation.StartIndex,
				EndMilestoneIndex:   trackedParticipation.EndIndex,
			}
			outputResponse := response.Outputs[trackedParticipation.OutputID.ToHex()]
			if outputResponse == nil {
				outputResponse = &OutputStatusResponse{
					Participations: make(map[string]*TrackedParticipation),
				}
				response.Outputs[trackedParticipation.OutputID.ToHex()] = outputResponse
			}
			outputResponse.Participations[trackedParticipation.EventID.ToHex()] = t
		}
	}

	return response, nil
}

func getActiveParticipations(c echo.Context) (*ParticipationsResponse, error) {
	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	response := &ParticipationsResponse{
		Participations: make(map[string]*TrackedParticipation),
	}
	if err := deps.ParticipationManager.ForEachActiveParticipation(eventID, func(trackedParticipation *participation.TrackedParticipation) bool {
		t := &TrackedParticipation{
			MessageID:           trackedParticipation.MessageID.ToHex(),
			Amount:              trackedParticipation.Amount,
			StartMilestoneIndex: trackedParticipation.StartIndex,
			EndMilestoneIndex:   trackedParticipation.EndIndex,
		}
		response.Participations[trackedParticipation.OutputID.ToHex()] = t
		return true
	}); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "error fetching active participations: %s", err)
	}
	return response, nil
}

func getPastParticipations(c echo.Context) (*ParticipationsResponse, error) {
	eventID, err := parseEventIDParam(c)
	if err != nil {
		return nil, err
	}

	// We need to lock the ledger here so that we don't get partial results while the next milestone is being confirmed
	deps.UTXOManager.ReadLockLedger()
	defer deps.UTXOManager.ReadUnlockLedger()

	response := &ParticipationsResponse{
		Participations: make(map[string]*TrackedParticipation),
	}
	if err := deps.ParticipationManager.ForEachPastParticipation(eventID, func(trackedParticipation *participation.TrackedParticipation) bool {
		t := &TrackedParticipation{
			MessageID:           trackedParticipation.MessageID.ToHex(),
			Amount:              trackedParticipation.Amount,
			StartMilestoneIndex: trackedParticipation.StartIndex,
			EndMilestoneIndex:   trackedParticipation.EndIndex,
		}
		response.Participations[trackedParticipation.OutputID.ToHex()] = t
		return true
	}); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "error fetching past participations: %s", err)
	}
	return response, nil
}
