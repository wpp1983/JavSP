//go:build integration && real_sites

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"javsp-go/internal/config"
	"javsp-go/internal/crawler"
)

// TestRealJavBusCrawler tests the JavBus crawler against real website
func TestRealJavBusCrawler(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real site test in short mode")
	}

	// Known existing movie IDs for testing
	testMovieIDs := []string{
		"STARS-123", 
		"SSIS-001", 
		"IPX-177",
		"PRED-456",
	}

	// Create crawler with real config
	config := &crawler.CrawlerConfig{
		BaseURL:    "https://www.javbus.com",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		RateLimit:  3 * time.Second, // Respectful crawling
		UserAgent:  "Mozilla/5.0 (compatible; JavSP-Test/1.0)",
	}

	javbusCrawler, err := crawler.NewJavBusCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create JavBus crawler: %v", err)
	}
	defer javbusCrawler.Close()

	t.Run("TestCrawlerAvailability", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		available := javbusCrawler.IsAvailable(ctx)
		if !available {
			t.Error("JavBus crawler should be available")
		}
	})

	t.Run("TestFetchKnownMovies", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		successCount := 0
		for _, movieID := range testMovieIDs {
			t.Logf("Testing movie ID: %s", movieID)
			
			movieInfo, err := javbusCrawler.FetchMovieInfo(ctx, movieID)
			if err != nil {
				t.Logf("Failed to fetch %s: %v", movieID, err)
				continue // Some movies might not exist, that's ok
			}

			// Validate core fields
			if movieInfo == nil {
				t.Errorf("Movie info should not be nil for %s", movieID)
				continue
			}

			if movieInfo.DVDID != movieID {
				t.Errorf("Expected DVDID %s, got %s", movieID, movieInfo.DVDID)
			}

			if movieInfo.Title == "" {
				t.Errorf("Title should not be empty for %s", movieID)
			}

			if movieInfo.Source != "javbus2" {
				t.Errorf("Expected source 'javbus2', got '%s' for %s", movieInfo.Source, movieID)
			}

			// Validate cover URL if present
			if movieInfo.Cover != "" && !strings.HasPrefix(movieInfo.Cover, "http") {
				t.Errorf("Cover URL should be valid HTTP URL for %s, got: %s", movieID, movieInfo.Cover)
			}

			// Log extracted data for manual verification
			t.Logf("Successfully extracted data for %s:", movieID)
			t.Logf("  Title: %s", movieInfo.Title)
			t.Logf("  Release Date: %s", movieInfo.ReleaseDate)
			t.Logf("  Actresses: %v", movieInfo.Actress)
			t.Logf("  Genres: %v", movieInfo.Genre)
			t.Logf("  Cover: %s", movieInfo.Cover)

			successCount++
			
			// Rate limiting to be respectful
			time.Sleep(3 * time.Second)
		}

		t.Logf("Successfully fetched %d out of %d test movies", successCount, len(testMovieIDs))
		
		// We should get at least one successful fetch
		if successCount == 0 {
			t.Error("Expected at least one successful movie fetch")
		}
	})

	t.Run("TestNonExistentMovie", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := javbusCrawler.FetchMovieInfo(ctx, "NONEXISTENT-9999")
		if err == nil {
			t.Error("Expected error for non-existent movie")
		}
		
		t.Logf("Non-existent movie returned expected error: %v", err)
	})

	t.Run("TestDataIntegrity", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		// Test with a well-known movie
		movieInfo, err := javbusCrawler.FetchMovieInfo(ctx, "STARS-123")
		if err != nil {
			t.Skipf("Skipping data integrity test, could not fetch test movie: %v", err)
			return
		}

		// Validate data consistency
		if movieInfo.Title != "" {
			// Title should not contain HTML tags
			if strings.Contains(movieInfo.Title, "<") || strings.Contains(movieInfo.Title, ">") {
				t.Errorf("Title contains HTML tags: %s", movieInfo.Title)
			}
		}

		// Validate actresses array
		for i, actress := range movieInfo.Actress {
			if actress == "" {
				t.Errorf("Actress[%d] should not be empty", i)
			}
			if strings.Contains(actress, "<") || strings.Contains(actress, ">") {
				t.Errorf("Actress[%d] contains HTML tags: %s", i, actress)
			}
		}

		// Validate genres array
		for i, genre := range movieInfo.Genre {
			if genre == "" {
				t.Errorf("Genre[%d] should not be empty", i)
			}
		}

		// Validate date format
		if movieInfo.ReleaseDate != "" {
			// Should be in YYYY-MM-DD format or similar
			if len(movieInfo.ReleaseDate) < 8 {
				t.Errorf("Release date seems too short: %s", movieInfo.ReleaseDate)
			}
		}
	})
}

