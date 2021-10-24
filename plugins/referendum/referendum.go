package referendum

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/referendum"
	"github.com/gohornet/hornet/pkg/restapi"
)

func parseReferendumIDParam(c echo.Context) (hornet.MessageID, error) {

	referendumIDHex := strings.ToLower(c.Param(ParameterReferendumID))
	if referendumIDHex == "" {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterReferendumID)
	}

	referendumID, err := hornet.MessageIDFromHex(referendumIDHex)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid referendum ID: %s, error: %s", referendumIDHex, err)
	}

	return referendumID, nil
}

func parseQuestionIndexParam(c echo.Context) (int, error) {

	questionIndexString := strings.ToLower(c.Param(ParameterQuestionIndex))
	if questionIndexString == "" {
		return 0, errors.WithMessagef(restapi.ErrInvalidParameter, "parameter \"%s\" not specified", ParameterQuestionIndex)
	}

	questionIndex, err := strconv.ParseUint(questionIndexString, 10, 64)
	if err != nil {
		return 0, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid question index: %s, error: %s", questionIndexString, err)
	}

	return int(questionIndex), nil
}

func getReferenda(_ echo.Context) (*referendum.ReferendaResponse, error) {
	return deps.ReferendumManager.Referenda()
}

func createReferendum(c echo.Context) (*referendum.CreateReferendumResponse, error) {

	request := &referendum.CreateReferendumRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request! Error: %s", err)
	}

	return deps.ReferendumManager.CreateReferendum()
}

func getReferendum(c echo.Context) (*referendum.ReferendumResponse, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	return deps.ReferendumManager.Referendum(referendumID)
}

func deleteReferendum(c echo.Context) error {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil
	}

	return deps.ReferendumManager.DeleteReferendum(referendumID)
}

func getReferendumQuestions(c echo.Context) (*referendum.ReferendumQuestionsResponse, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	return deps.ReferendumManager.ReferendumQuestions(referendumID)
}

func getReferendumQuestion(c echo.Context) (*referendum.ReferendumQuestionResponse, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	questionIndex, err := parseQuestionIndexParam(c)
	if err != nil {
		return nil, err
	}

	return deps.ReferendumManager.ReferendumQuestion(referendumID, questionIndex)
}

func getReferendumStatus(c echo.Context) (*referendum.ReferendumStatusResponse, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	return deps.ReferendumManager.ReferendumStatus(referendumID)
}

func getReferendumQuestionStatus(c echo.Context) (*referendum.ReferendumQuestionStatusResponse, error) {

	referendumID, err := parseReferendumIDParam(c)
	if err != nil {
		return nil, err
	}

	questionIndex, err := parseQuestionIndexParam(c)
	if err != nil {
		return nil, err
	}

	return deps.ReferendumManager.ReferendumQuestionStatus(referendumID, questionIndex)
}
