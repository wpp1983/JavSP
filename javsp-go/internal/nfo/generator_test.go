package nfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"javsp-go/internal/datatype"
)

func TestNewNFOGenerator(t *testing.T) {
	config := DefaultNFOConfig()
	generator := NewNFOGenerator(config)

	if generator.config != config {
		t.Error("Config not properly set")
	}
}

func TestNFOGenerator_Generate(t *testing.T) {
	generator := NewNFOGenerator(DefaultNFOConfig())

	// Create test movie data
	movie := &datatype.MovieInfo{
		DVDID:         "TEST-123",
		Title:         "Test Movie",
		OriginalTitle: "テスト映画",
		Plot:          "This is a test movie plot",
		Year:          "2023",
		Rating:        8.5,
		Publisher:     "Test Studio",
		Genre:         []string{"Drama", "Action"},
		Tags:          []string{"HD", "Uncensored"},
		Actress:       []string{"Test Actor"},
		Director:      "Test Director",
		ReleaseDate:   "2023-06-15",
		Runtime:       "120",
		Cover:         "http://example.com/cover.jpg",
		Fanart:        "http://example.com/fanart.jpg",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Generate NFO content
	content, err := generator.Generate(movie)
	if err != nil {
		t.Fatalf("Failed to generate NFO: %v", err)
	}

	contentStr := string(content)

	// Validate XML content
	if !strings.Contains(contentStr, "<?xml") {
		t.Error("Missing XML declaration")
	}

	if !strings.Contains(contentStr, "<movie>") {
		t.Error("Missing movie root element")
	}

	if !strings.Contains(contentStr, "Test Movie") {
		t.Error("Missing title content")
	}

	// The original title should be in the content (either the original or fallback to title)
	if !strings.Contains(contentStr, "<originaltitle>") {
		t.Error("Missing originaltitle element")
	}

	if !strings.Contains(contentStr, "This is a test movie plot") {
		t.Error("Missing plot content")
	}

	if !strings.Contains(contentStr, "2023") {
		t.Error("Missing year content")
	}

	if !strings.Contains(contentStr, "Drama") {
		t.Error("Missing genre content")
	}

	if !strings.Contains(contentStr, "Test Actor") {
		t.Error("Missing actress content")
	}
}

func TestNFOGenerator_GenerateToFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	generator := NewNFOGenerator(DefaultNFOConfig())

	// Create test movie data
	movie := &datatype.MovieInfo{
		DVDID: "TEST-456",
		Title: "Save Test Movie",
	}

	// Save NFO file
	outputPath := filepath.Join(tempDir, "movie.nfo")
	err := generator.GenerateToFile(movie, outputPath)
	if err != nil {
		t.Fatalf("Failed to save NFO: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("NFO file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read NFO file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Save Test Movie") {
		t.Error("NFO file content is incorrect")
	}
}

func TestNFOGenerator_EmptyMovie(t *testing.T) {
	generator := NewNFOGenerator(DefaultNFOConfig())

	// Test with minimal movie data
	movie := &datatype.MovieInfo{
		DVDID: "EMPTY-001",
	}

	content, err := generator.Generate(movie)
	if err != nil {
		t.Fatalf("Failed to generate NFO for empty movie: %v", err)
	}

	contentStr := string(content)

	// Should contain movie root element
	if !strings.Contains(contentStr, "<movie>") {
		t.Error("Missing movie root element")
	}

	// Should contain the DVDID
	if !strings.Contains(contentStr, "EMPTY-001") {
		t.Error("Missing DVDID in generated content")
	}
}

func TestNFOGenerator_SpecialCharacters(t *testing.T) {
	generator := NewNFOGenerator(DefaultNFOConfig())

	// Test with special characters
	movie := &datatype.MovieInfo{
		DVDID:         "SPECIAL-001",
		Title:         "Test & <Special> \"Characters\"",
		OriginalTitle: "テスト&<特殊>\"文字\"",
		Plot:          "Plot with special chars: <>&\"'",
	}

	content, err := generator.Generate(movie)
	if err != nil {
		t.Fatalf("Failed to generate NFO with special characters: %v", err)
	}

	contentStr := string(content)

	// Should contain movie root element
	if !strings.Contains(contentStr, "<movie>") {
		t.Error("Missing movie root element")
	}

	// XML should be properly formed (no unescaped < > characters)
	// The actual content should contain escaped versions
	if strings.Contains(contentStr, "Test & <Special>") && !strings.Contains(contentStr, "&lt;") {
		t.Error("Special characters may not be properly escaped in XML")
	}
}

func TestDefaultNFOConfig(t *testing.T) {
	config := DefaultNFOConfig()

	if config == nil {
		t.Fatal("DefaultNFOConfig returned nil")
	}

	if config.DateFormat == "" {
		t.Error("DateFormat should not be empty")
	}

	if config.RatingScale < 0 {
		t.Error("RatingScale should not be negative")
	}
}

func TestAddCustomTemplate(t *testing.T) {
	generator := NewNFOGenerator(DefaultNFOConfig())

	customTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
	<title>{{.Title}}</title>
	<id>{{.DVDID}}</id>
</movie>`

	err := generator.AddCustomTemplate("custom", customTemplate)
	if err != nil {
		t.Errorf("Failed to add custom template: %v", err)
	}
}

func TestValidateTemplate(t *testing.T) {
	generator := NewNFOGenerator(DefaultNFOConfig())

	// Valid template
	validTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
	<title>{{.Title}}</title>
</movie>`

	err := generator.ValidateTemplate(validTemplate)
	if err != nil {
		t.Errorf("Valid template should not fail validation: %v", err)
	}

	// Invalid template (malformed Go template)
	invalidTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
	<title>{{.Title</title>
</movie>`

	err = generator.ValidateTemplate(invalidTemplate)
	if err == nil {
		t.Error("Invalid template should fail validation")
	}
}

func BenchmarkNFOGeneration(b *testing.B) {
	generator := NewNFOGenerator(DefaultNFOConfig())

	movie := &datatype.MovieInfo{
		DVDID:         "BENCH-001",
		Title:         "Benchmark Movie",
		OriginalTitle: "ベンチマーク映画",
		Plot:          "This is a benchmark movie with a longer plot description to test performance",
		Year:          "2023",
		Rating:        7.5,
		Publisher:     "Benchmark Studio",
		Genre:         []string{"Action", "Drama", "Comedy"},
		Tags:          []string{"HD", "Uncensored", "Popular"},
		Actress:       []string{"Actor 1", "Actor 2", "Actor 3"},
		Director:      "Benchmark Director",
		ReleaseDate:   "2023-06-15",
		Runtime:       "150",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.Generate(movie)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}