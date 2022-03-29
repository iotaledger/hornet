package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {

	filter_self_compiled := versionFilterFunc(fixVersion("6875ccee"))
	require.False(t, filter_self_compiled("6875ccee"))      // self-compiled
	require.False(t, filter_self_compiled("v2.0.0"))        // major
	require.False(t, filter_self_compiled("v2.0.0-alpha1")) // major - pre-release

	filter_stable_version := versionFilterFunc(fixVersion("v1.1.3"))
	require.False(t, filter_stable_version("6875ccee"))      // self-compiled
	require.True(t, filter_stable_version("v1.2.0"))         // same major
	require.False(t, filter_stable_version("v1.2.0-rc1"))    // same major - pre-release
	require.False(t, filter_stable_version("v2.0.0"))        // other major
	require.False(t, filter_stable_version("v2.0.0-alpha1")) // other major - pre-release

	filter_pre_release := versionFilterFunc(fixVersion("v1.1.3-rc1"))
	require.False(t, filter_pre_release("6875ccee"))      // self-compiled
	require.True(t, filter_pre_release("v1.2.0"))         // same major
	require.True(t, filter_pre_release("v1.2.0-rc1"))     // same major - pre-release
	require.False(t, filter_pre_release("v2.0.0"))        // other major
	require.False(t, filter_pre_release("v2.0.0-alpha1")) // other major - pre-release
}
