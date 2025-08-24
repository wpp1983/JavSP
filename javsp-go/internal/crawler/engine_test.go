package crawler

import (
	"context"
	"errors"
	"testing"
	"time"

	"javsp-go/internal/config"
	"javsp-go/internal/datatype"
)

// MockCrawler for testing
type MockCrawler struct {
	name        string
	shouldError bool
	delay       time.Duration
	result      *datatype.MovieInfo
}

func (m *MockCrawler) Name() string {
	return m.name
}

func (m *MockCrawler) FetchMovieInfo(ctx context.Context, movieID string) (*datatype.MovieInfo, error) {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.shouldError {
		return nil, errors.New("mock error")
	}

	if m.result != nil {
		return m.result, nil
	}

	// Return a basic mock result
	info := datatype.NewMovieInfo(movieID)
	info.Title = "Mock Title"
	info.Source = m.name
	return info, nil
}

func (m *MockCrawler) Search(ctx context.Context, keyword string) ([]*datatype.MovieInfo, error) {
	info, err := m.FetchMovieInfo(ctx, keyword)
	if err != nil {
		return nil, err
	}
	return []*datatype.MovieInfo{info}, nil
}

func (m *MockCrawler) IsAvailable(ctx context.Context) bool {
	return !m.shouldError
}

func (m *MockCrawler) GetBaseURL() string {
	return "http://mock.example.com"
}

func (m *MockCrawler) GetSupportedTypes() []string {
	return []string{"normal"}
}

func (m *MockCrawler) Close() error {
	return nil
}

func TestNewEngine(t *testing.T) {
	cfg := &config.Config{
		Network: config.NetworkConfig{
			Timeout: 30 * time.Second,
			Retry:   3,
		},
		Crawler: config.CrawlerConfig{
			SleepAfterScraping:  1 * time.Second,
		},
	}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if engine == nil {
		t.Fatal("Engine should not be nil")
	}

	// Check that default crawlers are registered
	crawlers := engine.GetAvailableCrawlers()
	if len(crawlers) == 0 {
		t.Error("Expected default crawlers to be registered")
	}

	// Check for specific crawlers
	expectedCrawlers := map[string]bool{
		"javbus2": false,
		"avwiki":  false,
	}

	for _, name := range crawlers {
		if _, exists := expectedCrawlers[name]; exists {
			expectedCrawlers[name] = true
		}
	}

	for name, found := range expectedCrawlers {
		if !found {
			t.Errorf("Expected crawler '%s' to be registered", name)
		}
	}
}

func TestEngine_RegisterCrawler(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	mockCrawler := &MockCrawler{name: "mock"}
	engine.RegisterCrawler("mock", mockCrawler)

	crawlers := engine.GetAvailableCrawlers()
	found := false
	for _, name := range crawlers {
		if name == "mock" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Mock crawler should be registered")
	}
}

func TestEngine_CrawlMovie_Success(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Register mock crawlers
	mockCrawler1 := &MockCrawler{name: "mock1"}
	mockCrawler2 := &MockCrawler{name: "mock2"}
	engine.RegisterCrawler("mock1", mockCrawler1)
	engine.RegisterCrawler("mock2", mockCrawler2)

	ctx := context.Background()
	results, err := engine.CrawlMovie(ctx, "TEST-123", "mock1", "mock2")
	if err != nil {
		t.Fatalf("CrawlMovie failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check that both crawlers returned results
	sources := make(map[string]bool)
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Unexpected error from %s: %v", result.Source, result.Error)
		}
		if result.MovieInfo == nil {
			t.Errorf("Missing movie info from %s", result.Source)
		}
		sources[result.Source] = true
	}

	if !sources["mock1"] || !sources["mock2"] {
		t.Error("Expected results from both mock1 and mock2")
	}
}

