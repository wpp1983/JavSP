# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JavSP is an AV metadata scraper project. The current active codebase is the **javsp-go** directory, which is a complete Go rewrite of the original Python version. **All other parts of the project are deprecated and should be ignored.**

## Key Architecture Components

The Go application follows a clean architecture pattern with clear separation of concerns:

### Core Packages
- **cmd/javsp/**: Main application entry point with CLI setup
- **internal/config/**: Configuration management with YAML support and CLI integration using Cobra/Viper
- **internal/crawler/**: Web scraping engine with pluggable crawler interface
  - Supports multiple data sources (javbus2, avwiki)
  - Includes registry pattern for crawler management
  - Built-in statistics and health monitoring
- **internal/datatype/**: Core data structures (MovieInfo, etc.)
- **internal/avid/**: Movie ID pattern recognition and validation
- **internal/scanner/**: File system scanning and filtering
- **internal/organizer/**: File organization and NFO generation
- **pkg/web/**: HTTP client, browser automation (Chrome DevTools Protocol), HTML parsing

### Data Flow
1. Scanner identifies video files and extracts potential movie IDs
2. Crawler engines fetch metadata from multiple sources
3. Merger combines data from different sources  
4. Organizer generates NFO files and reorganizes files according to patterns

## Development Commands

### Building
```bash
# Build for current platform
make build

# Build for all platforms 
make build-all

# Use build script for cross-platform builds
./scripts/build.sh [version]
```

### Testing
The project uses a comprehensive test strategy with build tags:

```bash
# Run all tests (unit + integration)
make test

# Run specific test types
make test-unit          # Unit tests only
make test-integration   # Integration tests only  
make test-benchmark     # Benchmark tests only
make test-all          # All tests including benchmarks

# Coverage reports
make coverage          # Unit test coverage
make coverage-full     # Full coverage report

# Quick tests (no race detection)
make test-quick

# Advanced test script with options
./scripts/test.sh --help
```

### Code Quality
```bash
# Lint code
make lint

# Format code  
make format

# Development workflow (format + lint + test + build)
make dev

# Setup development environment
make setup
```

### Configuration
- Main config file: `javsp-go/config.yml`
- Test config: `javsp-go/test-config.yml`
- Configuration uses YAML format with comprehensive options for scanning, crawling, and file organization

### Dependencies
- Uses Go 1.25.0+
- Key dependencies: Cobra (CLI), Viper (config), chromedp (browser automation), goquery (HTML parsing)
- Run `make deps` to install/update dependencies

## Working Directory
Always work within the `javsp-go/` directory - this is the active codebase. The `废弃/` directory contains deprecated Python code that should not be modified.