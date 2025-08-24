package datatype

import (
	"strings"
	"testing"
	"time"
)

func TestNewMovie(t *testing.T) {
	filePath := "/test/path/STARS-123.mp4"
	movie := NewMovie(filePath)

	if movie.FilePath != filePath {
		t.Errorf("Expected FilePath %s, got %s", filePath, movie.FilePath)
	}

	expectedFileName := "STARS-123.mp4"
	if movie.FileName != expectedFileName {
		t.Errorf("Expected FileName %s, got %s", expectedFileName, movie.FileName)
	}
}

func TestNewMovieInfo(t *testing.T) {
	dvdid := "STARS-123"
	info := NewMovieInfo(dvdid)

	if info.DVDID != dvdid {
		t.Errorf("Expected DVDID %s, got %s", dvdid, info.DVDID)
	}

	if info.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if info.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestMovie_GetID(t *testing.T) {
	tests := []struct {
		name     string
		movie    *Movie
		expected string
	}{
		{
			name: "DVDID preferred",
			movie: &Movie{
				DVDID: "STARS-123",
				CID:   "stars00123",
			},
			expected: "STARS-123",
		},
		{
			name: "CID fallback",
			movie: &Movie{
				DVDID: "",
				CID:   "stars00123",
			},
			expected: "stars00123",
		},
		{
			name: "Empty",
			movie: &Movie{
				DVDID: "",
				CID:   "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.movie.GetID()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMovie_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		movie    *Movie
		expected bool
	}{
		{
			name: "Valid with DVDID",
			movie: &Movie{
				DVDID: "STARS-123",
			},
			expected: true,
		},
		{
			name: "Valid with CID",
			movie: &Movie{
				CID: "stars00123",
			},
			expected: true,
		},
		{
			name: "Valid with both",
			movie: &Movie{
				DVDID: "STARS-123",
				CID:   "stars00123",
			},
			expected: true,
		},
		{
			name: "Invalid",
			movie: &Movie{
				DVDID: "",
				CID:   "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.movie.IsValid()
			if result != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestMovieInfo_GetRuntimeMinutes(t *testing.T) {
	tests := []struct {
		name     string
		runtime  string
		expected int
	}{
		{"Empty", "", 0},
		{"Simple minutes", "120", 120},
		{"With 分钟", "120分钟", 120},
		{"With min", "120min", 120},
		{"MM:SS format", "120:30", 121}, // Rounded up
		{"HH:MM:SS format", "2:00:30", 121}, // 2*60 + 0 + 1 (rounded)
		{"HH:MM:SS exact", "1:30:00", 90},
		{"Invalid format", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &MovieInfo{Runtime: tt.runtime}
			result := info.GetRuntimeMinutes()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestMovieInfo_GetYear(t *testing.T) {
	tests := []struct {
		name        string
		year        string
		releaseDate string
		expected    string
	}{
		{"Direct year", "2023", "", "2023"},
		{"From release date YYYY-MM-DD", "", "2023-01-15", "2023"},
		{"From release date YYYY/MM/DD", "", "2023/01/15", "2023"},
		{"From release date YYYY.MM.DD", "", "2023.01.15", "2023"},
		{"From release date YYYYMMDD", "", "20230115", "2023"},
		{"Year prefix in date", "", "2023abc", "2023"},
		{"Both available", "2024", "2023-01-15", "2024"}, // Year takes precedence
		{"Invalid date", "", "invalid", ""},
		{"Empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &MovieInfo{
				Year:        tt.year,
				ReleaseDate: tt.releaseDate,
			}
			result := info.GetYear()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMovieInfo_Merge(t *testing.T) {
	base := &MovieInfo{
		DVDID:    "STARS-123",
		Title:    "Original Title",
		Actress:  []string{"Actress A"},
		Rating:   8.5,
		Votes:    100,
		CreatedAt: time.Now().Add(-time.Hour),
	}

	other := &MovieInfo{
		DVDID:    "STARS-123",
		Title:    "New Title",
		Plot:     "New Plot",
		Actress:  []string{"Actress B", "Actress C"},
		Genre:    []string{"Genre 1", "Genre 2"},
		Rating:   9.0,
		Votes:    200, // Higher votes
		Director: "Director Name",
	}

	base.Merge(other)

	// Check merged values
	if base.Title != "New Title" {
		t.Errorf("Expected title to be updated to 'New Title', got %s", base.Title)
	}

	if base.Plot != "New Plot" {
		t.Errorf("Expected plot to be 'New Plot', got %s", base.Plot)
	}

	if base.Director != "Director Name" {
		t.Errorf("Expected director to be 'Director Name', got %s", base.Director)
	}

	// Check merged arrays
	expectedActresses := []string{"Actress A", "Actress B", "Actress C"}
	if len(base.Actress) != 3 {
		t.Errorf("Expected 3 actresses, got %d", len(base.Actress))
	}
	for i, expected := range expectedActresses {
		if i >= len(base.Actress) || base.Actress[i] != expected {
			t.Errorf("Expected actress[%d] to be %s, got %s", i, expected, base.Actress[i])
		}
	}

	// Check rating update (higher votes should win)
	if base.Rating != 9.0 {
		t.Errorf("Expected rating to be 9.0, got %f", base.Rating)
	}

	if base.Votes != 200 {
		t.Errorf("Expected votes to be 200, got %d", base.Votes)
	}

	// Check UpdatedAt was updated
	if base.UpdatedAt.Before(base.CreatedAt) {
		t.Error("Expected UpdatedAt to be updated after merge")
	}
}

func TestMovieInfo_Clone(t *testing.T) {
	original := &MovieInfo{
		DVDID:     "STARS-123",
		Title:     "Test Title",
		Actress:   []string{"Actress A", "Actress B"},
		Genre:     []string{"Genre 1"},
		Rating:    8.5,
		CreatedAt: time.Now(),
	}

	clone := original.Clone()

	// Check basic fields
	if clone.DVDID != original.DVDID {
		t.Error("DVDID not cloned correctly")
	}

	if clone.Title != original.Title {
		t.Error("Title not cloned correctly")
	}

	if clone.Rating != original.Rating {
		t.Error("Rating not cloned correctly")
	}

	// Check deep copy of slices
	if len(clone.Actress) != len(original.Actress) {
		t.Error("Actress slice not cloned correctly")
	}

	// Modify clone to ensure independence
	clone.Title = "Modified Title"
	clone.Actress[0] = "Modified Actress"

	if original.Title == clone.Title {
		t.Error("Original and clone are not independent")
	}

	if original.Actress[0] == clone.Actress[0] {
		t.Error("Actress slices are not independent")
	}
}

func TestMovieInfo_Validate(t *testing.T) {
	tests := []struct {
		name      string
		info      *MovieInfo
		expectErr bool
	}{
		{
			name: "Valid with DVDID",
			info: &MovieInfo{
				DVDID: "STARS-123",
				Title: "Test Title",
			},
			expectErr: false,
		},
		{
			name: "Valid with CID",
			info: &MovieInfo{
				CID:   "stars00123",
				Title: "Test Title",
			},
			expectErr: false,
		},
		{
			name: "Missing ID",
			info: &MovieInfo{
				Title: "Test Title",
			},
			expectErr: true,
		},
		{
			name: "Missing title",
			info: &MovieInfo{
				DVDID: "STARS-123",
			},
			expectErr: true,
		},
		{
			name:      "Empty info",
			info:      &MovieInfo{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestMovieInfo_JSON(t *testing.T) {
	original := &MovieInfo{
		DVDID:     "STARS-123",
		Title:     "Test Title",
		Actress:   []string{"Actress A"},
		Rating:    8.5,
		CreatedAt: time.Now().Round(time.Second), // Round to avoid precision issues
		UpdatedAt: time.Now().Round(time.Second),
	}

	// Test ToJSON
	jsonStr, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if !strings.Contains(jsonStr, "STARS-123") {
		t.Error("JSON should contain DVDID")
	}

	// Test FromJSON
	restored, err := FromJSON(jsonStr)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if restored.DVDID != original.DVDID {
		t.Errorf("Expected DVDID %s, got %s", original.DVDID, restored.DVDID)
	}

	if restored.Title != original.Title {
		t.Errorf("Expected Title %s, got %s", original.Title, restored.Title)
	}

	if len(restored.Actress) != len(original.Actress) {
		t.Error("Actress array not restored correctly")
	}

	if restored.Rating != original.Rating {
		t.Errorf("Expected Rating %f, got %f", original.Rating, restored.Rating)
	}
}

func TestMergeStringSlices(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "No duplicates",
			a:        []string{"A", "B"},
			b:        []string{"C", "D"},
			expected: []string{"A", "B", "C", "D"},
		},
		{
			name:     "With duplicates",
			a:        []string{"A", "B"},
			b:        []string{"B", "C"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "Empty slices",
			a:        []string{},
			b:        []string{"A"},
			expected: []string{"A"},
		},
		{
			name:     "With empty strings",
			a:        []string{"A", ""},
			b:        []string{"", "B"},
			expected: []string{"A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeStringSlices(tt.a, tt.b)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Expected element[%d] to be %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}