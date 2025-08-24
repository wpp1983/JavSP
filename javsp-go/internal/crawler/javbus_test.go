package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Mock HTML content for testing
const mockJavBusHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>STARS-123 - Test Movie Title</title>
</head>
<body>
    <h3>STARS-123 Test Movie Title</h3>
    <div class="bigImage">
        <img src="https://www.javbus.com/cover/stars-123.jpg" alt="Cover">
    </div>
    
    <div class="info">
        <p>發行日期: 2023-01-15</p>
        <p>長度: 120分鐘</p>
        <p>導演: Test Director</p>
        <p>製作商: Test Studio</p>
        <p>發行商: Test Publisher</p>
        <p>系列: Test Series</p>
    </div>
    
    <div class="star-box">
        <div class="star-name"><a href="/star/123">Test Actress 1</a></div>
        <div class="star-name"><a href="/star/456">Test Actress 2</a></div>
    </div>
    
    <div class="genre">
        <a href="/genre/1">Drama</a>
        <a href="/genre/2">Romance</a>
    </div>
    
    <div class="sample-box">
        <img src="https://www.javbus.com/preview/stars-123-1.jpg">
        <img src="https://www.javbus.com/preview/stars-123-2.jpg">
    </div>
</body>
</html>
`

func TestJavBusCrawler_FetchMovieInfo(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockJavBusHTML))
	}))
	defer server.Close()

	// Create crawler with test config
	config := &CrawlerConfig{
		BaseURL:    server.URL,
		Timeout:    10 * time.Second,
		MaxRetries: 1,
		RetryDelay: 1 * time.Second,
		RateLimit:  0,
	}

	crawler, err := NewJavBusCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create crawler: %v", err)
	}
	defer crawler.Close()

	// Test fetching movie info
	ctx := context.Background()
	movieInfo, err := crawler.FetchMovieInfo(ctx, "STARS-123")
	if err != nil {
		t.Fatalf("Failed to fetch movie info: %v", err)
	}

	// Verify extracted data
	if movieInfo.DVDID != "STARS-123" {
		t.Errorf("Expected DVDID 'STARS-123', got '%s'", movieInfo.DVDID)
	}

	if movieInfo.Title != "Test Movie Title" {
		t.Errorf("Expected title 'Test Movie Title', got '%s'", movieInfo.Title)
	}

	if movieInfo.ReleaseDate != "2023-01-15" {
		t.Errorf("Expected release date '2023-01-15', got '%s'", movieInfo.ReleaseDate)
	}

	if movieInfo.Runtime != "120分鐘" {
		t.Errorf("Expected runtime '120分鐘', got '%s'", movieInfo.Runtime)
	}

	if movieInfo.Director != "Test Director" {
		t.Errorf("Expected director 'Test Director', got '%s'", movieInfo.Director)
	}

	if len(movieInfo.Actress) != 2 {
		t.Errorf("Expected 2 actresses, got %d", len(movieInfo.Actress))
	}

	if len(movieInfo.Genre) != 2 {
		t.Errorf("Expected 2 genres, got %d", len(movieInfo.Genre))
	}

	if len(movieInfo.Preview) != 2 {
		t.Errorf("Expected 2 preview images, got %d", len(movieInfo.Preview))
	}

	if movieInfo.Source != "javbus2" {
		t.Errorf("Expected source 'javbus2', got '%s'", movieInfo.Source)
	}
}

func TestJavBusCrawler_constructMovieURL(t *testing.T) {
	config := &CrawlerConfig{
		BaseURL: "https://www.javbus.com",
	}

	crawler, err := NewJavBusCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create crawler: %v", err)
	}

	tests := []struct {
		movieID  string
		expected string
	}{
		{"STARS-123", "https://www.javbus.com/STARS-123"},
		{"FC2-1234567", "https://www.javbus.com/FC2-1234567"},
		{"PRED-456", "https://www.javbus.com/PRED-456"},
	}

	for _, test := range tests {
		result := crawler.constructMovieURL(test.movieID)
		if result != test.expected {
			t.Errorf("constructMovieURL(%s): expected %s, got %s", test.movieID, test.expected, result)
		}
	}
}

func TestJavBusCrawler_cleanTitle(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewJavBusCrawler(config)

	tests := []struct {
		input    string
		expected string
	}{
		{"STARS-123 Test Title", "Test Title"},
		{"FC2-PPV-1234567 Another Title", "Another Title"},
		{"  Spaced Title  ", "Spaced Title"},
		{"Clean Title", "Clean Title"},
	}

	for _, test := range tests {
		result := crawler.cleanTitle(test.input)
		if result != test.expected {
			t.Errorf("cleanTitle(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestJavBusCrawler_normalizeImageURL(t *testing.T) {
	config := &CrawlerConfig{BaseURL: "https://www.javbus.com"}
	crawler, _ := NewJavBusCrawler(config)

	tests := []struct {
		input    string
		expected string
	}{
		{"//example.com/image.jpg", "https://example.com/image.jpg"},
		{"/images/cover.jpg", "https://www.javbus.com/images/cover.jpg"},
		{"https://example.com/full.jpg", "https://example.com/full.jpg"},
		{"", ""},
	}

	for _, test := range tests {
		result := crawler.normalizeImageURL(test.input)
		if result != test.expected {
			t.Errorf("normalizeImageURL(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestJavBusCrawler_GetSupportedTypes(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewJavBusCrawler(config)

	types := crawler.GetSupportedTypes()
	expected := []string{"normal", "fc2", "amateur"}

	if len(types) != len(expected) {
		t.Errorf("Expected %d supported types, got %d", len(expected), len(types))
	}

	for i, expectedType := range expected {
		if i >= len(types) || types[i] != expectedType {
			t.Errorf("Expected type[%d] to be '%s', got '%s'", i, expectedType, types[i])
		}
	}
}

func TestJavBusCrawler_IsAvailable(t *testing.T) {
	// Test with available server
	availableServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer availableServer.Close()

	config := &CrawlerConfig{BaseURL: availableServer.URL}
	crawler, _ := NewJavBusCrawler(config)
	defer crawler.Close()

	ctx := context.Background()
	if !crawler.IsAvailable(ctx) {
		t.Error("Expected crawler to be available")
	}

	// Test with unavailable URL
	config2 := &CrawlerConfig{BaseURL: "http://localhost:99999"}
	crawler2, _ := NewJavBusCrawler(config2)
	defer crawler2.Close()

	if crawler2.IsAvailable(ctx) {
		t.Error("Expected crawler to be unavailable")
	}
}

func TestJavBusCrawler_extractMovieIDFromURL(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewJavBusCrawler(config)

	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.javbus.com/STARS-123", "STARS-123"},
		{"https://www.javbus.com/FC2-1234567", "FC2-1234567"},
		{"/PRED-456", "PRED-456"},
		{"", ""},
	}

	for _, test := range tests {
		result := crawler.extractMovieIDFromURL(test.url)
		if result != test.expected {
			t.Errorf("extractMovieIDFromURL(%s): expected '%s', got '%s'", test.url, test.expected, result)
		}
	}
}

func BenchmarkJavBusCrawler_FetchMovieInfo(b *testing.B) {
	// Create simple mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><h3>Test</h3></body></html>`))
	}))
	defer server.Close()

	config := &CrawlerConfig{
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		RateLimit:  0,
	}

	crawler, err := NewJavBusCrawler(config)
	if err != nil {
		b.Fatalf("Failed to create crawler: %v", err)
	}
	defer crawler.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := crawler.FetchMovieInfo(ctx, "TEST-123")
		if err != nil && !strings.Contains(err.Error(), "validation failed") {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}