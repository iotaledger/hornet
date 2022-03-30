package version

import (
	"testing"

	"github.com/hashicorp/go-version"
	goversion "github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
	"github.com/tcnksm/go-latest"
)

type versionCheckerMock struct {
	tags              []string
	fixVersionStrFunc latest.FixVersionStrFunc
	tagFilterFunc     latest.TagFilterFunc
}

func (ver *versionCheckerMock) Validate() error {
	return nil
}

func (ver *versionCheckerMock) Fetch() (*latest.FetchResponse, error) {

	var versions []*goversion.Version
	var malformeds []string
	fr := &latest.FetchResponse{
		Versions:   versions,
		Malformeds: malformeds,
		Meta:       &latest.Meta{},
	}

	for _, name := range ver.tags {
		if !ver.tagFilterFunc(name) {
			fr.Malformeds = append(fr.Malformeds, name)
			continue
		}
		v, err := version.NewVersion(ver.fixVersionStrFunc(name))
		if err != nil {
			fr.Malformeds = append(fr.Malformeds, ver.fixVersionStrFunc(name))
			continue
		}
		fr.Versions = append(fr.Versions, v)
	}

	return fr, nil
}

// newVersionCheckerMock creates a new VersionChecker that can be used for tests.
func newVersionCheckerMock(version string, tags []string) *VersionChecker {

	fixedAppVersion := fixVersion(version)

	return &VersionChecker{
		fixedAppVersion: fixedAppVersion,
		versionSource: &versionCheckerMock{
			tags:              tags,
			fixVersionStrFunc: fixVersion,
			tagFilterFunc:     versionFilterFunc(fixedAppVersion),
		},
	}
}

func TestVersionFilterFunc(t *testing.T) {

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

	filter_pre_release_rc := versionFilterFunc(fixVersion("v1.1.3-rc1"))
	require.False(t, filter_pre_release_rc("6875ccee"))      // self-compiled
	require.True(t, filter_pre_release_rc("v1.2.0"))         // same major
	require.True(t, filter_pre_release_rc("v1.2.0-rc1"))     // same major - pre-release
	require.False(t, filter_pre_release_rc("v2.0.0"))        // other major
	require.False(t, filter_pre_release_rc("v2.0.0-alpha1")) // other major - pre-release

	filter_pre_release_alpha := versionFilterFunc(fixVersion("v1.1.3-alpha1"))
	require.False(t, filter_pre_release_alpha("6875ccee"))      // self-compiled
	require.True(t, filter_pre_release_alpha("v1.2.0"))         // same major
	require.True(t, filter_pre_release_alpha("v1.2.0-rc1"))     // same major - pre-release
	require.False(t, filter_pre_release_alpha("v2.0.0"))        // other major
	require.False(t, filter_pre_release_alpha("v2.0.0-alpha1")) // other major - pre-release
}

func TestVersion(t *testing.T) {

	var checkResponse *latest.CheckResponse
	var err error

	version_self_compiled := newVersionCheckerMock("6875ccee", []string{"6875ccee", "v1.2.0", "v1.2.0-rc1", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_self_compiled.CheckForUpdates()
	require.Error(t, err) // no updates available
	require.Nil(t, checkResponse)

	version_stable_version := newVersionCheckerMock("v1.1.3", []string{"6875ccee", "v1.2.0", "v1.2.0-rc1", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_stable_version.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.2.0", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v1.2.0-rc1", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)

	version_pre_release_rc := newVersionCheckerMock("v1.1.3-rc1", []string{"6875ccee", "v1.2.0", "v1.2.0-rc1", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_pre_release_rc.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.2.0", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)

	version_pre_release_rc1 := newVersionCheckerMock("v1.1.3-rc1", []string{"6875ccee", "v1.1.3-rc10", "v1.1.3-rc11", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_pre_release_rc1.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.1.3-rc.11", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)

	version_pre_release_rc9 := newVersionCheckerMock("v1.1.3-rc9", []string{"6875ccee", "v1.1.3-rc10", "v1.1.3-rc11", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_pre_release_rc9.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.1.3-rc.11", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)

	version_pre_release_rc10 := newVersionCheckerMock("v1.1.3-rc10", []string{"6875ccee", "v1.1.3-rc10", "v1.1.3-rc11", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_pre_release_rc10.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.1.3-rc.11", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)

	version_pre_release_alpha := newVersionCheckerMock("v1.1.3-alpha1", []string{"6875ccee", "v1.2.0", "v1.2.0-rc1", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_pre_release_alpha.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.2.0", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)

	version_pre_release_alpha9 := newVersionCheckerMock("v1.1.3-alpha9", []string{"6875ccee", "v1.1.3-alpha10", "v1.1.3-alpha11", "v2.0.0", "v2.0.0-alpha1"})
	checkResponse, err = version_pre_release_alpha9.CheckForUpdates()
	require.NoError(t, err)
	require.NotNil(t, checkResponse)
	require.Equal(t, "1.1.3-alpha.11", checkResponse.Current)
	require.EqualValues(t, []string{"6875ccee", "v2.0.0", "v2.0.0-alpha1"}, checkResponse.Malformeds)
	require.False(t, checkResponse.Latest)
	require.True(t, checkResponse.Outdated)
	require.False(t, checkResponse.New)
}
