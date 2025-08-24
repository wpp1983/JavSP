package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	inputDir    string
	logLevel    string
	interactive bool
	dryRun      bool
)

// ProcessingFunc is the type for the main processing function
type ProcessingFunc func(*Config) error

// Global processing function that will be set by main
var RunJavSPProcessing ProcessingFunc

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "javsp",
	Short: "JavSP Go - High-performance AV metadata scraper",
	Long: `JavSP Go is a high-performance AV metadata scraper written in Go.
It extracts movie IDs from filenames, scrapes metadata from multiple sites,
and organizes files with NFO generation for media servers like Emby, Jellyfin, and Kodi.

Features:
  • Auto movie ID recognition
  • Multi-site data scraping
  • High-resolution cover download
  • AI-powered cover cropping
  • NFO file generation
  • File organization and renaming`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runJavSP(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yml)")
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level (DEBUG, INFO, WARNING, ERROR)")
	
	// Command-specific flags
	RootCmd.Flags().StringVarP(&inputDir, "input", "i", "", "input directory to scan")
	RootCmd.Flags().BoolVarP(&interactive, "interactive", "I", true, "enable interactive mode")
	RootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "perform a trial run with no changes made")

	// Bind flags to viper
	viper.BindPFlag("scanner.input_directory", RootCmd.Flags().Lookup("input"))
	viper.BindPFlag("other.interactive", RootCmd.Flags().Lookup("interactive"))
	viper.BindPFlag("other.log_level", RootCmd.PersistentFlags().Lookup("log-level"))

	// Version command
	RootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number of JavSP Go",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("JavSP Go %s\n", getVersion())
		},
	})

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}

	// Config show subcommand
	configCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			
			fmt.Println("Current Configuration:")
			fmt.Printf("Scanner Input Directory: %s\n", config.Scanner.InputDirectory)
			fmt.Printf("Network Proxy: %v\n", config.Network.ProxyServer)
			fmt.Printf("Crawler Selection: %v\n", config.Crawler.Selection.Normal)
			fmt.Printf("Log Level: %s\n", config.Other.LogLevel)
			return nil
		},
	})

	// Config validate subcommand
	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := Load()
			if err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}
			fmt.Println("✓ Configuration is valid")
			return nil
		},
	})

	// Config generate subcommand
	configCmd.AddCommand(&cobra.Command{
		Use:   "generate [output-file]",
		Short: "Generate default configuration file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputFile := "config.yml"
			if len(args) > 0 {
				outputFile = args[0]
			}
			
			config := GetDefaultConfig()
			if err := SaveConfig(config, outputFile); err != nil {
				return fmt.Errorf("failed to generate config file: %w", err)
			}
			
			fmt.Printf("✓ Generated default configuration: %s\n", outputFile)
			return nil
		},
	})

	RootCmd.AddCommand(configCmd)
}

// initConfig reads in config file and ENV variables.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name "config" (without extension).
		viper.AddConfigPath(".")
		viper.AddConfigPath(home + "/.javsp")
		viper.AddConfigPath("/etc/javsp/")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}

// runJavSP is the main execution function
func runJavSP(cmd *cobra.Command, args []string) error {
	config, err := LoadWithOverrides(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if dryRun {
		fmt.Println("=== DRY RUN MODE ===")
		fmt.Printf("Input directory: %s\n", config.Scanner.InputDirectory)
		fmt.Printf("Crawler sites: %v\n", config.Crawler.Selection.Normal)
		fmt.Printf("Move files: %v\n", config.Summarizer.MoveFiles)
		fmt.Println()
	}

	// Here we would start the main processing logic
	fmt.Println("Starting JavSP Go processing...")
	
	// Validate input directory exists before proceeding
	if config.Scanner.InputDirectory == "" {
		return fmt.Errorf("ERROR: Input directory is not configured. Please set scanner.input_directory in config.yml or use --input flag")
	}
	
	if _, err := os.Stat(config.Scanner.InputDirectory); os.IsNotExist(err) {
		return fmt.Errorf("ERROR: Input directory does not exist: %s", config.Scanner.InputDirectory)
	}
	
	fmt.Printf("Scanning directory: %s\n", config.Scanner.InputDirectory)
	
	// Call RunJavSPProcessing function which will be implemented in main.go
	return RunJavSPProcessing(config)
}

// LoadWithOverrides loads configuration with command-line overrides
func LoadWithOverrides(cmd *cobra.Command) (*Config, error) {
	var config *Config
	var err error

	if cfgFile != "" {
		config, err = LoadFromFile(cfgFile)
	} else {
		config, err = Load()
	}

	if err != nil {
		return nil, err
	}

	// Apply command-line overrides
	if flag := cmd.Flags().Lookup("input"); flag != nil && flag.Changed {
		config.Scanner.InputDirectory = inputDir
	}

	if flag := cmd.PersistentFlags().Lookup("log-level"); flag != nil && flag.Changed {
		config.Other.LogLevel = logLevel
	}

	if flag := cmd.Flags().Lookup("interactive"); flag != nil && flag.Changed {
		config.Other.Interactive = interactive
	}

	// Set dry-run flag in config
	config.Other.DryRun = dryRun

	// Re-validate after overrides
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed after applying overrides: %w", err)
	}

	return config, nil
}

// getVersion returns version information
func getVersion() string {
	// These will be set by build flags
	version := "dev"
	commit := "unknown"
	date := "unknown"
	
	return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}