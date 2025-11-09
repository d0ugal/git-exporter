package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	promexporter_config "github.com/d0ugal/promexporter/config"
	"github.com/go-git/go-git/v5"
	"gopkg.in/yaml.v3"
)

// Duration uses promexporter Duration type
type Duration = promexporter_config.Duration

type Config struct {
	promexporter_config.BaseConfig

	Git GitConfig `yaml:"git"`
}

type GitConfig struct {
	Repositories []RepositoryConfig `yaml:"repositories"`
	Discover     []string           `yaml:"discover"` // Directories to scan for Git repositories
}

type RepositoryConfig struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// LoadConfig loads configuration from either a YAML file or environment variables
func LoadConfig(path string, configFromEnv bool) (*Config, error) {
	if configFromEnv {
		return loadFromEnv()
	}

	return Load(path)
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv() (*Config, error) {
	config := &Config{}

	// Load base configuration from environment
	baseConfig := &promexporter_config.BaseConfig{}

	// Server configuration
	if host := os.Getenv("GIT_EXPORTER_SERVER_HOST"); host != "" {
		baseConfig.Server.Host = host
	} else {
		baseConfig.Server.Host = "0.0.0.0"
	}

	if portStr := os.Getenv("GIT_EXPORTER_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err != nil {
			return nil, fmt.Errorf("invalid server port: %w", err)
		} else {
			baseConfig.Server.Port = port
		}
	} else {
		baseConfig.Server.Port = 8080
	}

	// Logging configuration
	if level := os.Getenv("GIT_EXPORTER_LOG_LEVEL"); level != "" {
		baseConfig.Logging.Level = level
	} else {
		baseConfig.Logging.Level = "info"
	}

	if format := os.Getenv("GIT_EXPORTER_LOG_FORMAT"); format != "" {
		baseConfig.Logging.Format = format
	} else {
		baseConfig.Logging.Format = "json"
	}

	// Metrics configuration
	if intervalStr := os.Getenv("GIT_EXPORTER_METRICS_DEFAULT_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err != nil {
			return nil, fmt.Errorf("invalid metrics default interval: %w", err)
		} else {
			baseConfig.Metrics.Collection.DefaultInterval = promexporter_config.Duration{Duration: interval}
			baseConfig.Metrics.Collection.DefaultIntervalSet = true
		}
	} else {
		baseConfig.Metrics.Collection.DefaultInterval = promexporter_config.Duration{Duration: time.Second * 30}
	}

	config.BaseConfig = *baseConfig

	// Apply generic environment variables (TRACING_ENABLED, PROFILING_ENABLED, etc.)
	if err := promexporter_config.ApplyGenericEnvVars(&config.BaseConfig); err != nil {
		return nil, fmt.Errorf("failed to apply generic environment variables: %w", err)
	}

	// Git configuration - for now, we'll require a config file for repositories
	// Environment variable support can be added later if needed

	// Set defaults for any missing values
	setDefaults(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}

	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}

	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	if !config.Metrics.Collection.DefaultIntervalSet {
		config.Metrics.Collection.DefaultInterval = promexporter_config.Duration{Duration: time.Second * 30}
	}
}

// Validate performs comprehensive validation of the configuration
func (c *Config) Validate() error {
	// Validate server configuration
	if err := c.validateServerConfig(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	// Validate logging configuration
	if err := c.validateLoggingConfig(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	// Validate metrics configuration
	if err := c.validateMetricsConfig(); err != nil {
		return fmt.Errorf("metrics config: %w", err)
	}

	// Validate git configuration
	if err := c.validateGitConfig(); err != nil {
		return fmt.Errorf("git config: %w", err)
	}

	return nil
}

func (c *Config) validateServerConfig() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Server.Port)
	}

	return nil
}

func (c *Config) validateLoggingConfig() error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid logging format: %s", c.Logging.Format)
	}

	return nil
}

func (c *Config) validateMetricsConfig() error {
	if c.Metrics.Collection.DefaultInterval.Seconds() < 1 {
		return fmt.Errorf("default interval must be at least 1 second, got %d", c.Metrics.Collection.DefaultInterval.Seconds())
	}

	if c.Metrics.Collection.DefaultInterval.Seconds() > 86400 {
		return fmt.Errorf("default interval must be at most 86400 seconds (24 hours), got %d", c.Metrics.Collection.DefaultInterval.Seconds())
	}

	return nil
}

