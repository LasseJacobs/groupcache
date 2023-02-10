package cmd

// Ports is used to simplify the configuration by
// providing default ports, and allowing the addresses
// to only be specified once
type PortConfig struct {
	HTTP   int // HTTP API
	Server int // Server internal RPC
}

// Config is the configuration that can be set for an Agent.
// Some of this is configurable as CLI flags, but most must
// be set using a configuration file.
type Config struct {
	// LogLevel is the level of the logs to putout
	LogLevel string `mapstructure:"log_level"`

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string `mapstructure:"node_name"`

	// BindAddr is used to control the address we bind to.
	// If not specified, the first private IP we find is used.
	// This controls the address we use for cluster facing
	// services (Gossip, Server RPC)
	BindAddr string `mapstructure:"bind_addr"`

	// Port configurations
	Ports PortConfig

	// StartJoin is a list of addresses to attempt to join when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	StartJoin []string `mapstructure:"start_join"`

	// Revision is the GitCommit this maps to
	Revision string `mapstructure:"-"`

	// Version is the release version number
	Version string `mapstructure:"-"`

	// VersionPrerelease is a label for pre-release builds
	VersionPrerelease string `mapstructure:"-"`
}

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	return &Config{
		LogLevel: "INFO",
		BindAddr: "0.0.0.0",
		Ports: PortConfig{
			HTTP:   8500,
			Server: 8300,
		},
	}
}
