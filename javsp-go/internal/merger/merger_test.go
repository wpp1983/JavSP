package merger

import (
	"fmt"
	"testing"

	"javsp-go/internal/datatype"
)

func TestNewMerger(t *testing.T) {
	// Test with default config
	merger1 := NewMerger(nil)
	if merger1 == nil {
		t.Fatal("Merger should not be nil")
	}
	if merger1.config == nil {
		t.Fatal("Merger config should not be nil")
	}

	// Test with custom config
	config := &MergeConfig{
		DefaultStrategy: StrategyPreferFirst,
	}
	merger2 := NewMerger(config)
	if merger2.config.DefaultStrategy != StrategyPreferFirst {
		t.Error("Custom config not applied")
	}
}

func TestMerger_MergeNoMovies(t *testing.T) {
	merger := NewMerger(nil)

	// Test empty slice
	result, err := merger.Merge([]*datatype.MovieInfo{})
	if err == nil {
		t.Error("Expected error for empty movies slice")
	}

	// Test nil slice
	result, err = merger.Merge(nil)
	if err == nil {
		t.Error("Expected error for nil movies slice")
	}

	// Test slice with nil movies
	result, err = merger.Merge([]*datatype.MovieInfo{nil, nil})
	if err == nil {
		t.Error("Expected error for slice with only nil movies")
	}

	if result != nil {
		t.Error("Result should be nil when merge fails")
	}
}

func TestMerger_MergeSingleMovie(t *testing.T) {
	merger := NewMerger(nil)

	movie := datatype.NewMovieInfo("TEST-001")
	movie.Title = "Test Movie"
	movie.Source = "javbus2"

	movies := []*datatype.MovieInfo{movie}
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if result.MergedMovie.DVDID != "TEST-001" {
		t.Errorf("Expected DVDID 'TEST-001', got '%s'", result.MergedMovie.DVDID)
	}

	if result.MergedMovie.Title != "Test Movie" {
		t.Errorf("Expected title 'Test Movie', got '%s'", result.MergedMovie.Title)
	}

	if len(result.SourcesUsed) != 1 || result.SourcesUsed[0] != "javbus2" {
		t.Errorf("Expected one source 'javbus2', got %v", result.SourcesUsed)
	}
}

func TestMerger_MergeMultipleMovies(t *testing.T) {
	merger := NewMerger(nil)

	// Create test movies with different information
	movie1 := datatype.NewMovieInfo("TEST-001")
	movie1.Title = "Short Title"
	movie1.Director = "Director A"
	movie1.Source = "javbus2"
	movie1.Actress = []string{"Actress A", "Actress B"}

	movie2 := datatype.NewMovieInfo("TEST-001")
	movie2.Title = "This is a much longer and more detailed title"
	movie2.Producer = "Producer B"
	movie2.Source = "avwiki"
	movie2.Actress = []string{"Actress B", "Actress C"}

	movies := []*datatype.MovieInfo{movie1, movie2}
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Check that longer title was preferred (StrategyPreferLongest for title)
	expectedTitle := "This is a much longer and more detailed title"
	if result.MergedMovie.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, result.MergedMovie.Title)
	}

	// Check that director from higher quality source was preserved
	if result.MergedMovie.Director != "Director A" {
		t.Errorf("Expected director 'Director A', got '%s'", result.MergedMovie.Director)
	}

	// Check that actresses were combined
	if len(result.MergedMovie.Actress) != 3 {
		t.Errorf("Expected 3 actresses after combining, got %d", len(result.MergedMovie.Actress))
	}

	// Check sources used
	if len(result.SourcesUsed) != 2 {
		t.Errorf("Expected 2 sources used, got %d", len(result.SourcesUsed))
	}
}

func TestMerger_MergeWithConflicts(t *testing.T) {
	merger := NewMerger(nil)

	movie1 := datatype.NewMovieInfo("TEST-001")
	movie1.Title = "Title A"
	movie1.Director = "Director A"
	movie1.Source = "javbus2"

	movie2 := datatype.NewMovieInfo("TEST-001")
	movie2.Title = "Title B"
	movie2.Director = "Director B"
	movie2.Source = "avwiki"

	movies := []*datatype.MovieInfo{movie1, movie2}
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Check that conflicts were detected
	if len(result.ConflictsFound) == 0 {
		t.Error("Expected conflicts to be detected")
	}

	// Check specific conflicts
	if titleConflicts, exists := result.ConflictsFound["title"]; exists {
		if len(titleConflicts) != 2 {
			t.Errorf("Expected 2 title conflicts, got %d", len(titleConflicts))
		}
	} else {
		t.Error("Expected title conflicts to be recorded")
	}

	if directorConflicts, exists := result.ConflictsFound["director"]; exists {
		if len(directorConflicts) != 2 {
			t.Errorf("Expected 2 director conflicts, got %d", len(directorConflicts))
		}
	} else {
		t.Error("Expected director conflicts to be recorded")
	}
}

