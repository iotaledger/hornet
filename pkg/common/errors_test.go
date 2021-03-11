package common_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gohornet/hornet/pkg/common"
)

func TestSoftError_Error(t *testing.T) {

	var anError = errors.New("an error")

	aWrappedSoftErr := fmt.Errorf("wrap me up: %w", common.SoftError{Err: anError})

	var isSoftErr common.SoftError
	fmt.Println(errors.As(aWrappedSoftErr, &isSoftErr))
}
