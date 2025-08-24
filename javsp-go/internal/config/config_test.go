package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	// Test scanner defaults
	if config.Scanner.InputDirectory != "" {
		t.Errorf("Expected empty input directory, got: %s", config.Scanner.InputDirectory)
	}

	if len(config.Scanner.FilenameExtensions) == 0 {
		t.Error("Expected non-empty filename extensions")
	}

	if config.Scanner.MinimumSize != "232MiB" {
		t.Errorf("Expected minimum size '232MiB', got: %s", config.Scanner.MinimumSize)
	}

	// Test network defaults
	if config.Network.Retry != 3 {
		t.Errorf("Expected retry count 3, got: %d", config.Network.Retry)
	}

	if config.Network.Timeout != 15*time.Second {
		t.Errorf("Expected timeout 15s, got: %v", config.Network.Timeout)
	}

	// Test crawler defaults
	if len(config.Crawler.Selection.Normal) == 0 {
		t.Error("Expected non-empty normal crawler selection")
	}

	if !config.Crawler.Hardworking {
		t.Error("Expected hardworking to be true by default")
	}

	// Test summarizer defaults
	if !config.Summarizer.MoveFiles {
		t.Error("Expected move_files to be true by default")
	}

	if config.Summarizer.Path.LengthMaximum != 250 {
		t.Errorf("Expected length maximum 250, got: %d", config.Summarizer.Path.LengthMaximum)
	}

	// Test other defaults
	if !config.Other.Interactive {
		t.Error("Expected interactive to be true by default")
	}

	if config.Other.LogLevel != "INFO" {
		t.Errorf("Expected log level INFO, got: %s", config.Other.LogLevel)
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.yml")

	configContent := `
scanner:
  input_directory: /test/path
  minimum_size: 100MiB

network:
  retry: 5
  timeout: 30s

other:
  log_level: DEBUG
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Reset global config to ensure clean test
	ResetConfig()

	config, err := LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	// Test that values were loaded correctly
	if config.Scanner.InputDirectory != "/test/path" {
		t.Errorf("Expected input directory '/test/path', got: %s", config.Scanner.InputDirectory)
	}

	if config.Scanner.MinimumSize != "100MiB" {
		t.Errorf("Expected minimum size '100MiB', got: %s", config.Scanner.MinimumSize)
	}

	if config.Network.Retry != 5 {
		t.Errorf("Expected retry count 5, got: %d", config.Network.Retry)
	}

	if config.Network.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got: %v", config.Network.Timeout)
	}

	if config.Other.LogLevel != "DEBUG" {
		t.Errorf("Expected log level DEBUG, got: %s", config.Other.LogLevel)
	}
}

func TestLoadFromNonExistentFile(t *testing.T) {
	_, err := LoadFromFile("/non/existent/config.yml")
	if err == nil {
		t.Error("Expected error when loading non-existent config file")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		modifyConfig func(*Config)
		expectError bool
	}{
		{
			name: "valid config",
			modifyConfig: func(c *Config) {
				// Default config should be valid
			},
			expectError: false,
		},
		{
			name: "negative retry count",
			modifyConfig: func(c *Config) {
				c.Network.Retry = -1
			},
			expectError: true,
		},
		{
			name: "zero timeout",
			modifyConfig: func(c *Config) {
				c.Network.Timeout = 0
			},
			expectError: true,
		},
		{
			name: "empty required keys",
			modifyConfig: func(c *Config) {
				c.Crawler.RequiredKeys = []string{}
			},
			expectError: true,
		},
		{
			name: "empty normal crawler selection",
			modifyConfig: func(c *Config) {
				c.Crawler.Selection.Normal = []string{}
			},
			expectError: true,
		},
		{
			name: "empty filename extensions",
			modifyConfig: func(c *Config) {
				c.Scanner.FilenameExtensions = []string{}
			},
			expectError: true,
		},
		{
			name: "zero length maximum",
			modifyConfig: func(c *Config) {
				c.Summarizer.Path.LengthMaximum = 0
			},
			expectError: true,
		},
		{
			name: "zero max actress count",
			modifyConfig: func(c *Config) {
				c.Summarizer.Path.MaxActressCount = 0
			},
			expectError: true,
		},
		{
			name: "invalid censor options count",
			modifyConfig: func(c *Config) {
				c.Summarizer.Censor = []string{"one", "two"} // Should be 3 elements
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetDefaultConfig()
			tt.modifyConfig(config)

			err := validateConfig(config)
			if tt.expectError && err == nil {
				t.Error("Expected validation error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestProcessConfig(t *testing.T) {
	config := GetDefaultConfig()
	
	// Test environment variable expansion
	testDir := "/test/directory"
	os.Setenv("TEST_DIR", testDir)
	defer os.Unsetenv("TEST_DIR")
	
	config.Scanner.InputDirectory = "${TEST_DIR}/videos"
	
	err := processConfig(config)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	expected := testDir + "/videos"
	if config.Scanner.InputDirectory != expected {
		t.Errorf("Expected input directory %s, got: %s", expected, config.Scanner.InputDirectory)
	}
}

func TestSaveConfig(t *testing.T) {
	config := GetDefaultConfig()
	config.Scanner.InputDirectory = "/test/save/path"
	config.Other.LogLevel = "DEBUG"

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "saved_config.yml")

	err := SaveConfig(config, configFile)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify saved config
	ResetConfig()
	loadedConfig, err := LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Scanner.InputDirectory != "/test/save/path" {
		t.Errorf("Expected input directory '/test/save/path', got: %s", loadedConfig.Scanner.InputDirectory)
	}

	if loadedConfig.Other.LogLevel != "DEBUG" {
		t.Errorf("Expected log level DEBUG, got: %s", loadedConfig.Other.LogLevel)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"PT15S", 15 * time.Second},
		{"PT1M", 1 * time.Minute},
		{"PT2H", 2 * time.Hour},
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse duration %s: %v", tt.input, err)
			}

			if result != tt.expected {
				t.Errorf("Expected duration %v, got: %v", tt.expected, result)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	// Reset config first
	ResetConfig()

	// Should panic when config is not loaded
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when getting config before loading")
		}
	}()

	GetConfig()
}