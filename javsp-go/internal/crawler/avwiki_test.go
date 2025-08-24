package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Mock HTML content for AVWiki testing
const mockAVWikiHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>SSIS-698 - Test Movie Page</title>
</head>
<body>
    <h1>佐野ゆま と 田中みお SSIS-698 まとめ</h1>
    
    <div class="entry-content">
        <p>メーカー: エスワン ナンバーワンスタイル</p>
        <p>配信開始日: 2023-01-15</p>
        <p>シリーズ: S1 Premium</p>
        <p>スタジオ: S1 Studio</p>
        <img src="https://pics.dmm.co.jp/digital/video/ssis00698/ssis00698pl.jpg" alt="Cover">
        <p>その他の情報...</p>
    </div>
</body>
</html>
`

const mockAVWiki404HTML = `
<!DOCTYPE html>
<html>
<head>
    <title>404 Not Found</title>
</head>
<body>
    <h1>Page Not Found</h1>
    <p>The requested page could not be found.</p>
</body>
</html>
`

func TestAVWikiCrawler_FetchMovieInfo(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "NOT-FOUND") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(mockAVWiki404HTML))
			return
		}
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockAVWikiHTML))
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

	crawler, err := NewAVWikiCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create crawler: %v", err)
	}
	defer crawler.Close()

	// Test fetching movie info
	ctx := context.Background()
	movieInfo, err := crawler.FetchMovieInfo(ctx, "SSIS-698")
	if err != nil {
		t.Fatalf("Failed to fetch movie info: %v", err)
	}

	// Verify extracted data
	if movieInfo.DVDID != "SSIS-698" {
		t.Errorf("Expected DVDID 'SSIS-698', got '%s'", movieInfo.DVDID)
	}

	if movieInfo.Title != "佐野ゆま と 田中みお SSIS-698" {
		t.Errorf("Expected cleaned title, got '%s'", movieInfo.Title)
	}

	if movieInfo.Producer != "エスワン ナンバーワンスタイル" {
		t.Errorf("Expected producer 'エスワン ナンバーワンスタイル', got '%s'", movieInfo.Producer)
	}

	if movieInfo.ReleaseDate != "2023-01-15" {
		t.Errorf("Expected release date '2023-01-15', got '%s'", movieInfo.ReleaseDate)
	}

	if movieInfo.Series != "S1 Premium" {
		t.Errorf("Expected series 'S1 Premium', got '%s'", movieInfo.Series)
	}

	if len(movieInfo.Actress) == 0 {
		t.Error("Expected to extract actresses from title")
	}

	if movieInfo.Source != "avwiki" {
		t.Errorf("Expected source 'avwiki', got '%s'", movieInfo.Source)
	}

	if movieInfo.Uncensored {
		t.Error("Expected uncensored to be false for AVWiki")
	}
}

func TestAVWikiCrawler_FetchMovieInfo_NotFound(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(mockAVWiki404HTML))
	}))
	defer server.Close()

	config := &CrawlerConfig{
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		RateLimit:  0,
	}

	crawler, err := NewAVWikiCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create crawler: %v", err)
	}
	defer crawler.Close()

	ctx := context.Background()
	_, err = crawler.FetchMovieInfo(ctx, "NOT-FOUND-123")
	if err == nil {
		t.Error("Expected error for non-existent movie")
	}

	if !strings.Contains(err.Error(), "movie not found") {
		t.Errorf("Expected 'movie not found' error, got: %v", err)
	}
}

func TestAVWikiCrawler_cleanTitle(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewAVWikiCrawler(config)

	tests := []struct {
		input    string
		expected string
	}{
		{"佐野ゆま と 田中みお SSIS-698に出てるAV女優名まとめ", "佐野ゆま と 田中みお SSIS-698"},
		{"テスト映画まとめ", "テスト映画"},
		{"普通のタイトル", "普通のタイトル"},
		{"  スペース付きタイトル  ", "スペース付きタイトル"},
	}

	for _, test := range tests {
		result := crawler.cleanTitle(test.input)
		if result != test.expected {
			t.Errorf("cleanTitle(%s): expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestAVWikiCrawler_extractActressesFromTitle(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewAVWikiCrawler(config)

	tests := []struct {
		input    string
		expected []string
	}{
		{"佐野ゆま と 田中みお", []string{"佐野ゆま", "田中みお"}},
		{"田中美玲", []string{"田中美玲"}},
		{"山田花子、佐藤太郎、田中美玲", []string{"山田花子", "佐藤太郎", "田中美玲"}},
		{"SSIS-698", []string{}}, // Should be filtered out
	}

	for _, test := range tests {
		result := crawler.extractActressesFromTitle(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("extractActressesFromTitle(%s): expected %d actresses, got %d", 
				test.input, len(test.expected), len(result))
			continue
		}

		for i, expected := range test.expected {
			if i >= len(result) || result[i] != expected {
				t.Errorf("extractActressesFromTitle(%s): expected actress[%d] '%s', got '%s'", 
					test.input, i, expected, result[i])
			}
		}
	}
}

func TestAVWikiCrawler_isNotActressName(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewAVWikiCrawler(config)

	tests := []struct {
		input    string
		expected bool
	}{
		{"佐野ゆま", false},
		{"田中みお", false},
		{"SSIS-698", true},
		{"AV女優", true},
		{"LUXU-1234", true},
		{"まとめ", true},
		{"普通の名前", false},
	}

	for _, test := range tests {
		result := crawler.isNotActressName(test.input)
		if result != test.expected {
			t.Errorf("isNotActressName(%s): expected %v, got %v", test.input, test.expected, result)
		}
	}
}

func TestAVWikiCrawler_GetSupportedTypes(t *testing.T) {
	config := &CrawlerConfig{}
	crawler, _ := NewAVWikiCrawler(config)

	types := crawler.GetSupportedTypes()
	expected := []string{"normal", "amateur"}

	if len(types) != len(expected) {
		t.Errorf("Expected %d supported types, got %d", len(expected), len(types))
	}

	for i, expectedType := range expected {
		if i >= len(types) || types[i] != expectedType {
			t.Errorf("Expected type[%d] to be '%s', got '%s'", i, expectedType, types[i])
		}
	}
}

func TestAVWikiCrawler_IsAvailable(t *testing.T) {
	// Test with available server
	availableServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer availableServer.Close()

	config := &CrawlerConfig{BaseURL: availableServer.URL}
	crawler, _ := NewAVWikiCrawler(config)
	defer crawler.Close()

	ctx := context.Background()
	if !crawler.IsAvailable(ctx) {
		t.Error("Expected crawler to be available")
	}

	// Test with unavailable URL
	config2 := &CrawlerConfig{BaseURL: "http://localhost:99999"}
	crawler2, _ := NewAVWikiCrawler(config2)
	defer crawler2.Close()

	if crawler2.IsAvailable(ctx) {
		t.Error("Expected crawler to be unavailable")
	}
}

func TestAVWikiCrawler_Search(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockAVWikiHTML))
	}))
	defer server.Close()

	config := &CrawlerConfig{
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		RateLimit:  0,
	}

	crawler, err := NewAVWikiCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create crawler: %v", err)
	}
	defer crawler.Close()

	ctx := context.Background()
	results, err := crawler.Search(ctx, "SSIS-698")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].DVDID != "SSIS-698" {
		t.Errorf("Expected DVDID 'SSIS-698', got '%s'", results[0].DVDID)
	}
}

func BenchmarkAVWikiCrawler_FetchMovieInfo(b *testing.B) {
	// Create simple mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><h1>Test Title</h1></body></html>`))
	}))
	defer server.Close()

	config := &CrawlerConfig{
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		RateLimit:  0,
	}

	crawler, err := NewAVWikiCrawler(config)
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