func TestMerger_SourceRanking(t *testing.T) {
	// Create merger with custom source ranking
	config := NewDefaultMergeConfig()
	config.SourceRanking["testsource1"] = 20
	config.SourceRanking["testsource2"] = 1
	merger := NewMerger(config)

	movie1 := datatype.NewMovieInfo("TEST-001")
	movie1.Title = "Low Quality Title"
	movie1.Source = "testsource2"

	movie2 := datatype.NewMovieInfo("TEST-001")
	movie2.Title = "High Quality Title"
	movie2.Source = "testsource1"

	movies := []*datatype.MovieInfo{movie1, movie2} // Lower quality first
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Should prefer the high quality source for basic quality strategy
	if result.MergedMovie.Title != "High Quality Title" {
		t.Errorf("Expected 'High Quality Title', got '%s'", result.MergedMovie.Title)
	}

	// Check that sources are ordered by quality in the result
	if result.SourcesUsed[0] != "testsource1" {
		t.Errorf("Expected first source to be 'testsource1', got '%s'", result.SourcesUsed[0])
	}
}

func TestMerger_ActressLimiting(t *testing.T) {
	config := NewDefaultMergeConfig()
	config.MaxActressCount = 2
	merger := NewMerger(config)

	movie := datatype.NewMovieInfo("TEST-001")
	movie.Title = "Test Movie"
	movie.Source = "javbus2"
	movie.Actress = []string{"A1", "A2", "A3", "A4", "A5"}

	movies := []*datatype.MovieInfo{movie}
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(result.MergedMovie.Actress) != 2 {
		t.Errorf("Expected 2 actresses after limiting, got %d", len(result.MergedMovie.Actress))
	}
}

func TestMerger_GenreLimiting(t *testing.T) {
	config := NewDefaultMergeConfig()
	config.MaxGenreCount = 3
	merger := NewMerger(config)

	movie := datatype.NewMovieInfo("TEST-001")
	movie.Title = "Test Movie"
	movie.Source = "javbus2"
	movie.Genre = []string{"G1", "G2", "G3", "G4", "G5"}

	movies := []*datatype.MovieInfo{movie}
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(result.MergedMovie.Genre) != 3 {
		t.Errorf("Expected 3 genres after limiting, got %d", len(result.MergedMovie.Genre))
	}
}

func TestMerger_QualityScore(t *testing.T) {
	merger := NewMerger(nil)

	// Create a complete movie
	completeMovie := datatype.NewMovieInfo("TEST-001")
	completeMovie.Title = "Complete Movie"
	completeMovie.Actress = []string{"Actress A"}
	completeMovie.Cover = "http://example.com/cover.jpg"
	completeMovie.Plot = "A detailed plot"
	completeMovie.Genre = []string{"Drama"}
	completeMovie.ReleaseDate = "2023-01-01"
	completeMovie.Director = "Director A"
	completeMovie.Producer = "Producer A"
	completeMovie.Source = "javbus2"

	// Create a minimal movie
	minimalMovie := datatype.NewMovieInfo("TEST-002")
	minimalMovie.Title = "Minimal Movie"
	minimalMovie.Source = "avwiki"

	// Test complete movie
	result1, err := merger.Merge([]*datatype.MovieInfo{completeMovie})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Test minimal movie
	result2, err := merger.Merge([]*datatype.MovieInfo{minimalMovie})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Complete movie should have higher quality score
	if result1.MergeStats.QualityScore <= result2.MergeStats.QualityScore {
		t.Errorf("Complete movie should have higher quality score: %.2f vs %.2f",
			result1.MergeStats.QualityScore, result2.MergeStats.QualityScore)
	}

	// Quality score should be between 0 and 1
	if result1.MergeStats.QualityScore < 0 || result1.MergeStats.QualityScore > 1 {
		t.Errorf("Quality score should be between 0 and 1, got %.2f", result1.MergeStats.QualityScore)
	}
}