func TestEngine_CrawlMovie_WithErrors(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Register mock crawlers - one that errors
	mockCrawler1 := &MockCrawler{name: "mock1", shouldError: true}
	mockCrawler2 := &MockCrawler{name: "mock2"}
	engine.RegisterCrawler("mock1", mockCrawler1)
	engine.RegisterCrawler("mock2", mockCrawler2)

	ctx := context.Background()
	results, err := engine.CrawlMovie(ctx, "TEST-123", "mock1", "mock2")
	if err != nil {
		t.Fatalf("CrawlMovie failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check results
	var errorCount, successCount int
	for _, result := range results {
		if result.Error != nil {
			errorCount++
			if result.Source != "mock1" {
				t.Errorf("Expected error from mock1, got error from %s", result.Source)
			}
		} else {
			successCount++
			if result.Source != "mock2" {
				t.Errorf("Expected success from mock2, got success from %s", result.Source)
			}
		}
	}

	if errorCount != 1 || successCount != 1 {
		t.Errorf("Expected 1 error and 1 success, got %d errors and %d successes", errorCount, successCount)
	}
}

func TestEngine_CrawlMovie_Timeout(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Override timeout for this test
	engine.config.DefaultTimeout = 100 * time.Millisecond

	// Register slow mock crawler
	slowCrawler := &MockCrawler{name: "slow", delay: 200 * time.Millisecond}
	engine.RegisterCrawler("slow", slowCrawler)

	ctx := context.Background()
	results, err := engine.CrawlMovie(ctx, "TEST-123", "slow")
	if err != nil {
		t.Fatalf("CrawlMovie failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Error == nil {
		t.Error("Expected timeout error, got success")
	}
}

func TestEngine_CrawlBatch(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	mockCrawler := &MockCrawler{name: "mock"}
	engine.RegisterCrawler("mock", mockCrawler)

	movieIDs := []string{"TEST-001", "TEST-002", "TEST-003"}
	ctx := context.Background()
	
	results, err := engine.CrawlBatch(ctx, movieIDs, "mock")
	if err != nil {
		t.Fatalf("CrawlBatch failed: %v", err)
	}

	if len(results) != len(movieIDs) {
		t.Errorf("Expected %d results, got %d", len(movieIDs), len(results))
	}

	for _, movieID := range movieIDs {
		if _, exists := results[movieID]; !exists {
			t.Errorf("Missing result for movie ID: %s", movieID)
		}
	}
}

func TestEngine_GetStats(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	stats := engine.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	if stats.LastReset.IsZero() {
		t.Error("LastReset should be set")
	}

	if stats.CrawlerStats == nil {
		t.Error("CrawlerStats map should be initialized")
	}
}

func TestEngine_ResetStats(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Set some stats
	engine.stats.TotalRequests = 10
	engine.stats.SuccessfulCrawls = 8
	engine.stats.FailedCrawls = 2

	originalReset := engine.stats.LastReset

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	engine.ResetStats()

	stats := engine.GetStats()
	if stats.TotalRequests != 0 {
		t.Errorf("Expected TotalRequests to be 0, got %d", stats.TotalRequests)
	}
	if stats.SuccessfulCrawls != 0 {
		t.Errorf("Expected SuccessfulCrawls to be 0, got %d", stats.SuccessfulCrawls)
	}
	if stats.FailedCrawls != 0 {
		t.Errorf("Expected FailedCrawls to be 0, got %d", stats.FailedCrawls)
	}
	if !stats.LastReset.After(originalReset) {
		t.Error("LastReset should be updated")
	}
}

func TestEngine_IsHealthy(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Register healthy and unhealthy crawlers
	healthyCrawler := &MockCrawler{name: "healthy", shouldError: false}
	unhealthyCrawler := &MockCrawler{name: "unhealthy", shouldError: true}
	
	engine.RegisterCrawler("healthy", healthyCrawler)
	engine.RegisterCrawler("unhealthy", unhealthyCrawler)

	ctx := context.Background()
	if !engine.IsHealthy(ctx) {
		t.Error("Engine should be healthy when at least one crawler is available")
	}

	// Remove healthy crawler and test again
	engine.registry.crawlers = map[string]Crawler{"unhealthy": unhealthyCrawler}
	
	if engine.IsHealthy(ctx) {
		t.Error("Engine should be unhealthy when no crawlers are available")
	}
}

func TestEngine_shouldRetry(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("movie not found"), false},
		{errors.New("validation failed"), false},
		{errors.New("context canceled"), false},
		{errors.New("context deadline exceeded"), false},
		{errors.New("network error"), true},
		{errors.New("server error"), true},
	}

	for _, test := range tests {
		result := engine.shouldRetry(test.err)
		if result != test.expected {
			t.Errorf("shouldRetry(%v): expected %v, got %v", test.err, test.expected, result)
		}
	}
}

func BenchmarkEngine_CrawlMovie(b *testing.B) {
	engine, err := NewEngine(nil)
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	mockCrawler := &MockCrawler{name: "mock"}
	engine.RegisterCrawler("mock", mockCrawler)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.CrawlMovie(ctx, "TEST-123", "mock")
		if err != nil {
			b.Errorf("CrawlMovie failed: %v", err)
		}
	}
}