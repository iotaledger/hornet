package main

import (
	"fmt"
	"os"

	"github.com/iotaledger/hive.go/apputils/config"
	"github.com/iotaledger/hive.go/core/app"

	hornetApp "github.com/iotaledger/hornet/v2/core/app"
)

func createMarkdownFile(app *app.App, markdownHeaderPath string, markdownFilePath string, ignoreFlags map[string]struct{}, replaceTopicNames map[string]string) {

	var markdownHeader []byte

	if markdownHeaderPath != "" {
		var err error
		markdownHeader, err = os.ReadFile(markdownHeaderPath)
		if err != nil {
			panic(err)
		}
	}

	println(fmt.Sprintf("Create markdown file for %s...", app.Info().Name))
	md := config.GetConfigurationMarkdown(app.Config(), app.FlagSet(), ignoreFlags, replaceTopicNames)
	os.WriteFile(markdownFilePath, append(markdownHeader, []byte(md)...), os.ModePerm)
	println(fmt.Sprintf("Markdown file for %s stored: %s", app.Info().Name, markdownFilePath))
}

func createDefaultConfigFile(app *app.App, configFilePath string, ignoreFlags map[string]struct{}) {
	println(fmt.Sprintf("Create default configuration file for %s...", app.Info().Name))
	conf := config.GetDefaultAppConfigJSON(app.Config(), app.FlagSet(), ignoreFlags)
	os.WriteFile(configFilePath, []byte(conf), os.ModePerm)
	println(fmt.Sprintf("Default configuration file for %s stored: %s", app.Info().Name, configFilePath))
}

func main() {

	// MUST BE LOWER CASE
	ignoreFlags := make(map[string]struct{})
	ignoreFlags["db.debug"] = struct{}{}
	ignoreFlags["p2p.autopeering.inboundpeers"] = struct{}{}
	ignoreFlags["p2p.autopeering.outboundpeers"] = struct{}{}
	ignoreFlags["p2p.autopeering.saltlifetime"] = struct{}{}

	replaceTopicNames := make(map[string]string)
	replaceTopicNames["app"] = "Application"
	replaceTopicNames["p2p"] = "Peer to Peer"
	replaceTopicNames["db"] = "Database"
	replaceTopicNames["pow"] = "Proof of Work"
	replaceTopicNames["jwtAuth"] = "JWT Auth"
	replaceTopicNames["warpsync"] = "WarpSync"
	replaceTopicNames["api"] = "API"
	replaceTopicNames["tipsel"] = "Tipselection"
	replaceTopicNames["inx"] = "INX"

	application := hornetApp.App()

	createMarkdownFile(
		application,
		"configuration_header.md",
		"../../documentation/docs/references/configuration.md",
		ignoreFlags,
		replaceTopicNames,
	)

	createDefaultConfigFile(
		application,
		"../../config_defaults.json",
		ignoreFlags,
	)

	createDefaultConfigFile(
		application,
		"../../docker/config_defaults.json",
		ignoreFlags,
	)
}
