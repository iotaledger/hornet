package versioncheck

import (
	"context"
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/app"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/version"
	"github.com/iotaledger/hive.go/timeutil"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:      "VersionCheck",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies
)

type dependencies struct {
	dig.In
	AppInfo        *app.AppInfo
	VersionChecker *version.VersionChecker
}

func provide(c *dig.Container) {
	if err := c.Provide(func(appInfo *app.AppInfo) *version.VersionChecker {
		return version.NewVersionChecker("gohornet", "hornet", appInfo.Version)
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	checkLatestVersion()
}

func run() {
	// create a background worker that checks for latest version every hour
	if err := Plugin.Node.Daemon().BackgroundWorker("Version update checker", func(ctx context.Context) {
		ticker := timeutil.NewTicker(checkLatestVersion, 1*time.Hour, ctx)
		ticker.WaitForGracefulShutdown()
	}, shutdown.PriorityUpdateCheck); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}

func checkLatestVersion() {
	res, err := deps.VersionChecker.CheckForUpdates()
	if err != nil {
		Plugin.LogWarnf("Update check failed: %s", err)
		return
	}

	if res.Outdated {
		Plugin.LogInfof("Update to %s available on https://github.com/gohornet/hornet/releases/latest", res.Current)
		deps.AppInfo.LatestGitHubVersion = res.Current
	}
}
