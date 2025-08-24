package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/sirupsen/logrus"
)

var (
	globalConfig *Config
	logger       = logrus.New()
)

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	// Initialize with defaults
	config := GetDefaultConfig()

	// Setup viper
	v := viper.New()
	
	// Set config name and paths - use .yml extension to match Python version
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	
	// Add config search paths
	v.AddConfigPath(".")                    // Current directory
	v.AddConfigPath("$HOME/.javsp")         // Home directory
	v.AddConfigPath("/etc/javsp/")          // System config directory
	
	// Set environment variable prefix
	v.SetEnvPrefix("JAVSP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Info("No config file found, using defaults")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		logger.Infof("Using config file: %s", v.ConfigFileUsed())
	}

	// Unmarshal config
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Post-process and validate
	if err := processConfig(config); err != nil {
		return nil, fmt.Errorf("error processing config: %w", err)
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	globalConfig = config
	return config, nil
}

// LoadFromFile loads configuration from a specific file
func LoadFromFile(configPath string) (*Config, error) {
	if !filepath.IsAbs(configPath) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error getting working directory: %w", err)
		}
		configPath = filepath.Join(wd, configPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", configPath)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetEnvPrefix("JAVSP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", configPath, err)
	}
	
	// Initialize with defaults, then unmarshal to override
	config := GetDefaultConfig()

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := processConfig(config); err != nil {
		return nil, fmt.Errorf("error processing config: %w", err)
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	globalConfig = config
	return config, nil
}

// processConfig handles post-processing of the configuration
func processConfig(config *Config) error {
	// Expand environment variables in paths
	if config.Scanner.InputDirectory != "" {
		config.Scanner.InputDirectory = os.ExpandEnv(config.Scanner.InputDirectory)
	}

	if config.Crawler.FC2FanLocalPath != nil && *config.Crawler.FC2FanLocalPath != "" {
		expanded := os.ExpandEnv(*config.Crawler.FC2FanLocalPath)
		config.Crawler.FC2FanLocalPath = &expanded
	}

	// Skip directory validation in loader - let CLI handle it with better error messages

	// Setup logging level
	if level, err := logrus.ParseLevel(config.Other.LogLevel); err != nil {
		logger.Warnf("Invalid log level '%s', using INFO", config.Other.LogLevel)
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(level)
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate network settings
	if config.Network.Retry < 0 {
		return fmt.Errorf("network.retry must be non-negative, got: %d", config.Network.Retry)
	}

	if config.Network.Timeout <= 0 {
		return fmt.Errorf("network.timeout must be positive, got: %v", config.Network.Timeout)
	}

	// Validate crawler settings
	if len(config.Crawler.RequiredKeys) == 0 {
		return fmt.Errorf("crawler.required_keys cannot be empty")
	}

	// Validate crawler selection
	if len(config.Crawler.Selection.Normal) == 0 {
		return fmt.Errorf("crawler.selection.normal cannot be empty")
	}

	// Validate file extensions
	if len(config.Scanner.FilenameExtensions) == 0 {
		return fmt.Errorf("scanner.filename_extensions cannot be empty")
	}

	// Validate path settings
	if config.Summarizer.Path.LengthMaximum <= 0 {
		return fmt.Errorf("summarizer.path.length_maximum must be positive, got: %d", 
			config.Summarizer.Path.LengthMaximum)
	}

	if config.Summarizer.Path.MaxActressCount <= 0 {
		return fmt.Errorf("summarizer.path.max_actress_count must be positive, got: %d", 
			config.Summarizer.Path.MaxActressCount)
	}

	// Validate censor options
	if len(config.Summarizer.Censor) != 3 {
		return fmt.Errorf("summarizer.censor_options_representation must have exactly 3 elements, got: %d", 
			len(config.Summarizer.Censor))
	}

	return nil
}

// GetConfig returns the global configuration instance
func GetConfig() *Config {
	if globalConfig == nil {
		panic("configuration not loaded. Call Load() first.")
	}
	return globalConfig
}

// ResetConfig resets the global configuration (mainly for testing)
func ResetConfig() {
	globalConfig = nil
}

// SaveConfig saves the current configuration to a file
func SaveConfig(config *Config, filePath string) error {
	v := viper.New()
	
	// Convert config to map for viper
	v.Set("scanner", config.Scanner)
	v.Set("network", config.Network)
	v.Set("crawler", config.Crawler)
	v.Set("summarizer", config.Summarizer)
	v.Set("other", config.Other)

	return v.WriteConfigAs(filePath)
}

// ParseDuration parses duration strings from config (like "PT15S")
func ParseDuration(s string) (time.Duration, error) {
	// Handle ISO 8601 duration format (PT15S)
	if strings.HasPrefix(s, "PT") {
		s = strings.TrimPrefix(s, "PT")
		if strings.HasSuffix(s, "S") {
			s = strings.TrimSuffix(s, "S") + "s"
		} else if strings.HasSuffix(s, "M") {
			s = strings.TrimSuffix(s, "M") + "m"
		} else if strings.HasSuffix(s, "H") {
			s = strings.TrimSuffix(s, "H") + "h"
		}
	}
	
	return time.ParseDuration(s)
}