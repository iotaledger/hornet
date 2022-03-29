package version

import (
	"strings"

	goversion "github.com/hashicorp/go-version"
	"github.com/tcnksm/go-latest"
)

// fixVersion fixes broken version strings.
func fixVersion(version string) string {
	ver := strings.Replace(version, "v", "", 1)
	if !strings.Contains(ver, "-rc.") {
		ver = strings.Replace(ver, "-rc", "-rc.", 1)
	}
	return ver
}

// versionIsPreRelease checks if a version is a pre-release.
func versionIsPreRelease(version *goversion.Version) bool {
	// version is a pre-release if the string is not empty
	return version.Prerelease() != ""
}

// versionFilterFunc filters possible versions for updates based on the current AppVersion.
// If the AppVersion is self-compiled, we don't search for updates.
// We only check for any versions in the same MAJOR version. (e.g. 1.1.3 => 1.2.0)
// If the AppVersion is a pre-release, we also check for any pre-releases in the same MAJOR version. (e.g. 1.1.4-rc1 => 1.2.0-alpha1 / 1.1.5)
func versionFilterFunc(fixedAppVersion string) latest.TagFilterFunc {

	appVersion, err := goversion.NewSemver(fixedAppVersion)
	if err != nil {
		// if the AppVersion can't be parsed, it may be a self compiled version.
		// => no need to check for updates.

		// filter everything
		return func(version string) bool {
			return false
		}
	}

	appIsPreRelease := versionIsPreRelease(appVersion)
	appVersionMajor := appVersion.Segments()[0]

	// filter versions based on current AppVersion
	return func(version string) bool {
		ver, err := goversion.NewSemver(fixVersion(version))
		if err != nil {
			// every version that can't be parsed is ignored.
			return false
		}

		if !appIsPreRelease && versionIsPreRelease(ver) {
			// the current AppVersion is not a pre-release.
			// => ignore all pre-releases.
			return false
		}

		// we only check for any versions in the same MAJOR version.
		return appVersionMajor == ver.Segments()[0]
	}
}

// VersionChecker can be used to check for updates on GitHub.
type VersionChecker struct {
	fixedAppVersion string
	versionSource   latest.Source
}

// NewVersionChecker creates a new VersionChecker that can be used to check for updates on GitHub.
func NewVersionChecker(owner string, repository string, version string) *VersionChecker {

	fixedAppVersion := fixVersion(version)

	return &VersionChecker{
		fixedAppVersion: fixedAppVersion,
		versionSource: &latest.GithubTag{
			Owner:             owner,
			Repository:        repository,
			FixVersionStrFunc: fixVersion,
			TagFilterFunc:     versionFilterFunc(fixedAppVersion),
		},
	}
}

// CheckForUpdates checks for latest updates on GitHub.
func (v *VersionChecker) CheckForUpdates() (*latest.CheckResponse, error) {
	return latest.Check(v.versionSource, v.fixedAppVersion)
}
