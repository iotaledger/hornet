package versioncheck

import (
	"strings"
	"time"

	"github.com/tcnksm/go-latest"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/iotaledger/hive.go/timeutil"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:      "VersionCheck",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	githubTag *latest.GithubTag
)

type dependencies struct {
	dig.In
	AppInfo *app.AppInfo
}

func configure() {
	githubTag = &latest.GithubTag{
		Owner:             "gohornet",
		Repository:        "hornet",
		FixVersionStrFunc: fixVersion,
		TagFilterFunc:     includeVersionInCheck,
	}

	checkLatestVersion()
}

func run() {
	// create a background worker that checks for latest version every hour
	if err := Plugin.Node.Daemon().BackgroundWorker("Version update checker", func(shutdownSignal <-chan struct{}) {
		ticker := timeutil.NewTicker(checkLatestVersion, 1*time.Hour, shutdownSignal)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityUpdateCheck); err != nil {
		Plugin.Panicf("failed to start worker: %s", err)
	}
}

func fixVersion(version string) string {
	ver := strings.Replace(version, "v", "", 1)
	if !strings.Contains(ver, "-rc.") {
		ver = strings.Replace(ver, "-rc", "-rc.", 1)
	}
	return ver
}

func includeVersionInCheck(version string) bool {
	isPrerelease := func(ver string) bool {
		return strings.Contains(ver, "-rc")
	}

	if isPrerelease(deps.AppInfo.Version) {
		// When using pre-release versions, check for any updates
		return true
	}

	return !isPrerelease(version)
}

func checkLatestVersion() {

	res, err := latest.Check(githubTag, fixVersion(deps.AppInfo.Version))
	if err != nil {
		Plugin.LogWarnf("Update check failed: %s", err)
		return
	}

	if res.Outdated {
		Plugin.LogInfof("Update to %s available on https://github.com/gohornet/hornet/releases/latest", res.Current)
		deps.AppInfo.LatestGitHubVersion = res.Current
	}
}
