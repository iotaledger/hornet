//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package autopeering_test

import (
	"os"
	"testing"

	"github.com/iotaledger/hornet/v2/integration-tests/tester/framework"
)

var f *framework.Framework

// TestMain gets called by the test utility and is executed before any other test in this package.
// It is therefore used to initialize the integration testing framework.
func TestMain(m *testing.M) {
	var err error
	if f, err = framework.Instance(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
