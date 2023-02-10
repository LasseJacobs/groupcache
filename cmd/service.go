package cmd

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"golang.org/x/exp/slog"
	"os"
	"runtime"
	"time"
)

type ServiceCommand struct {
	Revision          string
	Version           string
	VersionPrerelease string
	Ui                cli.Ui
	ShutdownCh        <-chan struct{}
	args              []string
}

func (c *ServiceCommand) Help() string {
	return ""
}

func (c *ServiceCommand) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	// Parse our configs
	c.args = args
	config := c.readConfig()
	if config == nil {
		return 1
	}
	c.args = args

	// Check GOMAXPROCS
	if runtime.GOMAXPROCS(0) == 1 {
		c.Ui.Error("WARNING: It is highly recommended to set GOMAXPROCS higher than 1")
	}

	// Setup the log outputs
	logGate, logWriter, logOutput := c.setupLoggers(config)
	if logWriter == nil {
		return 1
	}

	/* Setup telemetry
	Aggregate on 10 second intervals for 1 minute. Expose the
	metrics over stderr when there is a SIGUSR1 received.
	*/
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(inm)
	metricsConf := metrics.DefaultConfig("consul")

	// Optionally configure a statsite sink if provided
	if config.StatsiteAddr != "" {
		sink, err := metrics.NewStatsiteSink(config.StatsiteAddr)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to start statsite sink. Got: %s", err))
			return 1
		}
		fanout := metrics.FanoutSink{inm, sink}
		metrics.NewGlobal(metricsConf, fanout)

	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, inm)
	}

	// Create the agent
	if err := c.setupAgent(config, logOutput, logWriter); err != nil {
		return 1
	}
	defer c.agent.Shutdown()
	if c.rpcServer != nil {
		defer c.rpcServer.Shutdown()
	}
	if c.httpServer != nil {
		defer c.httpServer.Shutdown()
	}

	// Join startup nodes if specified
	if err := c.startupJoin(config); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Register the services
	for _, service := range config.Services {
		ns := service.NodeService()
		chkType := service.CheckType()
		if err := c.agent.AddService(ns, chkType); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to register service '%s': %v", service.Name, err))
			return 1
		}
	}

	// Register the checks
	for _, check := range config.Checks {
		health := check.HealthCheck(config.NodeName)
		chkType := &check.CheckType
		if err := c.agent.AddCheck(health, chkType); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to register check '%s': %v %v", check.Name, err, check))
			return 1
		}
	}

	// Let the agent know we've finished registration
	c.agent.StartSync()

	c.Ui.Output("Consul agent running!")
	c.Ui.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	c.Ui.Info(fmt.Sprintf("    Datacenter: '%s'", config.Datacenter))
	c.Ui.Info(fmt.Sprintf("        Server: %v (bootstrap: %v)", config.Server, config.Bootstrap))
	c.Ui.Info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, DNS: %d, RPC: %d)", config.ClientAddr,
		config.Ports.HTTP, config.Ports.DNS, config.Ports.RPC))
	c.Ui.Info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddr,
		config.Ports.SerfLan, config.Ports.SerfWan))
	c.Ui.Info(fmt.Sprintf("Gossip encrypt: %v, RPC-TLS: %v, TLS-Incoming: %v",
		config.EncryptKey != "", config.VerifyOutgoing, config.VerifyIncoming))

	// Enable log streaming
	c.Ui.Info("")
	c.Ui.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Wait for exit
	return c.handleSignals(config)
}

func (c *ServiceCommand) Synopsis() string {
	return ""
}

// readConfig is responsible for setup of our configuration using
// the command line and any file configs
func (c *ServiceCommand) readConfig() *Config {
	var cmdConfig Config
	//var configFiles []string
	cmdFlags := flag.NewFlagSet("agent", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.StringVar(&cmdConfig.LogLevel, "log-level", "", "log level")
	cmdFlags.StringVar(&cmdConfig.NodeName, "node", "", "node name")

	cmdFlags.StringVar(&cmdConfig.BindAddr, "bind", "", "address to bind server listeners to")

	if err := cmdFlags.Parse(c.args); err != nil {
		return nil
	}

	config := DefaultConfig()
	/*
		if len(configFiles) > 0 {
			fileConfig, err := ReadConfigPaths(configFiles)
			if err != nil {
				c.Ui.Error(err.Error())
				return nil
			}

			config = MergeConfig(config, fileConfig)
		}
		config = MergeConfig(config, &cmdConfig)
	*/

	if config.NodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error determining hostname: %s", err))
			return nil
		}
		config.NodeName = hostname
	}

	// Set the version info
	config.Revision = c.Revision
	config.Version = c.Version
	config.VersionPrerelease = c.VersionPrerelease

	return config
}

func (c *ServiceCommand) setupLogger(config *Config) *slog.Logger {
	return nil
}
