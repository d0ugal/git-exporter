package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/d0ugal/git-exporter/internal/collectors"
	"github.com/d0ugal/git-exporter/internal/config"
	"github.com/d0ugal/git-exporter/internal/metrics"
	"github.com/d0ugal/git-exporter/internal/version"
	"github.com/d0ugal/promexporter/app"
	"github.com/d0ugal/promexporter/logging"
	promexporter_metrics "github.com/d0ugal/promexporter/metrics"
)

func main() {
	// Parse command line flags
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information")

	var (
		configPath    string
		configFromEnv bool
	)

	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.BoolVar(&configFromEnv, "config-from-env", false, "Deprecated: env vars are always applied; this flag is a no-op")
	flag.Parse()

	// Show version if requested
	if showVersion {
		fmt.Printf("git-exporter %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.Commit)
		fmt.Printf("Build Date: %s\n", version.BuildDate)
		os.Exit(0)
	}

	if configPath == "config.yaml" {
		if envConfig := os.Getenv("CONFIG_PATH"); envConfig != "" {
			configPath = envConfig
		}
	}

	if configFromEnv {
		fmt.Fprintln(os.Stderr, "Warning: --config-from-env is deprecated and has no effect. Env vars are always applied on top of yaml config.")
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Configure logging using promexporter
	logging.Configure(&logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	// Initialize metrics registry using promexporter
	metricsRegistry := promexporter_metrics.NewRegistry("git_exporter_info")

	// Add custom metrics to the registry
	gitRegistry := metrics.NewGitRegistry(metricsRegistry)

	// Create and run application using promexporter
	application := app.New("Git Exporter").
		WithConfig(&cfg.BaseConfig).
		WithMetrics(metricsRegistry).
		WithVersionInfo(version.Version, version.Commit, version.BuildDate).
		Build()

	// Create collector with app reference for tracing
	gitCollector := collectors.NewGitCollector(cfg, gitRegistry, application)
	application.WithCollector(gitCollector)

	if err := application.Run(); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

