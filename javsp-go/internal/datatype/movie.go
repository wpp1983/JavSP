package datatype

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Movie represents a movie file with metadata
type Movie struct {
	// File information
	FilePath     string `json:"file_path"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	
	// Movie identification
	DVDID        string `json:"dvdid"`        // DVD ID (main identifier)
	CID          string `json:"cid"`          // DMM Content ID
	PID          string `json:"pid"`          // DMM Product ID
	
	// Metadata
	Info         *MovieInfo `json:"info,omitempty"`
}

// MovieInfo contains detailed metadata about a movie
type MovieInfo struct {
	// Basic information
	DVDID        string    `json:"dvdid"`
	CID          string    `json:"cid"`
	Title        string    `json:"title"`
	OriginalTitle string   `json:"original_title,omitempty"`
	Plot         string    `json:"plot,omitempty"`
	
	// Release information
	ReleaseDate  string    `json:"release_date,omitempty"`
	Year         string    `json:"year,omitempty"`
	Runtime      string    `json:"runtime,omitempty"`
	
	// People
	Director     string    `json:"director,omitempty"`
	Producer     string    `json:"producer,omitempty"`
	Publisher    string    `json:"publisher,omitempty"`
	Actress      []string  `json:"actress,omitempty"`
	
	// Categories
	Genre        []string  `json:"genre,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	Series       string    `json:"series,omitempty"`
	
	// Media information
	Cover        string    `json:"cover,omitempty"`        // Cover image URL
	Fanart       string    `json:"fanart,omitempty"`       // Fanart image URL
	ExtraFanarts []string  `json:"extra_fanarts,omitempty"` // Additional fanart URLs
	Preview      []string  `json:"preview,omitempty"`      // Preview image URLs
	
	// Ratings and scores
	Rating       float64   `json:"rating,omitempty"`
	Votes        int       `json:"votes,omitempty"`
	
	// Additional metadata
	Website      string    `json:"website,omitempty"`
	Label        string    `json:"label,omitempty"`
	Uncensored   bool      `json:"uncensored,omitempty"`
	HasSubtitle  bool      `json:"has_subtitle,omitempty"`
	
	// Source information
	Source       string    `json:"source,omitempty"`       // Which site this data came from
	SourceURL    string    `json:"source_url,omitempty"`   // Original URL
	
	// Internal fields
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewMovie creates a new Movie instance from file path
func NewMovie(filePath string) *Movie {
	return &Movie{
		FilePath: filePath,
		FileName: filepath.Base(filePath),
	}
}

// NewMovieInfo creates a new MovieInfo instance
func NewMovieInfo(dvdid string) *MovieInfo {
	now := time.Now()
	return &MovieInfo{
		DVDID:     dvdid,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// GetID returns the primary identifier (DVDID or CID)
func (m *Movie) GetID() string {
	if m.DVDID != "" {
		return m.DVDID
	}
	return m.CID
}

// GetDisplayTitle returns the display title for the movie
func (m *Movie) GetDisplayTitle() string {
	if m.Info != nil && m.Info.Title != "" {
		return m.Info.Title
	}
	return m.GetID()
}

// IsValid checks if the movie has valid identification
func (m *Movie) IsValid() bool {
	return m.DVDID != "" || m.CID != ""
}

// GetActressString returns actresses as a comma-separated string
func (mi *MovieInfo) GetActressString() string {
	return strings.Join(mi.Actress, ", ")
}

// GetGenreString returns genres as a comma-separated string
func (mi *MovieInfo) GetGenreString() string {
	return strings.Join(mi.Genre, ", ")
}

// GetRuntimeMinutes returns runtime in minutes
func (mi *MovieInfo) GetRuntimeMinutes() int {
	if mi.Runtime == "" {
		return 0
	}
	
	// Handle formats like "120分钟", "2:00:00", "120min", "120"
	runtime := strings.ToLower(mi.Runtime)
	runtime = strings.ReplaceAll(runtime, "分钟", "")
	runtime = strings.ReplaceAll(runtime, "min", "")
	runtime = strings.TrimSpace(runtime)
	
	// Handle time format HH:MM:SS or MM:SS
	if strings.Contains(runtime, ":") {
		parts := strings.Split(runtime, ":")
		var totalMinutes int
		
		switch len(parts) {
		case 2: // MM:SS
			if minutes, err := strconv.Atoi(parts[0]); err == nil {
				totalMinutes = minutes
				if seconds, err := strconv.Atoi(parts[1]); err == nil {
					totalMinutes += (seconds + 30) / 60 // Round to nearest minute
				}
			}
		case 3: // HH:MM:SS
			if hours, err := strconv.Atoi(parts[0]); err == nil {
				totalMinutes = hours * 60
				if minutes, err := strconv.Atoi(parts[1]); err == nil {
					totalMinutes += minutes
					if seconds, err := strconv.Atoi(parts[2]); err == nil {
						totalMinutes += (seconds + 30) / 60 // Round to nearest minute
					}
				}
			}
		}
		
		return totalMinutes
	}
	
	// Handle simple number format
	if minutes, err := strconv.Atoi(runtime); err == nil {
		return minutes
	}
	
	return 0
}

// GetYear extracts year from release date
func (mi *MovieInfo) GetYear() string {
	if mi.Year != "" {
		return mi.Year
	}
	
	if mi.ReleaseDate != "" {
		// Try to parse various date formats
		dateFormats := []string{
			"2006-01-02",
			"2006/01/02",
			"2006.01.02",
			"20060102",
		}
		
		for _, format := range dateFormats {
			if t, err := time.Parse(format, mi.ReleaseDate); err == nil {
				return strconv.Itoa(t.Year())
			}
		}
		
		// If parsing fails, try to extract year from string
		if len(mi.ReleaseDate) >= 4 {
			if year, err := strconv.Atoi(mi.ReleaseDate[:4]); err == nil && year > 1900 && year <= time.Now().Year() {
				return strconv.Itoa(year)
			}
		}
	}
	
	return ""
}

// Merge merges another MovieInfo into this one, with priority rules
func (mi *MovieInfo) Merge(other *MovieInfo) {
	if other == nil {
		return
	}
	
	// Update timestamp
	mi.UpdatedAt = time.Now()
	
	// Basic information - prefer non-empty values
	if other.Title != "" {
		mi.Title = other.Title
	}
	if other.OriginalTitle != "" {
		mi.OriginalTitle = other.OriginalTitle
	}
	if other.Plot != "" {
		mi.Plot = other.Plot
	}
	
	// Release information
	if other.ReleaseDate != "" {
		mi.ReleaseDate = other.ReleaseDate
	}
	if other.Year != "" {
		mi.Year = other.Year
	}
	if other.Runtime != "" {
		mi.Runtime = other.Runtime
	}
	
	// People
	if other.Director != "" {
		mi.Director = other.Director
	}
	if other.Producer != "" {
		mi.Producer = other.Producer
	}
	if other.Publisher != "" {
		mi.Publisher = other.Publisher
	}
	
	// Merge arrays (remove duplicates)
	mi.Actress = mergeStringSlices(mi.Actress, other.Actress)
	mi.Genre = mergeStringSlices(mi.Genre, other.Genre)
	mi.Tags = mergeStringSlices(mi.Tags, other.Tags)
	mi.ExtraFanarts = mergeStringSlices(mi.ExtraFanarts, other.ExtraFanarts)
	mi.Preview = mergeStringSlices(mi.Preview, other.Preview)
	
	// Series
	if other.Series != "" {
		mi.Series = other.Series
	}
	
	// Media - prefer higher quality or longer URLs (usually more detailed)
	if other.Cover != "" && (mi.Cover == "" || len(other.Cover) > len(mi.Cover)) {
		mi.Cover = other.Cover
	}
	if other.Fanart != "" && (mi.Fanart == "" || len(other.Fanart) > len(mi.Fanart)) {
		mi.Fanart = other.Fanart
	}
	
	// Ratings - prefer higher vote counts
	if other.Rating > 0 && (mi.Rating == 0 || other.Votes > mi.Votes) {
		mi.Rating = other.Rating
		mi.Votes = other.Votes
	}
	
	// Additional metadata
	if other.Website != "" {
		mi.Website = other.Website
	}
	if other.Label != "" {
		mi.Label = other.Label
	}
	
	// Boolean fields - OR logic for positive attributes
	mi.Uncensored = mi.Uncensored || other.Uncensored
	mi.HasSubtitle = mi.HasSubtitle || other.HasSubtitle
	
	// Source information - keep track of sources
	if other.Source != "" {
		if mi.Source == "" {
			mi.Source = other.Source
		} else if !strings.Contains(mi.Source, other.Source) {
			mi.Source = mi.Source + "," + other.Source
		}
	}
	if other.SourceURL != "" {
		mi.SourceURL = other.SourceURL
	}
}

// Clone creates a deep copy of MovieInfo
func (mi *MovieInfo) Clone() *MovieInfo {
	clone := &MovieInfo{
		DVDID:         mi.DVDID,
		CID:           mi.CID,
		Title:         mi.Title,
		OriginalTitle: mi.OriginalTitle,
		Plot:          mi.Plot,
		ReleaseDate:   mi.ReleaseDate,
		Year:          mi.Year,
		Runtime:       mi.Runtime,
		Director:      mi.Director,
		Producer:      mi.Producer,
		Publisher:     mi.Publisher,
		Series:        mi.Series,
		Cover:         mi.Cover,
		Fanart:        mi.Fanart,
		Rating:        mi.Rating,
		Votes:         mi.Votes,
		Website:       mi.Website,
		Label:         mi.Label,
		Uncensored:    mi.Uncensored,
		HasSubtitle:   mi.HasSubtitle,
		Source:        mi.Source,
		SourceURL:     mi.SourceURL,
		CreatedAt:     mi.CreatedAt,
		UpdatedAt:     mi.UpdatedAt,
	}
	
	// Deep copy slices
	clone.Actress = make([]string, len(mi.Actress))
	copy(clone.Actress, mi.Actress)
	
	clone.Genre = make([]string, len(mi.Genre))
	copy(clone.Genre, mi.Genre)
	
	clone.Tags = make([]string, len(mi.Tags))
	copy(clone.Tags, mi.Tags)
	
	clone.ExtraFanarts = make([]string, len(mi.ExtraFanarts))
	copy(clone.ExtraFanarts, mi.ExtraFanarts)
	
	clone.Preview = make([]string, len(mi.Preview))
	copy(clone.Preview, mi.Preview)
	
	return clone
}

// Validate checks if the MovieInfo has minimum required fields
func (mi *MovieInfo) Validate() error {
	if mi.DVDID == "" && mi.CID == "" {
		return fmt.Errorf("either DVDID or CID must be provided")
	}
	
	if mi.Title == "" {
		return fmt.Errorf("title is required")
	}
	
	return nil
}

// String returns a string representation of the MovieInfo
func (mi *MovieInfo) String() string {
	id := mi.DVDID
	if id == "" {
		id = mi.CID
	}
	return fmt.Sprintf("[%s] %s (%s)", id, mi.Title, mi.GetActressString())
}

// ToJSON converts MovieInfo to JSON string
func (mi *MovieInfo) ToJSON() (string, error) {
	data, err := json.MarshalIndent(mi, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(data), nil
}

// FromJSON creates MovieInfo from JSON string
func FromJSON(jsonStr string) (*MovieInfo, error) {
	var mi MovieInfo
	if err := json.Unmarshal([]byte(jsonStr), &mi); err != nil {
		return nil, fmt.Errorf("failed to unmarshal from JSON: %w", err)
	}
	return &mi, nil
}

// mergeStringSlices merges two string slices, removing duplicates
func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	// Add all elements from slice a
	for _, item := range a {
		if item != "" && !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	
	// Add unique elements from slice b
	for _, item := range b {
		if item != "" && !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	
	return result
}