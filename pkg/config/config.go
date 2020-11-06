package config

import (
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
)

const (
	// FlagFilePathConfig is the name of the flag to define the config file path.
	FlagFilePathConfig = "config"
	// FlagFilePathPeeringConfig is the name of the flag to define the peering file path.
	FlagFilePathPeeringConfig = "peeringConfig"
	// FlagFilePathProfilesConfig is the name of the flag to define the profiles file path.
	FlagFilePathProfilesConfig = "profilesConfig"
)

// Configuration holds all configuration options for a HORNET node.
type Configuration struct {
	opts *ConfigurationOptions

	// Configurations
	NodeConfig     *configuration.Configuration
	PeeringConfig  *configuration.Configuration
	ProfilesConfig *configuration.Configuration
}

// New creates a new Configuration.
func New(opts ...ConfigurationOption) *Configuration {
	configOpts := &ConfigurationOptions{}
	configOpts.apply(defaultConfigurationOptions...)
	configOpts.apply(opts...)

	return &Configuration{
		NodeConfig:     configuration.New(),
		PeeringConfig:  configuration.New(),
		ProfilesConfig: configuration.New(),
		opts:           configOpts,
	}
}

// the default options applied to the Manager.
var defaultConfigurationOptions = []ConfigurationOption{
	WithNonHiddenFlags(map[string]struct{}{
		"config":              {},
		"config-dir":          {},
		"node.disablePlugins": {},
		"node.enablePlugins":  {},
		"peeringConfig":       {},
		"profilesConfig":      {},
		"useProfile":          {},
		"version":             {},
		"help":                {},
	}),
	WithConfigFilePath("config.json"),
	WithPeeringFilePath("peering.json"),
	WithProfilesFilePath("profiles.json"),
	WithConfigFlagSet(flag.NewFlagSet("", flag.ContinueOnError)),
	WithPeeringFlagSet(flag.NewFlagSet("", flag.ContinueOnError)),
}

// ConfigurationOptions defines options for a Configuration.
type ConfigurationOptions struct {
	// a list of flags which should be printed via --help
	NonHiddenFlags map[string]struct{}
	// the path to the config file
	ConfigFilePath string
	// the path of the peering file
	PeeringFilePath string
	// the path of the profiles file
	ProfilesFilePath string
	// the flag set for the config options.
	ConfigFlagSet *flag.FlagSet
	// the flag set for the peering options.
	PeeringFlagSet *flag.FlagSet
}

// ConfigurationOption is a function setting a ConfigurationOptions option.
type ConfigurationOption func(opts *ConfigurationOptions)

// WithNonHiddenFlags defines a list of flags which should be printed via --help
func WithNonHiddenFlags(nonHiddenFlags map[string]struct{}) ConfigurationOption {
	return func(opts *ConfigurationOptions) {
		opts.NonHiddenFlags = nonHiddenFlags
	}
}

// WithConfigFilePath defines the config file path.
func WithConfigFilePath(filePath string) ConfigurationOption {
	return func(opts *ConfigurationOptions) {
		opts.ConfigFilePath = filePath
	}
}

// WithPeeringFilePath defines the peering file path.
func WithPeeringFilePath(filePath string) ConfigurationOption {
	return func(opts *ConfigurationOptions) {
		opts.PeeringFilePath = filePath
	}
}

// WithProfilesFilePath defines the profiles file path.
func WithProfilesFilePath(filePath string) ConfigurationOption {
	return func(opts *ConfigurationOptions) {
		opts.ProfilesFilePath = filePath
	}
}

// WithConfigFlagSet defines the config flag set.
func WithConfigFlagSet(flagset *flag.FlagSet) ConfigurationOption {
	return func(opts *ConfigurationOptions) {
		opts.ConfigFlagSet = flagset
	}
}

// WithPeeringFlagSet defines the peering flag set.
func WithPeeringFlagSet(flagset *flag.FlagSet) ConfigurationOption {
	return func(opts *ConfigurationOptions) {
		opts.PeeringFlagSet = flagset
	}
}

// applies the given ConfigurationOption.
func (co *ConfigurationOptions) apply(opts ...ConfigurationOption) {
	for _, opt := range opts {
		opt(co)
	}
}

// HideConfigFlags hides all non essential flags from the help/usage text.
func (c *Configuration) HideConfigFlags() {
	flag.VisitAll(func(f *flag.Flag) {
		_, notHidden := c.opts.NonHiddenFlags[f.Name]
		f.Hidden = !notHidden
	})
	c.opts.ConfigFlagSet.VisitAll(func(f *flag.Flag) {
		_, notHidden := c.opts.NonHiddenFlags[f.Name]
		f.Hidden = !notHidden
	})
	c.opts.PeeringFlagSet.VisitAll(func(f *flag.Flag) {
		_, notHidden := c.opts.NonHiddenFlags[f.Name]
		f.Hidden = !notHidden
	})
}

// ParseFlags defines and parses the command-line flags from os.Args[1:].
func (c *Configuration) ParseFlags() {
	flag.CommandLine.AddFlagSet(c.opts.ConfigFlagSet)
	flag.CommandLine.AddFlagSet(c.opts.PeeringFlagSet)
	flag.Parse()
}

// FetchConfig fetches all config values (order: default, files, env, flags).
func (c *Configuration) FetchConfig() error {

	if err := c.NodeConfig.LoadFile(c.opts.ConfigFilePath); err != nil {
		if hasFlag(flag.CommandLine, FlagFilePathConfig) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No config file found via '%s'. Loading default settings.", c.opts.ConfigFilePath)
	}

	if err := c.PeeringConfig.LoadFile(c.opts.PeeringFilePath); err != nil {
		if hasFlag(flag.CommandLine, FlagFilePathPeeringConfig) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No peering config file found via '%s'. Loading default settings.", c.opts.PeeringFilePath)
	}

	if err := c.ProfilesConfig.LoadFile(c.opts.ProfilesFilePath); err != nil {
		if hasFlag(flag.CommandLine, FlagFilePathProfilesConfig) {
			// if a file was explicitly specified, raise the error
			return err
		}
		fmt.Printf("No profiles config file found via '%s'. Loading default settings.", c.opts.ProfilesFilePath)
	}

	// load the flags to set the default values
	if err := c.NodeConfig.LoadFlagSet(c.opts.ConfigFlagSet); err != nil {
		return err
	}

	if err := c.PeeringConfig.LoadFlagSet(c.opts.PeeringFlagSet); err != nil {
		return err
	}

	// load the env vars after default values from flags were set (otherwise the env vars are not added because the keys don't exist)
	if err := c.NodeConfig.LoadEnvironmentVars(""); err != nil {
		return err
	}

	// load the flags again to overwrite env vars that were also set via command line
	if err := c.NodeConfig.LoadFlagSet(c.opts.ConfigFlagSet); err != nil {
		return err
	}

	if err := c.PeeringConfig.LoadFlagSet(c.opts.PeeringFlagSet); err != nil {
		return err
	}

	return nil
}

// PrintConfig prints the configuration.
func (c *Configuration) PrintConfig(ignoreSettingsAtPrint ...[]string) {
	c.NodeConfig.Print(ignoreSettingsAtPrint...)
}

func hasFlag(flagSet *flag.FlagSet, name string) bool {
	has := false
	flagSet.Visit(func(f *flag.Flag) {
		if f.Name == name {
			has = true
		}
	})
	return has
}
