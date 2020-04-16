package test

import (
	"testing"

	"github.com/gohornet/hornet/plugins/coordinator"
)

func TestAddress(t *testing.T) {
	seed := "SQLWMYIVOUMMFKGCERHKEFIRJODAMPMENYLEYCJYWSJHSVFYCB9KGTK9LCWZMVSKKTTNAKGGTQFTPDZZU"
	expectedResult := "SPYSMYNKAFXBBNKUEMQGEMNVOJVXYK9YPPJZVRNPNLEYPLZEYIXZHVIBILBFHWGKVPRHTCWUFNEJMRRZX"
	securityLvl := 2

	address, err := coordinator.GetAddress(seed, 1074, securityLvl)
	if err != nil {
		t.Error(err)
	}

	if address != expectedResult {
		t.Errorf("Wrong address %v != %v", expectedResult, address)
	}
}