func TestMerger_CombineStrategies(t *testing.T) {
	config := NewDefaultMergeConfig()
	config.FieldStrategies["actress"] = StrategyCombine
	config.FieldStrategies["genre"] = StrategyCombine
	merger := NewMerger(config)

	movie1 := datatype.NewMovieInfo("TEST-001")
	movie1.Title = "Test Movie"
	movie1.Source = "javbus2"
	movie1.Actress = []string{"Actress A", "Actress B"}
	movie1.Genre = []string{"Drama", "Romance"}

	movie2 := datatype.NewMovieInfo("TEST-001")
	movie2.Title = "Test Movie"
	movie2.Source = "avwiki"
	movie2.Actress = []string{"Actress B", "Actress C"} // B is duplicate
	movie2.Genre = []string{"Romance", "Action"}        // Romance is duplicate

	movies := []*datatype.MovieInfo{movie1, movie2}
	result, err := merger.Merge(movies)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Check that actresses were combined and deduplicated
	expectedActresses := 3
	if len(result.MergedMovie.Actress) != expectedActresses {
		t.Errorf("Expected %d unique actresses, got %d", expectedActresses, len(result.MergedMovie.Actress))
	}

	// Check that genres were combined and deduplicated
	expectedGenres := 3
	if len(result.MergedMovie.Genre) != expectedGenres {
		t.Errorf("Expected %d unique genres, got %d", expectedGenres, len(result.MergedMovie.Genre))
	}

	// Verify no duplicates in actresses
	actressMap := make(map[string]bool)
	for _, actress := range result.MergedMovie.Actress {
		if actressMap[actress] {
			t.Errorf("Duplicate actress found: %s", actress)
		}
		actressMap[actress] = true
	}

	// Verify no duplicates in genres
	genreMap := make(map[string]bool)
	for _, genre := range result.MergedMovie.Genre {
		if genreMap[genre] {
			t.Errorf("Duplicate genre found: %s", genre)
		}
		genreMap[genre] = true
	}
}

func TestMerger_selectLongest(t *testing.T) {
	merger := NewMerger(nil)

	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"short", "much longer text", "medium"}, "much longer text"},
		{[]string{"same", "same"}, "same"},
		{[]string{"only"}, "only"},
		{[]string{}, ""},
	}

	for _, test := range tests {
		result := merger.selectLongest(test.input)
		if result != test.expected {
			t.Errorf("selectLongest(%v): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestMerger_combineAndDeduplicate(t *testing.T) {
	merger := NewMerger(nil)

	tests := []struct {
		input    []string
		expected int // Expected length after deduplication
	}{
		{[]string{"a", "b", "c"}, 3},
		{[]string{"a", "b", "a"}, 2},
		{[]string{"a", "a", "a"}, 1},
		{[]string{"", "a", ""}, 1}, // Empty strings should be filtered out
		{[]string{}, 0},
	}

	for _, test := range tests {
		result := merger.combineAndDeduplicate(test.input)
		if len(result) != test.expected {
			t.Errorf("combineAndDeduplicate(%v): expected length %d, got %d", test.input, test.expected, len(result))
		}

		// Check for duplicates
		seen := make(map[string]bool)
		for _, item := range result {
			if item == "" {
				t.Errorf("Empty string should not be in result")
			}
			if seen[item] {
				t.Errorf("Duplicate item found in result: %s", item)
			}
			seen[item] = true
		}
	}
}

func BenchmarkMerger_Merge(b *testing.B) {
	merger := NewMerger(nil)

	// Create test movies
	movies := make([]*datatype.MovieInfo, 5)
	for i := 0; i < 5; i++ {
		movie := datatype.NewMovieInfo("TEST-001")
		movie.Title = fmt.Sprintf("Movie Title %d", i)
		movie.Source = fmt.Sprintf("source%d", i)
		movie.Actress = []string{fmt.Sprintf("Actress %d", i), fmt.Sprintf("Actress %d", i+10)}
		movie.Genre = []string{fmt.Sprintf("Genre %d", i)}
		movies[i] = movie
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := merger.Merge(movies)
		if err != nil {
			b.Errorf("Merge failed: %v", err)
		}
	}
}