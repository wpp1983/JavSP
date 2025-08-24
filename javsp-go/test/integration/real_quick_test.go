//go:build integration && real_sites

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"javsp-go/internal/crawler"
	"javsp-go/internal/downloader"
	"os"
	"path/filepath"
)

// TestQuickRealSites performs quick tests on real sites
func TestQuickRealSites(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real site test in short mode")
	}

	t.Run("TestJavBusAvailability", func(t *testing.T) {
		config := &crawler.CrawlerConfig{
			BaseURL:    "https://www.javbus.com",
			Timeout:    10 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
			RateLimit:  1 * time.Second,
			UserAgent:  "Mozilla/5.0 (compatible; JavSP-Test/1.0)",
		}

		javbusCrawler, err := crawler.NewJavBusCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create JavBus crawler: %v", err)
		}
		defer javbusCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		available := javbusCrawler.IsAvailable(ctx)
		t.Logf("JavBus availability: %v", available)
		
		if !available {
			t.Log("JavBus is not available - this might be due to network issues or site maintenance")
		}
	})

	t.Run("TestAVWikiAvailability", func(t *testing.T) {
		config := &crawler.CrawlerConfig{
			BaseURL:    "https://av-wiki.net",
			Timeout:    10 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
			RateLimit:  1 * time.Second,
			UserAgent:  "Mozilla/5.0 (compatible; JavSP-Test/1.0)",
		}

		avwikiCrawler, err := crawler.NewAVWikiCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create AVWiki crawler: %v", err)
		}
		defer avwikiCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		available := avwikiCrawler.IsAvailable(ctx)
		t.Logf("AVWiki availability: %v", available)
		
		if !available {
			t.Log("AVWiki is not available - this might be due to network issues or site maintenance")
		}
	})

	t.Run("TestQuickImageDownload", func(t *testing.T) {
		tempDir := t.TempDir()
		
		config := downloader.DefaultDownloadConfig()
		config.MaxConcurrency = 1
		config.Timeout = 10 * time.Second
		config.RetryAttempts = 1

		imageDownloader, err := downloader.NewImageDownloader(config)
		if err != nil {
			t.Fatalf("Failed to create image downloader: %v", err)
		}
		defer imageDownloader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "test_download.jpg")
		
		result, err := imageDownloader.Download(ctx, "https://httpbin.org/image/jpeg", dstPath)
		if err != nil {
			t.Logf("Download failed: %v", err)
			return
		}

		if result.Error != nil {
			t.Logf("Download result contains error: %v", result.Error)
			return
		}

		if result.Skipped {
			t.Logf("Download was skipped: %s", result.SkipReason)
			return
		}

		// Check file exists
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Error("Downloaded file should exist")
		} else {
			t.Logf("Successfully downloaded %d bytes to %s", result.Size, dstPath)
		}
	})

	t.Run("TestMovieFetch", func(t *testing.T) {
		// Only test if JavBus is available first
		config := &crawler.CrawlerConfig{
			BaseURL:    "https://www.javbus.com",
			Timeout:    15 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
			RateLimit:  2 * time.Second,
			UserAgent:  "Mozilla/5.0 (compatible; JavSP-Test/1.0)",
		}

		javbusCrawler, err := crawler.NewJavBusCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create JavBus crawler: %v", err)
		}
		defer javbusCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Test with a well-known movie ID
		movieInfo, err := javbusCrawler.FetchMovieInfo(ctx, "STARS-123")
		if err != nil {
			t.Logf("Failed to fetch STARS-123: %v", err)
			
			// Try another ID
			movieInfo, err = javbusCrawler.FetchMovieInfo(ctx, "SSIS-001")
			if err != nil {
				t.Logf("Failed to fetch SSIS-001: %v", err)
				t.Skip("Skipping movie fetch test - both test IDs failed")
				return
			}
		}

		if movieInfo == nil {
			t.Error("Movie info should not be nil")
			return
		}

		// Basic validation
		if movieInfo.DVDID == "" {
			t.Error("DVDID should not be empty")
		}

		if movieInfo.Title != "" {
			// Title should not contain HTML tags
			if strings.Contains(movieInfo.Title, "<") || strings.Contains(movieInfo.Title, ">") {
				t.Errorf("Title contains HTML tags: %s", movieInfo.Title)
			}
		}

		if movieInfo.Source != "javbus2" {
			t.Errorf("Expected source 'javbus2', got '%s'", movieInfo.Source)
		}

		t.Logf("Successfully fetched movie info:")
		t.Logf("  ID: %s", movieInfo.DVDID)
		t.Logf("  Title: %s", movieInfo.Title)
		t.Logf("  Release Date: %s", movieInfo.ReleaseDate)
		t.Logf("  Actresses: %v", movieInfo.Actress)
		t.Logf("  Genres: %v", movieInfo.Genre)
		t.Logf("  Cover: %s", movieInfo.Cover)
	})
}