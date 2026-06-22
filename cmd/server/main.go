// Package main provides the entry point for the CLI Proxy API server.
// This server acts as a proxy that provides OpenAI/Gemini/Claude compatible API interfaces
// for CLI models, allowing CLI models to be used with tools and libraries designed for standard AI APIs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	configaccess "github.com/router-for-me/CLIProxyAPI/v7/internal/access/config_access"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/home"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/managementasset"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/redisqueue"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/safemode"
	_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v7/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

var (
	Version           = "dev"
	Commit            = "none"
	BuildDate         = "unknown"
	DefaultConfigPath = ""
)

// init initializes the shared logger setup.
func init() {
	logging.SetupBaseLogger()
	buildinfo.Version = Version
	buildinfo.Commit = Commit
	buildinfo.BuildDate = BuildDate
}

func shouldStartExampleAPIKeyWarningServer(cfg *config.Config, commandMode, cloudConfigMissing, homeMode bool) bool {
	if cfg == nil || commandMode || homeMode || cloudConfigMissing {
		return false
	}
	return safemode.HasExampleAPIKeys(cfg.APIKeys)
}

// main is the entry point of the application.
// It parses command-line flags, loads configuration, and starts the appropriate
// service based on the provided flags (login, codex-login, or server mode).
func main() {
	fmt.Printf("CLIProxyAPI Version: %s, Commit: %s, BuiltAt: %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate)

	// Command-line flags to control the application's behavior.
	var configPath string
	var password string
	var homeJWT string
	var homeDisableClusterDiscovery bool
	var localModel bool
	var port int

	// Define command-line flags for different operation modes.
	flag.StringVar(&configPath, "config", DefaultConfigPath, "Configure File Path")
	flag.StringVar(&password, "password", "", "")
	flag.StringVar(&homeJWT, "home-jwt", "", "Home control plane JWT for mTLS certificate bootstrap and connection")
	flag.BoolVar(&homeDisableClusterDiscovery, "home-disable-cluster-discovery", false, "Disable Home CLUSTER NODES discovery and keep using the configured -home-jwt address")
	flag.BoolVar(&localModel, "local-model", false, "Use embedded model catalog only, skip remote model fetching")
	flag.IntVar(&port, "port", 0, "Override the server port from config")

	flag.CommandLine.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(out, "Usage of %s\n", os.Args[0])
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			if f.Name == "password" {
				return
			}
			s := fmt.Sprintf("  -%s", f.Name)
			name, unquoteUsage := flag.UnquoteUsage(f)
			if name != "" {
				s += " " + name
			}
			if len(s) <= 4 {
				s += "	"
			} else {
				s += "\n    "
			}
			if unquoteUsage != "" {
				s += unquoteUsage
			}
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				s += fmt.Sprintf(" (default %s)", f.DefValue)
			}
			_, _ = fmt.Fprint(out, s+"\n")
		})
	}

	// Parse the command-line flags.
	flag.Parse()

	// Core application variables.
	var err error
	var cfg *config.Config
	var isCloudDeploy bool
	var configLoadedFromHome bool

	wd, err := os.Getwd()
	if err != nil {
		log.Errorf("failed to get working directory: %v", err)
		return
	}

	// Load environment variables from .env if present.
	if errLoad := godotenv.Load(filepath.Join(wd, ".env")); errLoad != nil {
		if !errors.Is(errLoad, os.ErrNotExist) {
			log.WithError(errLoad).Warn("failed to load .env file")
		}
	}

	lookupEnv := func(keys ...string) (string, bool) {
		for _, key := range keys {
			if value, ok := os.LookupEnv(key); ok {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed, true
				}
			}
		}
		return "", false
	}

	if strings.TrimSpace(homeJWT) == "" {
		if v, ok := lookupEnv("HOME_JWT", "home_jwt"); ok {
			homeJWT = v
		}
	}

	// Check for cloud deploy mode only on first execution
	// Read env var name in uppercase: DEPLOY
	deployEnv := os.Getenv("DEPLOY")
	if deployEnv == "cloud" {
		isCloudDeploy = true
	}

	// Determine and load the configuration file.
	// Prefer the Postgres store when configured, otherwise fallback to git or local files.
	var configFilePath string
	if strings.TrimSpace(homeJWT) != "" {
		configLoadedFromHome = true
		ctxHome, cancelHome := context.WithTimeout(context.Background(), 30*time.Second)
		homeCfg, errHomeCfg := home.ConfigFromJWT(ctxHome, homeJWT)
		cancelHome()
		if errHomeCfg != nil {
			log.Errorf("invalid -home-jwt: %v", errHomeCfg)
			return
		}
		if homeDisableClusterDiscovery {
			homeCfg.DisableClusterDiscovery = true
		}
		homeClient := home.New(homeCfg)
		defer homeClient.Close()

		ctxHomeConfig, cancelHomeConfig := context.WithTimeout(context.Background(), 30*time.Second)
		raw, errGetConfig := homeClient.GetConfig(ctxHomeConfig)
		cancelHomeConfig()
		if errGetConfig != nil {
			log.Errorf("failed to fetch config from home: %v", errGetConfig)
			return
		}

		parsed, errParseConfig := config.ParseConfigBytes(raw)
		if errParseConfig != nil {
			log.Errorf("failed to parse config payload from home: %v", errParseConfig)
			return
		}
		if parsed == nil {
			parsed = &config.Config{}
		}
		parsed.Home = homeCfg
		parsed.Port = 8317 // Default to 8317 for home mode, can be overridden by home config
		parsed.UsageStatisticsEnabled = true
		cfg = parsed

		// Keep a non-empty config path for downstream components (log paths, management assets, etc),
		// but do not require the file to exist when loading config from home.
		if strings.TrimSpace(configPath) != "" {
			configFilePath = configPath
		} else {
			configFilePath = filepath.Join(wd, "config.yaml")
		}
	} else if configPath != "" {
		configFilePath = configPath
		cfg, err = config.LoadConfigOptional(configPath, isCloudDeploy)
	} else {
		wd, err = os.Getwd()
		if err != nil {
			log.Errorf("failed to get working directory: %v", err)
			return
		}
		configFilePath = filepath.Join(wd, "config.yaml")
		cfg, err = config.LoadConfigOptional(configFilePath, isCloudDeploy)
	}
	if err != nil {
		log.Errorf("failed to load config: %v", err)
		return
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	// Override port from command-line flag if provided.
	if port != 0 {
		cfg.Port = port
	}

	// In cloud deploy mode, check if we have a valid configuration
	var configFileExists bool
	if isCloudDeploy {
		if configLoadedFromHome && cfg != nil {
			configFileExists = cfg.Port != 0
		} else {
			if info, errStat := os.Stat(configFilePath); errStat != nil {
				// Don't mislead: API server will not start until configuration is provided.
				log.Info("Cloud deploy mode: No configuration file detected; standing by for configuration")
				configFileExists = false
			} else if info.IsDir() {
				log.Info("Cloud deploy mode: Config path is a directory; standing by for configuration")
				configFileExists = false
			} else if cfg.Port == 0 {
				// LoadConfigOptional returns empty config when file is empty or invalid.
				// Config file exists but is empty or invalid; treat as missing config
				log.Info("Cloud deploy mode: Configuration file is empty or invalid; standing by for valid configuration")
				configFileExists = false
			} else {
				log.Info("Cloud deploy mode: Configuration file detected; starting service")
				configFileExists = true
			}
		}
	}
	redisqueue.SetUsageStatisticsEnabled(cfg.UsageStatisticsEnabled)
	redisqueue.SetRetentionSeconds(cfg.RedisUsageQueueRetentionSeconds)
	coreauth.SetQuotaCooldownDisabled(cfg.DisableCooling)

	if err = logging.ConfigureLogOutput(cfg); err != nil {
		log.Errorf("failed to configure log output: %v", err)
		return
	}

	log.Infof("CLIProxyAPI Version: %s, Commit: %s, BuiltAt: %s", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate)

	// Set the log level based on the configuration.
	util.SetLogLevel(cfg)

	if resolvedAuthDir, errResolveAuthDir := util.ResolveAuthDir(cfg.AuthDir); errResolveAuthDir != nil {
		log.Errorf("failed to resolve auth directory: %v", errResolveAuthDir)
		return
	} else {
		cfg.AuthDir = resolvedAuthDir
	}
	managementasset.SetCurrentConfig(cfg)

	commandMode := false
	cloudConfigMissing := isCloudDeploy && !configFileExists
	homeMode := configLoadedFromHome || (cfg != nil && cfg.Home.Enabled)
	if shouldStartExampleAPIKeyWarningServer(cfg, commandMode, cloudConfigMissing, homeMode) {
		matches := safemode.ExampleAPIKeys(cfg.APIKeys)
		log.WithField("api_keys", strings.Join(matches, ",")).Error("unsafe example API key configured; starting warning-only server")
		cmd.StartExampleAPIKeyWarningServer(cfg, configFilePath, matches)
		return
	}

	// Register the shared token store once so all components use the same persistence backend.
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())

	// Register built-in access providers before constructing services.
	configaccess.Register(&cfg.SDKConfig)

	// Handle different command modes based on the provided flags.
	if true {
		// In cloud deploy mode without config file, just wait for shutdown signals
		if isCloudDeploy && !configFileExists {
			// No config file available, just wait for shutdown
			cmd.WaitForCloudDeploy()
			return
		}
		if localModel {
			log.Info("Local model mode: using embedded model catalog, remote model updates disabled")
		}
		// Start the main proxy service
		managementasset.StartAutoUpdater(context.Background(), configFilePath)
		if !localModel && !cfg.Home.Enabled {
			registry.StartModelsUpdater(context.Background())
		} else if cfg.Home.Enabled {
			log.Info("Home mode: remote model updates disabled")
		}
		cmd.StartService(cfg, configFilePath, password)
	}
}
