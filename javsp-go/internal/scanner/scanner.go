package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"javsp-go/internal/avid"
	"javsp-go/internal/config"
	"javsp-go/internal/datatype"
)

// Scanner handles file system scanning for video files
type Scanner struct {
	config     *config.Config
	recognizer *avid.Recognizer
	filters    *FileFilter
}

// FileFilter contains compiled filter patterns
type FileFilter struct {
	extensionSet      map[string]bool
	ignoredDirPattern *regexp.Regexp
	minimumSizeBytes  int64
}

// ScanResult represents the result of a file scan
type ScanResult struct {
	Movies       []*datatype.Movie `json:"movies"`
	TotalFiles   int               `json:"total_files"`
	ValidMovies  int               `json:"valid_movies"`
	SkippedFiles int               `json:"skipped_files"`
	Errors       []ScanError       `json:"errors,omitempty"`
}

// ScanError represents an error encountered during scanning
type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// NewScanner creates a new file scanner
func NewScanner(cfg *config.Config) (*Scanner, error) {
	filter, err := NewFileFilter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create file filter: %w", err)
	}

	return &Scanner{
		config:     cfg,
		recognizer: avid.NewRecognizerWithConfig(cfg),
		filters:    filter,
	}, nil
}

// NewFileFilter creates a new file filter from config
func NewFileFilter(cfg *config.Config) (*FileFilter, error) {
	filter := &FileFilter{
		extensionSet: make(map[string]bool),
	}

	// Build extension set (case insensitive)
	for _, ext := range cfg.Scanner.FilenameExtensions {
		filter.extensionSet[strings.ToLower(ext)] = true
	}

	// Compile ignored folder pattern
	if len(cfg.Scanner.IgnoredFolderNamePattern) > 0 {
		pattern := "(" + strings.Join(cfg.Scanner.IgnoredFolderNamePattern, "|") + ")"
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile ignored folder pattern: %w", err)
		}
		filter.ignoredDirPattern = compiled
	}

	// Parse minimum size
	sizeBytes, err := parseSize(cfg.Scanner.MinimumSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum size: %w", err)
	}
	filter.minimumSizeBytes = sizeBytes

	return filter, nil
}

// Scan scans the input directory for video files
func (s *Scanner) Scan() (*ScanResult, error) {
	inputDir := s.config.Scanner.InputDirectory
	if inputDir == "" {
		return nil, fmt.Errorf("input directory not specified")
	}

	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("input directory does not exist: %s", inputDir)
	}

	result := &ScanResult{
		Movies: make([]*datatype.Movie, 0),
	}

	// Walk directory tree
	err := filepath.WalkDir(inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: fmt.Sprintf("walk error: %v", err),
			})
			return nil // Continue walking
		}

		// Skip directories and check if directory should be ignored
		if d.IsDir() {
			if s.shouldIgnoreDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Process file
		if movie, skip := s.processFile(path, d); movie != nil {
			result.Movies = append(result.Movies, movie)
			result.ValidMovies++
		} else if skip {
			result.SkippedFiles++
		}

		result.TotalFiles++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return result, nil
}

// ScanSingle scans a single file
func (s *Scanner) ScanSingle(filePath string) (*datatype.Movie, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	dirEntry := fs.FileInfoToDirEntry(info)
	movie, _ := s.processFile(filePath, dirEntry)
	return movie, nil
}

// processFile processes a single file and returns a Movie if valid
func (s *Scanner) processFile(filePath string, d fs.DirEntry) (*datatype.Movie, bool) {
	// Check if file should be skipped
	if !s.shouldProcessFile(filePath, d) {
		return nil, true // skipped
	}

	// Check if directory contains NFO and should be skipped
	if s.config.Scanner.SkipNfoDir && s.hasNfoFile(filepath.Dir(filePath)) {
		return nil, true // skipped
	}

	// Get file info
	info, err := d.Info()
	if err != nil {
		return nil, false // error
	}

	// Check file size
	if info.Size() < s.filters.minimumSizeBytes {
		return nil, true // skipped
	}

	// Create movie object
	movie := datatype.NewMovie(filePath)
	movie.FileSize = info.Size()

	// Recognize movie ID
	movieID := s.recognizer.Recognize(filepath.Base(filePath))
	if movieID == "" {
		return nil, true // skipped - no valid ID found
	}

	movie.DVDID = movieID
	movie.CID = avid.GetCID(movieID)

	return movie, false
}

// shouldProcessFile checks if a file should be processed
func (s *Scanner) shouldProcessFile(filePath string, d fs.DirEntry) bool {
	// Check if it's a regular file
	if !d.Type().IsRegular() {
		return false
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	return s.filters.extensionSet[ext]
}

// shouldIgnoreDir checks if a directory should be ignored
func (s *Scanner) shouldIgnoreDir(dirName string) bool {
	if s.filters.ignoredDirPattern == nil {
		return false
	}
	return s.filters.ignoredDirPattern.MatchString(dirName)
}

// hasNfoFile checks if a directory contains NFO files
func (s *Scanner) hasNfoFile(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".nfo" {
				return true
			}
		}
	}
	return false
}

// parseSize parses size strings like "232MiB", "1GB", etc.
func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}

	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	
	// Define multipliers
	multipliers := map[string]int64{
		"B":   1,
		"KB":  1000,
		"MB":  1000 * 1000,
		"GB":  1000 * 1000 * 1000,
		"TB":  1000 * 1000 * 1000 * 1000,
		"KIB": 1024,
		"MIB": 1024 * 1024,
		"GIB": 1024 * 1024 * 1024,
		"TIB": 1024 * 1024 * 1024 * 1024,
	}

	// Try each multiplier
	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(sizeStr, suffix) {
			numberStr := strings.TrimSuffix(sizeStr, suffix)
			if number, err := strconv.ParseFloat(numberStr, 64); err == nil {
				return int64(number * float64(multiplier)), nil
			}
		}
	}

	// Try parsing as plain number (assume bytes)
	if number, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
		return number, nil
	}

	return 0, fmt.Errorf("invalid size format: %s", sizeStr)
}

// GetSupportedExtensions returns the list of supported file extensions
func (s *Scanner) GetSupportedExtensions() []string {
	extensions := make([]string, 0, len(s.filters.extensionSet))
	for ext := range s.filters.extensionSet {
		extensions = append(extensions, ext)
	}
	return extensions
}

// GetStats returns scanning statistics
func (sr *ScanResult) GetStats() map[string]interface{} {
	successRate := 0.0
	if sr.TotalFiles > 0 {
		successRate = float64(sr.ValidMovies) / float64(sr.TotalFiles) * 100
	}

	return map[string]interface{}{
		"total_files":    sr.TotalFiles,
		"valid_movies":   sr.ValidMovies,
		"skipped_files":  sr.SkippedFiles,
		"errors":         len(sr.Errors),
		"success_rate":   fmt.Sprintf("%.1f%%", successRate),
	}
}