package test

import (
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/gohornet/hornet/pkg/model/coordinator"
)

func TestAddress(t *testing.T) {
	seed := "SQLWMYIVOUMMFKGCERHKEFIRJODAMPMENYLEYCJYWSJHSVFYCB9KGTK9LCWZMVSKKTTNAKGGTQFTPDZZU"
	expectedResult := "SPYSMYNKAFXBBNKUEMQGEMNVOJVXYK9YPPJZVRNPNLEYPLZEYIXZHVIBILBFHWGKVPRHTCWUFNEJMRRZX"
	securityLvl := 2

	address, err := coordinator.Address(seed, 1074, securityLvl)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, address, expectedResult)
}