func (c *Config) validateGitConfig() error {
	// At least one repository or discovery pattern must be configured
	if len(c.Git.Repositories) == 0 && len(c.Git.Discover) == 0 {
		return fmt.Errorf("at least one git repository or discovery pattern must be configured")
	}

	// Validate explicit repositories
	for i, repo := range c.Git.Repositories {
		if repo.Name == "" {
			return fmt.Errorf("repository[%d]: name is required", i)
		}
		if repo.Path == "" {
			return fmt.Errorf("repository[%d]: path is required", i)
		}
	}

	// Validate discovery paths
	for i, path := range c.Git.Discover {
		if path == "" {
			return fmt.Errorf("discover[%d]: path is required", i)
		}
	}

	return nil
}

// GetDefaultInterval returns the default collection interval
func (c *Config) GetDefaultInterval() int {
	return int(c.Metrics.Collection.DefaultInterval.Seconds())
}

// ExpandRepositories expands discovery patterns into actual repository configurations
// and returns a combined list of explicit repositories and discovered ones
func (c *Config) ExpandRepositories() ([]RepositoryConfig, error) {
	var allRepos []RepositoryConfig

	// Add explicit repositories
	allRepos = append(allRepos, c.Git.Repositories...)

	// Discover repositories from patterns
	for _, discoverPath := range c.Git.Discover {
		repos, err := discoverRepositories(discoverPath)
		if err != nil {
			return nil, fmt.Errorf("failed to discover repositories in %s: %w", discoverPath, err)
		}
		allRepos = append(allRepos, repos...)
	}

	return allRepos, nil
}

// discoverRepositories scans a directory for Git repositories
// It looks for directories that contain a .git subdirectory or are bare repositories
func discoverRepositories(rootPath string) ([]RepositoryConfig, error) {
	var repos []RepositoryConfig

	// Check if the discovery path exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		// Path doesn't exist - return empty list (not an error, just log a warning)
		return repos, nil
	}

	// Check if the root path itself is a Git repository
	if isGitRepository(rootPath) {
		repoName := filepath.Base(rootPath)
		repos = append(repos, RepositoryConfig{
			Name: repoName,
			Path: rootPath,
		})
		// If root is a Git repo, don't walk into it
		return repos, nil
	}

	// Walk the directory tree looking for Git repositories
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			return nil
		}

		// Skip the root path itself (already checked)
		if path == rootPath {
			return nil
		}

		// Only check directories
		if !info.IsDir() {
			return nil
		}

		// Check if this directory is a Git repository
		if isGitRepository(path) {
			// Get relative path from root for the repository name
			relPath, err := filepath.Rel(rootPath, path)
			if err != nil {
				// If we can't get relative path, use the base name
				relPath = filepath.Base(path)
			}
			// Use forward slashes for cleaner names
			repoName := filepath.ToSlash(relPath)
			repos = append(repos, RepositoryConfig{
				Name: repoName,
				Path: path,
			})
			// Skip walking into this directory's subdirectories
			return filepath.SkipDir
		}

		return nil
	})

	return repos, err
}

// isGitRepository checks if a path is a Git repository
// It checks for both regular repositories (.git directory) and bare repositories (.git file or bare repo structure)
func isGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	gitDirInfo, err := os.Stat(gitDir)
	if err != nil {
		return false
	}

	// Check if .git is a directory (regular repository) or a file (worktree)
	if gitDirInfo.IsDir() {
		// Check for bare repository indicators
		headFile := filepath.Join(gitDir, "HEAD")
		configFile := filepath.Join(gitDir, "config")
		if _, err := os.Stat(headFile); err == nil {
			if _, err := os.Stat(configFile); err == nil {
				// Try to open with go-git to verify it's a valid repository
				_, err := git.PlainOpen(path)
				return err == nil
			}
		}
	} else {
		// .git is a file (worktree), check if it points to a valid git dir
		// For simplicity, if .git exists as a file, we consider it a repository
		// Try to open with go-git to verify it's a valid repository
		_, err := git.PlainOpen(path)
		return err == nil
	}

	return false
}