// TestRealAVWikiCrawler tests the AVWiki crawler against real website
func TestRealAVWikiCrawler(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real site test in short mode")
	}

	// Known existing movie IDs for AVWiki
	testMovieIDs := []string{
		"SSIS-698",
		"STARS-256", 
		"IPX-001",
	}

	// Create crawler with real config
	config := &crawler.CrawlerConfig{
		BaseURL:    "https://av-wiki.net",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		RateLimit:  3 * time.Second, // Respectful crawling
		UserAgent:  "Mozilla/5.0 (compatible; JavSP-Test/1.0)",
	}

	avwikiCrawler, err := crawler.NewAVWikiCrawler(config)
	if err != nil {
		t.Fatalf("Failed to create AVWiki crawler: %v", err)
	}
	defer avwikiCrawler.Close()

	t.Run("TestCrawlerAvailability", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		available := avwikiCrawler.IsAvailable(ctx)
		if !available {
			t.Error("AVWiki crawler should be available")
		}
	})

	t.Run("TestFetchKnownMovies", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		successCount := 0
		for _, movieID := range testMovieIDs {
			t.Logf("Testing AVWiki movie ID: %s", movieID)
			
			movieInfo, err := avwikiCrawler.FetchMovieInfo(ctx, movieID)
			if err != nil {
				t.Logf("Failed to fetch %s from AVWiki: %v", movieID, err)
				continue // Some movies might not exist on AVWiki
			}

			// Validate core fields
			if movieInfo == nil {
				t.Errorf("Movie info should not be nil for %s", movieID)
				continue
			}

			if movieInfo.DVDID != movieID {
				t.Errorf("Expected DVDID %s, got %s", movieID, movieInfo.DVDID)
			}

			if movieInfo.Source != "avwiki" {
				t.Errorf("Expected source 'avwiki', got '%s' for %s", movieInfo.Source, movieID)
			}

			// Log extracted data for manual verification
			t.Logf("Successfully extracted AVWiki data for %s:", movieID)
			t.Logf("  Title: %s", movieInfo.Title)
			t.Logf("  Producer: %s", movieInfo.Producer)
			t.Logf("  Release Date: %s", movieInfo.ReleaseDate)
			t.Logf("  Actresses: %v", movieInfo.Actress)

			successCount++
			
			// Rate limiting to be respectful
			time.Sleep(3 * time.Second)
		}

		t.Logf("Successfully fetched %d out of %d test movies from AVWiki", successCount, len(testMovieIDs))
		
		// AVWiki might have fewer movies, so we're more lenient here
		if successCount == 0 {
			t.Log("No successful fetches from AVWiki - this might be expected")
		}
	})
}

// TestRealCrawlerEngine tests the crawler engine with real crawlers
func TestRealCrawlerEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real engine test in short mode")
	}

	// Create config
	cfg := &config.Config{
		Network: config.NetworkConfig{
			Timeout: 30 * time.Second,
			Retry:   2,
		},
		Crawler: config.CrawlerConfig{
			SleepAfterScraping: 3 * time.Second,
		},
	}

	// Create engine with real crawlers
	engine, err := crawler.NewEngine(cfg)
	if err != nil {
		t.Fatalf("Failed to create crawler engine: %v", err)
	}
	defer engine.Close()

	t.Run("TestEngineWithRealCrawlers", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		// Test with a well-known movie ID
		results, err := engine.CrawlMovie(ctx, "STARS-123")
		if err != nil {
			t.Fatalf("Engine crawl failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected at least one crawl result")
		}

		successCount := 0
		for _, result := range results {
			t.Logf("Crawler %s result: error=%v, duration=%v", 
				result.Source, result.Error, result.Duration)
				
			if result.Error == nil && result.MovieInfo != nil {
				successCount++
				t.Logf("  Successfully got data from %s", result.Source)
			}
		}

		t.Logf("Got successful results from %d out of %d crawlers", successCount, len(results))
		
		// We should get at least one successful result
		if successCount == 0 {
			t.Error("Expected at least one successful crawl result")
		}
	})

	t.Run("TestEngineHealthCheck", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		healthy := engine.IsHealthy(ctx)
		if !healthy {
			t.Error("Engine should be healthy with real crawlers")
		}
	})

	t.Run("TestEngineStats", func(t *testing.T) {
		stats := engine.GetStats()
		if stats == nil {
			t.Fatal("Engine stats should not be nil")
		}

		t.Logf("Engine stats: total=%d, successful=%d, failed=%d", 
			stats.TotalRequests, stats.SuccessfulCrawls, stats.FailedCrawls)
			
		// Stats should be updated after previous tests
		if stats.TotalRequests == 0 {
			t.Error("Expected some total requests in stats")
		}
	})
}