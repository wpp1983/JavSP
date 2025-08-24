//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"javsp-go/internal/config"
	"javsp-go/test/testutils"
)

func TestConfigIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := testutils.CreateTempDir(t)

	t.Run("TestFullConfigWorkflow", func(t *testing.T) {
		// Create comprehensive config
		configContent := `
scanner:
  input_directory: "` + tmpDir + `"
  minimum_size: "232MiB"
  filename_extensions: [".mp4", ".mkv", ".avi", ".wmv"]
  ignored_id_pattern: ["TEST"]

network:
  retry: 5
  timeout: "45s"
  proxy_server: "http://proxy.example.com:8080"

crawler:
  hardworking: true
  sleep_after_scraping: "2s"
  required_keys: ["title", "actress", "genre", "cover"]
  selection:
    normal: ["javbus", "avwiki"]
    amateur: ["javbus"]
    fc2: ["javbus"]

summarizer:
  move_files: true
  path:
    length_maximum: 200
    max_actress_count: 3
    actress_name_delimiter: " "
  censor: ["无码", "有码", "骑兵"]

other:
  log_level: "INFO"
  interactive: false
  debug: true
`

		configPath := filepath.Join(tmpDir, "full_config.yml")
		testutils.WriteTestFile(t, configPath, configContent)

		// Test loading
		cfg, err := config.LoadFromFile(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Test scanner configuration
		if cfg.Scanner.InputDirectory != tmpDir {
			t.Errorf("Expected input directory %s, got %s", tmpDir, cfg.Scanner.InputDirectory)
		}

		if cfg.Scanner.MinimumSize != "232MiB" {
			t.Errorf("Expected minimum size '232MiB', got '%s'", cfg.Scanner.MinimumSize)
		}

		expectedExtensions := []string{".mp4", ".mkv", ".avi", ".wmv"}
		if len(cfg.Scanner.FilenameExtensions) != len(expectedExtensions) {
			t.Errorf("Expected %d extensions, got %d", len(expectedExtensions), len(cfg.Scanner.FilenameExtensions))
		}

		// Test network configuration
		if cfg.Network.Retry != 5 {
			t.Errorf("Expected retry 5, got %d", cfg.Network.Retry)
		}

		if cfg.Network.Timeout != 45*time.Second {
			t.Errorf("Expected timeout 45s, got %v", cfg.Network.Timeout)
		}

		if cfg.Network.ProxyServer == nil || *cfg.Network.ProxyServer != "http://proxy.example.com:8080" {
			t.Errorf("Expected proxy server 'http://proxy.example.com:8080', got %v", cfg.Network.ProxyServer)
		}

		// Test crawler configuration
		if !cfg.Crawler.Hardworking {
			t.Error("Expected hardworking to be true")
		}

		if cfg.Crawler.SleepAfterScraping != 2*time.Second {
			t.Errorf("Expected sleep after scraping 2s, got %v", cfg.Crawler.SleepAfterScraping)
		}

		expectedKeys := []string{"title", "actress", "genre", "cover"}
		if len(cfg.Crawler.RequiredKeys) != len(expectedKeys) {
			t.Errorf("Expected %d required keys, got %d", len(expectedKeys), len(cfg.Crawler.RequiredKeys))
		}

		// Test summarizer configuration
		if !cfg.Summarizer.MoveFiles {
			t.Error("Expected move_files to be true")
		}

		if cfg.Summarizer.Path.LengthMaximum != 200 {
			t.Errorf("Expected length maximum 200, got %d", cfg.Summarizer.Path.LengthMaximum)
		}

		if cfg.Summarizer.Path.MaxActressCount != 3 {
			t.Errorf("Expected max actress count 3, got %d", cfg.Summarizer.Path.MaxActressCount)
		}

		// Test other configuration
		if cfg.Other.LogLevel != "INFO" {
			t.Errorf("Expected log level 'INFO', got '%s'", cfg.Other.LogLevel)
		}

		if cfg.Other.Interactive {
			t.Error("Expected interactive to be false")
		}
	})

	t.Run("TestConfigSaveAndLoad", func(t *testing.T) {
		// Create original config
		originalConfig := config.GetDefaultConfig()
		originalConfig.Scanner.InputDirectory = tmpDir
		originalConfig.Network.Retry = 7
		originalConfig.Other.LogLevel = "DEBUG"

		configPath := filepath.Join(tmpDir, "save_load_test.yml")

		// Save config
		if err := config.SaveConfig(originalConfig, configPath); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Verify file exists
		testutils.AssertFileExists(t, configPath)

		// Load config
		config.ResetConfig()
		loadedConfig, err := config.LoadFromFile(configPath)
		if err != nil {
			t.Fatalf("Failed to load saved config: %v", err)
		}

		// Compare key values
		if loadedConfig.Scanner.InputDirectory != originalConfig.Scanner.InputDirectory {
			t.Errorf("Input directory mismatch: expected %s, got %s",
				originalConfig.Scanner.InputDirectory, loadedConfig.Scanner.InputDirectory)
		}

		if loadedConfig.Network.Retry != originalConfig.Network.Retry {
			t.Errorf("Retry count mismatch: expected %d, got %d",
				originalConfig.Network.Retry, loadedConfig.Network.Retry)
		}

		if loadedConfig.Other.LogLevel != originalConfig.Other.LogLevel {
			t.Errorf("Log level mismatch: expected %s, got %s",
				originalConfig.Other.LogLevel, loadedConfig.Other.LogLevel)
		}
	})

	t.Run("TestEnvironmentVariableSubstitution", func(t *testing.T) {
		// Set environment variable
		testValue := "/test/env/path"
		os.Setenv("JAVSP_TEST_PATH", testValue)
		defer os.Unsetenv("JAVSP_TEST_PATH")

		configContent := `
scanner:
  input_directory: "${JAVSP_TEST_PATH}/videos"
  
network:
  timeout: "${JAVSP_TEST_TIMEOUT:-30s}"
`

		configPath := filepath.Join(tmpDir, "env_test.yml")
		testutils.WriteTestFile(t, configPath, configContent)

		cfg, err := config.LoadFromFile(configPath)
		if err != nil {
			t.Fatalf("Failed to load config with env vars: %v", err)
		}

		expectedPath := testValue + "/videos"
		if cfg.Scanner.InputDirectory != expectedPath {
			t.Errorf("Expected input directory %s, got %s", expectedPath, cfg.Scanner.InputDirectory)
		}

		// Test default value
		if cfg.Network.Timeout != 30*time.Second {
			t.Errorf("Expected timeout 30s, got %v", cfg.Network.Timeout)
		}
	})

	t.Run("TestConfigValidationErrors", func(t *testing.T) {
		testCases := []struct {
			name        string
			config      string
			expectError bool
		}{
			{
				name: "negative retry",
				config: `
network:
  retry: -1
`,
				expectError: true,
			},
			{
				name: "zero timeout",
				config: `
network:
  timeout: "0s"
`,
				expectError: true,
			},
			{
				name: "empty extensions",
				config: `
scanner:
  filename_extensions: []
`,
				expectError: true,
			},
			{
				name: "invalid censor count",
				config: `
summarizer:
  censor: ["only_one"]
`,
				expectError: true,
			},
			{
				name: "valid minimal config",
				config: `
scanner:
  filename_extensions: [".mp4"]
network:
  retry: 1
  timeout: "10s"
crawler:
  required_keys: ["title"]
  selection:
    normal: ["javbus"]
`,
				expectError: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				configPath := filepath.Join(tmpDir, tc.name+".yml")
				testutils.WriteTestFile(t, configPath, tc.config)

				_, err := config.LoadFromFile(configPath)
				
				if tc.expectError && err == nil {
					t.Error("Expected validation error, but got none")
				}
				
				if !tc.expectError && err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			})
		}
	})

	t.Run("TestDurationParsing", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected time.Duration
		}{
			{"30s", 30 * time.Second},
			{"2m", 2 * time.Minute},
			{"1h", 1 * time.Hour},
			{"PT15S", 15 * time.Second},
			{"PT1M30S", 90 * time.Second},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result, err := config.ParseDuration(tc.input)
				if err != nil {
					t.Fatalf("Failed to parse duration %s: %v", tc.input, err)
				}

				if result != tc.expected {
					t.Errorf("Expected duration %v, got %v", tc.expected, result)
				}
			})
		}
	})

	t.Run("TestConfigMerging", func(t *testing.T) {
		// Create base config
		baseConfig := `
scanner:
  minimum_size: "100MiB"
  filename_extensions: [".mp4"]

network:
  retry: 3
  timeout: "30s"
`

		// Create override config
		overrideConfig := `
scanner:
  minimum_size: "500MiB"

network:
  retry: 5

other:
  log_level: "DEBUG"
`

		baseConfigPath := filepath.Join(tmpDir, "base.yml")
		overrideConfigPath := filepath.Join(tmpDir, "override.yml")

		testutils.WriteTestFile(t, baseConfigPath, baseConfig)
		testutils.WriteTestFile(t, overrideConfigPath, overrideConfig)

		// Load base config
		config.ResetConfig()
		baseCfg, err := config.LoadFromFile(baseConfigPath)
		if err != nil {
			t.Fatalf("Failed to load base config: %v", err)
		}

		// Verify base values
		if baseCfg.Scanner.MinimumSize != "100MiB" {
			t.Errorf("Expected base minimum size '100MiB', got '%s'", baseCfg.Scanner.MinimumSize)
		}

		if baseCfg.Network.Retry != 3 {
			t.Errorf("Expected base retry 3, got %d", baseCfg.Network.Retry)
		}

		// Load override config
		config.ResetConfig()
		overrideCfg, err := config.LoadFromFile(overrideConfigPath)
		if err != nil {
			t.Fatalf("Failed to load override config: %v", err)
		}

		// Should have overridden values and defaults
		if overrideCfg.Scanner.MinimumSize != "500MiB" {
			t.Errorf("Expected override minimum size '500MiB', got '%s'", overrideCfg.Scanner.MinimumSize)
		}

		if overrideCfg.Network.Retry != 5 {
			t.Errorf("Expected override retry 5, got %d", overrideCfg.Network.Retry)
		}

		if overrideCfg.Other.LogLevel != "DEBUG" {
			t.Errorf("Expected override log level 'DEBUG', got '%s'", overrideCfg.Other.LogLevel)
		}
	})
}

func TestConfigPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := testutils.CreateTempDir(t)

	// Create a large config file
	configContent := `
scanner:
  input_directory: "` + tmpDir + `"
  minimum_size: "232MiB"
  filename_extensions: [".mp4", ".mkv", ".avi", ".wmv", ".flv", ".m4v", ".mov", ".mpg", ".mpeg"]
  ignored_id_pattern: ["TEST", "TEMP", "SAMPLE"]

network:
  retry: 3
  timeout: "30s"

crawler:
  hardworking: true
  required_keys: ["title", "actress", "genre", "cover", "release_date", "runtime"]
  selection:
    normal: ["javbus", "avwiki", "javlib", "dmm"]
    amateur: ["javbus", "fc2hub"]
    fc2: ["javbus", "fc2hub"]

summarizer:
  move_files: true
  path:
    length_maximum: 250
    max_actress_count: 5
    actress_name_delimiter: " "
  censor: ["无码", "有码", "骑兵"]

other:
  log_level: "INFO"
  interactive: false
`

	configPath := filepath.Join(tmpDir, "large_config.yml")
	testutils.WriteTestFile(t, configPath, configContent)

	t.Run("TestConfigLoadPerformance", func(t *testing.T) {
		iterations := 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			config.ResetConfig()
			_, err := config.LoadFromFile(configPath)
			if err != nil {
				t.Fatalf("Failed to load config on iteration %d: %v", i, err)
			}
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)

		t.Logf("Loaded config %d times in %v (avg: %v per load)", iterations, duration, avgDuration)

		if avgDuration > 10*time.Millisecond {
			t.Errorf("Config loading too slow: %v per load", avgDuration)
		}
	})

	t.Run("TestConfigValidationPerformance", func(t *testing.T) {
		config.ResetConfig()
		cfg, err := config.LoadFromFile(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		iterations := 1000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			if err := config.ValidateConfig(cfg); err != nil {
				t.Fatalf("Validation failed on iteration %d: %v", i, err)
			}
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)

		t.Logf("Validated config %d times in %v (avg: %v per validation)", iterations, duration, avgDuration)

		if avgDuration > 1*time.Millisecond {
			t.Errorf("Config validation too slow: %v per validation", avgDuration)
		}
	})
}

func BenchmarkConfigLoad(b *testing.B) {
	tmpDir := testutils.CreateTempDir(&testing.T{})

	configContent := `
scanner:
  input_directory: "` + tmpDir + `"
  minimum_size: "232MiB"
  filename_extensions: [".mp4", ".mkv", ".avi"]

network:
  retry: 3
  timeout: "30s"

crawler:
  hardworking: true
  selection:
    normal: ["javbus", "avwiki"]

other:
  log_level: "INFO"
`

	configPath := filepath.Join(tmpDir, "benchmark_config.yml")
	testutils.WriteTestFile(&testing.T{}, configPath, configContent)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.ResetConfig()
		_, err := config.LoadFromFile(configPath)
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}