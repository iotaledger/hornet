package referendum

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/referendum"
	"github.com/gohornet/hornet/pkg/restapi"
)

var (
	NullReferendumID = referendum.ReferendumID{}
)

// ReferendumIDFromHex creates a ReferendumID from a hex string representation.
func ReferendumIDFromHex(hexString string) (referendum.ReferendumID, error) {

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return NullReferendumID, err
	}

	if len(b) != referendum.ReferendumIDLength {
		return referendum.ReferendumID{}, fmt.Errorf("unknown referendumID length (%d)", len(b))
	}

	var referendumID referendum.ReferendumID
	copy(referendumID[:], b)
	return referendumID, nil
}

func parseReferendumIDParam(c echo.Context) (referendum.ReferendumID, error) {

	referendumIDHex := strings.ToLower(c.Param(ParameterReferendumID))
	if referendumIDHex == "" {
		return NullReferendumID, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterReferendumID)
	}

	referendumID, err := ReferendumIDFromHex(referendumIDHex)
	if err != nil {
		return NullReferendumID, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid referendum ID: %s, error: %s", referendumIDHex, err)
	}

	return referendumID, nil
}

func getReferendums(_ echo.Context) (*ReferendumsResponse, error) {
	referendums, err := deps.ReferendumManager.Referendums()
	if err != nil {
		return nil, err
	}

	referendumIDs := []string{}
	for _, referendum := range referendums {
		referendumID, err := referendum.ID()
		if err != nil {
			return nil, err
		}
		referendumIDs = append(referendumIDs, hex.EncodeToString(referendumID[:]))
	}

	return &ReferendumsResponse{ReferendumIDs: referendumIDs}, nil
}

func createReferendum(c echo.Context) (*CreateReferendumResponse, error) {

	//TODO: add support for binary representation too?

	referendum := &referendum.Referendum{}
	if err := c.Bind(referendum); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request! Error: %s", err)
	}

	referendumID, err := deps.ReferendumManager.StoreReferendum(referendum)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid referendum, error: %s", err)
	}

	return &CreateReferendumResponse{
		ReferendumID: hex.EncodeToString(referendumID[:]),
	}, nil
}

func getReferendum(c echo.Context) (*referendum.Referendum, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	referendum, err := deps.ReferendumManager.Referendum(referendumID)
	if err != nil {
		return nil, err
	}

	if referendum == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "referendum not found: %s", hex.EncodeToString(referendumID[:]))
	}

	return referendum, nil
}

func deleteReferendum(c echo.Context) error {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil
	}

	return deps.ReferendumManager.DeleteReferendum(referendumID)
}

func getReferendumStatus(c echo.Context) (*ReferendumStatusResponse, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	ref, err := deps.ReferendumManager.Referendum(referendumID)
	if err != nil {
		return nil, err
	}

	if ref == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "referendum not found: %s", hex.EncodeToString(referendumID[:]))
	}

	response := &ReferendumStatusResponse{}

	// For each referendum, iterate over all questions
	for idx, value := range ref.Questions {
		questionIndex := uint8(idx)
		question := value.(*referendum.Question) // force cast here since we are sure the stored Referendum is valid

		questionStatus := QuestionStatus{}
		// For each question, iterate over all answers. Include 0 here, since that is valid, i.e. answer skipped by voter
		for idx := 0; idx <= len(question.Answers); idx++ {
			answerIndex := uint8(idx)

			currentBalance, err := deps.ReferendumManager.CurrentBalanceForReferendum(referendumID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}

			totalBalance, err := deps.ReferendumManager.TotalBalanceForReferendum(referendumID, questionIndex, answerIndex)
			if err != nil {
				return nil, err
			}
			questionStatus.Answers = append(questionStatus.Answers, AnswerStatus{
				Current:     currentBalance,
				Accumulated: totalBalance,
			})
		}
		response.Questions = append(response.Questions, questionStatus)
	}

	return response, nil
}
