package scanner

import (
	"path/filepath"
	"regexp"
	"strings"
)

// AdvancedFilter provides advanced filtering capabilities
type AdvancedFilter struct {
	// Size filters
	MinSize int64
	MaxSize int64

	// Name filters
	IncludePatterns []*regexp.Regexp
	ExcludePatterns []*regexp.Regexp

	// Path filters
	IncludePaths []*regexp.Regexp
	ExcludePaths []*regexp.Regexp

	// Custom filter function
	CustomFilter func(string) bool
}

// NewAdvancedFilter creates a new advanced filter
func NewAdvancedFilter() *AdvancedFilter {
	return &AdvancedFilter{}
}

// AddIncludePattern adds a pattern that files must match
func (af *AdvancedFilter) AddIncludePattern(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	af.IncludePatterns = append(af.IncludePatterns, compiled)
	return nil
}

// AddExcludePattern adds a pattern that files must NOT match
func (af *AdvancedFilter) AddExcludePattern(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	af.ExcludePatterns = append(af.ExcludePatterns, compiled)
	return nil
}

// AddIncludePath adds a path pattern that files must be in
func (af *AdvancedFilter) AddIncludePath(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	af.IncludePaths = append(af.IncludePaths, compiled)
	return nil
}

// AddExcludePath adds a path pattern that files must NOT be in
func (af *AdvancedFilter) AddExcludePath(pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	af.ExcludePaths = append(af.ExcludePaths, compiled)
	return nil
}

// ShouldInclude determines if a file should be included based on all filters
func (af *AdvancedFilter) ShouldInclude(filePath string, fileSize int64) bool {
	// Size filters
	if af.MinSize > 0 && fileSize < af.MinSize {
		return false
	}
	if af.MaxSize > 0 && fileSize > af.MaxSize {
		return false
	}

	fileName := strings.ToLower(filePath)
	
	// Include patterns - file must match at least one if any are specified
	if len(af.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range af.IncludePatterns {
			if pattern.MatchString(fileName) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Exclude patterns - file must not match any
	for _, pattern := range af.ExcludePatterns {
		if pattern.MatchString(fileName) {
			return false
		}
	}

	// Include paths - file must be in at least one if any are specified
	if len(af.IncludePaths) > 0 {
		matched := false
		for _, pattern := range af.IncludePaths {
			if pattern.MatchString(filePath) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Exclude paths - file must not be in any
	for _, pattern := range af.ExcludePaths {
		if pattern.MatchString(filePath) {
			return false
		}
	}

	// Custom filter
	if af.CustomFilter != nil && !af.CustomFilter(filePath) {
		return false
	}

	return true
}

// FileTypeFilter filters files by type
type FileTypeFilter struct {
	VideoExtensions []string
	ImageExtensions []string
	SubtitleExtensions []string
}

// NewFileTypeFilter creates a new file type filter with defaults
func NewFileTypeFilter() *FileTypeFilter {
	return &FileTypeFilter{
		VideoExtensions: []string{
			".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
			".m4v", ".3gp", ".ts", ".m2ts", ".vob", ".iso", ".rm", 
			".rmvb", ".f4v", ".mpeg", ".mpg", ".strm",
		},
		ImageExtensions: []string{
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".svg",
		},
		SubtitleExtensions: []string{
			".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx", ".sup",
		},
	}
}

// IsVideoFile checks if file is a video file
func (ftf *FileTypeFilter) IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, videoExt := range ftf.VideoExtensions {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// IsImageFile checks if file is an image file
func (ftf *FileTypeFilter) IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, imgExt := range ftf.ImageExtensions {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// IsSubtitleFile checks if file is a subtitle file
func (ftf *FileTypeFilter) IsSubtitleFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, subExt := range ftf.SubtitleExtensions {
		if ext == subExt {
			return true
		}
	}
	return false
}

// CommonFilters provides commonly used filters
var CommonFilters = struct {
	// Skip sample/trailer files
	SkipSamples *regexp.Regexp
	// Skip backup/temp files  
	SkipTemp *regexp.Regexp
	// Skip system files
	SkipSystem *regexp.Regexp
}{
	SkipSamples: regexp.MustCompile(`(?i)(sample|trailer|preview|demo)[\W_]`),
	SkipTemp:    regexp.MustCompile(`(?i)(\.(tmp|temp|bak|backup)|~$|^\.)`),
	SkipSystem:  regexp.MustCompile(`(?i)(thumbs\.db|desktop\.ini|\.ds_store)`),
}