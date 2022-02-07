package participation

import "github.com/gohornet/hornet/pkg/model/milestone"

// EventsResponse defines the response of a GET RouteParticipationEvents REST API call.
type EventsResponse struct {
	// The hex encoded IDs of the found events.
	EventIDs []string `json:"eventIds"`
}

// CreateEventResponse defines the response of a POST RouteParticipationEvents REST API call.
type CreateEventResponse struct {
	// The hex encoded ID of the created participation event.
	EventID string `json:"eventId"`
}

// TrackedParticipation holds the information for each tracked participation.
type TrackedParticipation struct {
	// MessageID is the ID of the message that included the transaction that created the output the participation was made.
	MessageID string `json:"messageId"`
	// Amount is the amount of tokens that were included in the output the participation was made.
	Amount uint64 `json:"amount"`
	// StartMilestoneIndex is the milestone index the participation started.
	StartMilestoneIndex milestone.Index `json:"startMilestoneIndex"`
	// EndMilestoneIndex is the milestone index the participation ended. 0 if the participation is still active.
	EndMilestoneIndex milestone.Index `json:"endMilestoneIndex"`
}

// OutputStatusResponse defines the response of a GET RouteOutputStatus REST API call.
type OutputStatusResponse struct {
	// Participations holds the participations that were created in the output.
	Participations map[string]*TrackedParticipation `json:"participations"`
}

// AddressReward holds the amount and token symbol for a certain reward.
type AddressReward struct {
	// Amount is the staking reward.
	Amount uint64 `json:"amount"`
	// Symbol is the symbol of the rewarded tokens.
	Symbol string `json:"symbol"`
	// MinimumReached tells whether the minimum rewards required to be included in the staking results are reached.
	MinimumReached bool `json:"minimumReached"`
}

// AddressRewardsResponse defines the response of a GET RouteAddressBech32Status or RouteAddressEd25519Status REST API call.
type AddressRewardsResponse struct {
	// Rewards is a map of rewards per event.
	Rewards map[string]*AddressReward `json:"rewards"`
	// MilestoneIndex is the milestone index the rewards were calculated for.
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
}

// AddressOutputsResponse defines the response of a GET RouteAddressBech32Outputs or RouteAddressEd25519Outputs REST API call.
type AddressOutputsResponse struct {
	// Outputs is a map of output status per outputID.
	Outputs map[string]*OutputStatusResponse `json:"outputs"`
}

// RewardsResponse defines the response of a GET RouteAdminRewards REST API call and contains the rewards for each address.
type RewardsResponse struct {
	// Symbol is the symbol of the rewarded tokens.
	Symbol string `json:"symbol"`
	// MilestoneIndex is the milestone index the rewards were calculated for.
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
	// TotalRewards is the total reward.
	TotalRewards uint64 `json:"totalRewards"`
	// Checksum is the SHA256 checksum of the staking amount and rewards calculated for this MilestoneIndex.
	Checksum string `json:"checksum"`
	// Rewards is a map of rewards per address.
	Rewards map[string]uint64 `json:"rewards"`
}

// ParticipationsResponse defines the response of a GET RouteAdminActiveParticipations or RouteAdminPastParticipations REST API call.
type ParticipationsResponse struct {
	// Participations holds the participations that are/were tracked.
	Participations map[string]*TrackedParticipation `json:"participations"`
}
