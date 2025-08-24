//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"javsp-go/internal/crawler"
	"javsp-go/internal/datatype"
	"javsp-go/test/testutils"
)

func TestCrawlerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock servers
	javbusServer := testutils.CreateJavBusTestServer()
	defer javbusServer.Close()
	
	avwikiServer := testutils.CreateAVWikiTestServer()
	defer avwikiServer.Close()

	// Create mock crawler factory
	factory := testutils.NewMockCrawlerFactory()
	
	// Create mock crawlers
	javbusCrawler := testutils.NewMockCrawler("javbus", javbusServer.URL())
	avwikiCrawler := testutils.NewMockCrawler("avwiki", avwikiServer.URL())
	
	// Set up test data
	generator := testutils.NewTestDataGenerator()
	testMovie := generator.GenerateMovieInfo("STARS-123")
	
	javbusCrawler.SetMovieData("STARS-123", testMovie)
	avwikiCrawler.SetMovieData("STARS-123", testMovie)
	
	factory.RegisterCrawler("javbus", javbusCrawler)
	factory.RegisterCrawler("avwiki", avwikiCrawler)

	t.Run("TestCrawlerRegistry", func(t *testing.T) {
		registry := crawler.NewCrawlerRegistry()
		
		// Register crawlers
		registry.Register("javbus", javbusCrawler)
		registry.Register("avwiki", avwikiCrawler)
		
		// Test getting crawlers
		if crawler, exists := registry.Get("javbus"); !exists {
			t.Error("Expected javbus crawler to exist")
		} else if crawler.Name() != "javbus" {
			t.Errorf("Expected name 'javbus', got '%s'", crawler.Name())
		}
		
		// Test getting all crawlers
		allCrawlers := registry.GetAll()
		if len(allCrawlers) != 2 {
			t.Errorf("Expected 2 crawlers, got %d", len(allCrawlers))
		}
		
		// Test cleanup
		if err := registry.Close(); err != nil {
			t.Errorf("Failed to close registry: %v", err)
		}
	})
	
	t.Run("TestCrawlerStats", func(t *testing.T) {
		registry := crawler.NewCrawlerRegistry()
		registry.Register("javbus", javbusCrawler)
		
		ctx := context.Background()
		
		// Fetch movie info to generate stats
		_, err := javbusCrawler.FetchMovieInfo(ctx, "STARS-123")
		if err != nil {
			t.Fatalf("Failed to fetch movie info: %v", err)
		}
		
		// Update stats manually for testing
		registry.UpdateStats("javbus", true, 100*time.Millisecond)
		
		stats, exists := registry.GetStats("javbus")
		if !exists {
			t.Fatal("Expected stats to exist")
		}
		
		if stats.Name != "javbus" {
			t.Errorf("Expected name 'javbus', got '%s'", stats.Name)
		}
		
		if stats.RequestCount == 0 {
			t.Error("Expected request count > 0")
		}
		
		if stats.SuccessCount == 0 {
			t.Error("Expected success count > 0")
		}
		
		if !stats.IsHealthy {
			t.Error("Expected crawler to be healthy")
		}
	})
	
	t.Run("TestCrawlerFailure", func(t *testing.T) {
		failingCrawler := testutils.NewMockCrawler("failing", "http://failing.test")
		failingCrawler.SetShouldFail(true)
		
		registry := crawler.NewCrawlerRegistry()
		registry.Register("failing", failingCrawler)
		
		ctx := context.Background()
		
		// Attempt to fetch, should fail
		_, err := failingCrawler.FetchMovieInfo(ctx, "UNKNOWN-123")
		if err == nil {
			t.Error("Expected error from failing crawler")
		}
		
		// Update stats with failure
		registry.UpdateStats("failing", false, 200*time.Millisecond)
		
		stats, exists := registry.GetStats("failing")
		if !exists {
			t.Fatal("Expected stats to exist")
		}
		
		if stats.ErrorCount == 0 {
			t.Error("Expected error count > 0")
		}
	})
	
	t.Run("TestCrawlerTimeout", func(t *testing.T) {
		slowCrawler := testutils.NewMockCrawler("slow", "http://slow.test")
		slowCrawler.SetDelay(2 * time.Second)
		slowCrawler.SetMovieData("SLOW-123", testMovie)
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		start := time.Now()
		_, err := slowCrawler.FetchMovieInfo(ctx, "SLOW-123")
		duration := time.Since(start)
		
		// Should timeout quickly due to context
		if err == nil {
			t.Error("Expected timeout error")
		}
		
		if duration > 200*time.Millisecond {
			t.Errorf("Expected quick timeout, took %v", duration)
		}
	})
	
	t.Run("TestCrawlerAvailability", func(t *testing.T) {
		availableCrawler := testutils.NewMockCrawler("available", "http://available.test")
		unavailableCrawler := testutils.NewMockCrawler("unavailable", "http://unavailable.test")
		unavailableCrawler.SetAvailable(false)
		
		ctx := context.Background()
		
		if !availableCrawler.IsAvailable(ctx) {
			t.Error("Available crawler should be available")
		}
		
		if unavailableCrawler.IsAvailable(ctx) {
			t.Error("Unavailable crawler should not be available")
		}
	})
}

func TestCrawlerEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock server
	server := testutils.CreateJavBusTestServer()
	defer server.Close()

	// Create mock crawler
	mockCrawler := testutils.NewMockCrawler("test", server.URL())
	generator := testutils.NewTestDataGenerator()
	
	testMovies := []string{"STARS-123", "SSIS-001", "IPX-177"}
	for _, movieID := range testMovies {
		movieInfo := generator.GenerateMovieInfo(movieID)
		mockCrawler.SetMovieData(movieID, movieInfo)
	}

	// Create registry and register crawler
	registry := crawler.NewCrawlerRegistry()
	registry.Register("test", mockCrawler)
	defer registry.Close()

	t.Run("TestEngineBasicFetch", func(t *testing.T) {
		ctx := context.Background()
		
		// Fetch movie info
		movieInfo, err := mockCrawler.FetchMovieInfo(ctx, "STARS-123")
		if err != nil {
			t.Fatalf("Failed to fetch movie info: %v", err)
		}
		
		if movieInfo == nil {
			t.Fatal("Movie info should not be nil")
		}
		
		if movieInfo.DVDID != "STARS-123" {
			t.Errorf("Expected DVDID 'STARS-123', got '%s'", movieInfo.DVDID)
		}
		
		if movieInfo.Title == "" {
			t.Error("Movie title should not be empty")
		}
	})
	
	t.Run("TestEngineSearch", func(t *testing.T) {
		// Set up search results
		searchResults := []*datatype.MovieInfo{
			generator.GenerateMovieInfo("STARS-001"),
			generator.GenerateMovieInfo("STARS-002"),
		}
		mockCrawler.SetSearchResults("STARS", searchResults)
		
		ctx := context.Background()
		results, err := mockCrawler.Search(ctx, "STARS")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		if len(results) != 2 {
			t.Errorf("Expected 2 search results, got %d", len(results))
		}
		
		for i, result := range results {
			if result.DVDID != searchResults[i].DVDID {
				t.Errorf("Expected DVDID '%s', got '%s'", searchResults[i].DVDID, result.DVDID)
			}
		}
	})
	
	t.Run("TestEngineNotFound", func(t *testing.T) {
		ctx := context.Background()
		
		_, err := mockCrawler.FetchMovieInfo(ctx, "NONEXISTENT-999")
		if err == nil {
			t.Error("Expected error for non-existent movie")
		}
	})
	
	t.Run("TestEngineEmptySearch", func(t *testing.T) {
		ctx := context.Background()
		
		results, err := mockCrawler.Search(ctx, "NONEXISTENT")
		if err != nil {
			t.Fatalf("Search should not fail for empty results: %v", err)
		}
		
		if len(results) != 0 {
			t.Errorf("Expected empty results, got %d", len(results))
		}
	})
}

func TestCrawlerConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("TestCrawlerConfig", func(t *testing.T) {
		config := &crawler.CrawlerConfig{
			BaseURL:          "https://test.example.com",
			Timeout:          10 * time.Second,
			MaxRetries:       3,
			RetryDelay:       1 * time.Second,
			RateLimit:        500 * time.Millisecond,
			UserAgent:        "Test Agent",
			EnableJavaScript: true,
		}
		
		if config.BaseURL != "https://test.example.com" {
			t.Errorf("Expected base URL 'https://test.example.com', got '%s'", config.BaseURL)
		}
		
		if config.Timeout != 10*time.Second {
			t.Errorf("Expected timeout 10s, got %v", config.Timeout)
		}
		
		if config.MaxRetries != 3 {
			t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
		}
	})
	
	t.Run("TestCrawlerTypes", func(t *testing.T) {
		crawler := testutils.NewMockCrawler("test", "https://test.com")
		
		supportedTypes := crawler.GetSupportedTypes()
		expectedTypes := []string{"normal", "amateur"}
		
		if len(supportedTypes) != len(expectedTypes) {
			t.Errorf("Expected %d supported types, got %d", len(expectedTypes), len(supportedTypes))
		}
		
		for i, expectedType := range expectedTypes {
			if supportedTypes[i] != expectedType {
				t.Errorf("Expected type '%s' at index %d, got '%s'", expectedType, i, supportedTypes[i])
			}
		}
	})
}

func BenchmarkCrawlerFetch(b *testing.B) {
	server := testutils.CreateJavBusTestServer()
	defer server.Close()
	
	crawler := testutils.NewMockCrawler("benchmark", server.URL())
	generator := testutils.NewTestDataGenerator()
	
	// Set up test data
	for i := 0; i < 100; i++ {
		movieID := fmt.Sprintf("TEST-%03d", i+1)
		movieInfo := generator.GenerateMovieInfo(movieID)
		crawler.SetMovieData(movieID, movieInfo)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			movieID := fmt.Sprintf("TEST-%03d", (i%100)+1)
			_, err := crawler.FetchMovieInfo(ctx, movieID)
			if err != nil {
				b.Fatalf("Fetch failed: %v", err)
			}
			i++
		}
	})
}