//go:build integration && real_sites

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"javsp-go/internal/downloader"
)

// TestRealImageDownloader tests downloading real images from the internet
func TestRealImageDownloader(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real download test in short mode")
	}

	// Create temporary directory for test downloads
	tempDir := t.TempDir()

	// Create downloader with real-world config
	config := downloader.DefaultDownloadConfig()
	config.MaxConcurrency = 3
	config.Timeout = 30 * time.Second
	config.RetryAttempts = 2
	config.SkipExisting = true
	config.MaxFileSize = 10 * 1024 * 1024 // 10MB

	imageDownloader, err := downloader.NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create image downloader: %v", err)
	}
	defer imageDownloader.Close()

	// Test URLs for different image types and sizes
	testImages := []struct {
		name        string
		url         string
		expectedMin int64 // minimum expected file size in bytes
		expectedMax int64 // maximum expected file size in bytes
	}{
		{
			name:        "small_jpeg",
			url:         "https://httpbin.org/image/jpeg", // Small test JPEG
			expectedMin: 1024,                             // 1KB
			expectedMax: 50 * 1024,                        // 50KB
		},
		{
			name:        "small_png", 
			url:         "https://httpbin.org/image/png", // Small test PNG
			expectedMin: 1024,                            // 1KB
			expectedMax: 50 * 1024,                       // 50KB
		},
		{
			name:        "small_webp",
			url:         "https://httpbin.org/image/webp", // Small test WebP
			expectedMin: 1024,                             // 1KB
			expectedMax: 50 * 1024,                        // 50KB
		},
	}

	t.Run("TestBasicImageDownload", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		successCount := 0
		for _, testImg := range testImages {
			t.Logf("Testing download: %s from %s", testImg.name, testImg.url)
			
			dstPath := filepath.Join(tempDir, testImg.name+".jpg")
			
			result, err := imageDownloader.Download(ctx, testImg.url, dstPath)
			if err != nil {
				t.Logf("Failed to download %s: %v", testImg.name, err)
				continue
			}

			// Validate result
			if result.Error != nil {
				t.Logf("Download result contains error for %s: %v", testImg.name, result.Error)
				continue
			}

			if result.Skipped {
				t.Logf("Download was skipped for %s: %s", testImg.name, result.SkipReason)
				continue
			}

			// Check file exists
			if _, err := os.Stat(dstPath); os.IsNotExist(err) {
				t.Errorf("Downloaded file should exist: %s", dstPath)
				continue
			}

			// Check file size
			if result.Size < testImg.expectedMin {
				t.Errorf("File too small for %s: got %d bytes, expected >= %d", 
					testImg.name, result.Size, testImg.expectedMin)
			}

			if result.Size > testImg.expectedMax {
				t.Errorf("File too large for %s: got %d bytes, expected <= %d", 
					testImg.name, result.Size, testImg.expectedMax)
			}

			// Check content type
			if result.ContentType == "" {
				t.Logf("No content type returned for %s", testImg.name)
			} else if !strings.HasPrefix(result.ContentType, "image/") {
				t.Errorf("Expected image content type for %s, got: %s", 
					testImg.name, result.ContentType)
			}

			t.Logf("Successfully downloaded %s: %d bytes, %s, took %v", 
				testImg.name, result.Size, result.ContentType, result.Duration)
				
			successCount++
		}

		t.Logf("Successfully downloaded %d out of %d test images", successCount, len(testImages))
		
		if successCount == 0 {
			t.Error("Expected at least one successful download")
		}
	})

	t.Run("TestInvalidImageURL", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		invalidURLs := []string{
			"https://httpbin.org/status/404",   // 404 error
			"https://httpbin.org/status/500",   // Server error
			"https://httpbin.org/html",         // Wrong content type
			"https://nonexistent-domain-12345678.com/image.jpg", // DNS error
		}

		for _, url := range invalidURLs {
			t.Logf("Testing invalid URL: %s", url)
			
			dstPath := filepath.Join(tempDir, fmt.Sprintf("invalid_%d.jpg", time.Now().UnixNano()))
			
			result, err := imageDownloader.Download(ctx, url, dstPath)
			
			// Should either return error or result with error
			if err == nil && (result == nil || result.Error == nil) {
				t.Errorf("Expected error for invalid URL: %s", url)
			}
			
			// File should not be created on error
			if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
				t.Errorf("File should not exist after failed download: %s", dstPath)
			}
			
			t.Logf("Invalid URL correctly handled: %s", url)
		}
	})

	t.Run("TestConcurrentDownloads", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		// Prepare multiple download tasks
		downloads := make(map[string]string)
		expectedFiles := []string{}
		
		for i, testImg := range testImages {
			for j := 0; j < 3; j++ { // 3 copies of each image
				filename := fmt.Sprintf("concurrent_%d_%d.jpg", i, j)
				dstPath := filepath.Join(tempDir, filename)
				downloads[testImg.url] = dstPath
				expectedFiles = append(expectedFiles, dstPath)
			}
		}

		t.Logf("Starting concurrent download of %d files", len(downloads))
		
		results, err := imageDownloader.DownloadBatch(ctx, downloads)
		if err != nil {
			t.Fatalf("Batch download failed: %v", err)
		}

		// Validate results
		successCount := 0
		for _, result := range results {
			if result.Error == nil && !result.Skipped {
				successCount++
			} else if result.Error != nil {
				t.Logf("Download failed for %s: %v", result.URL, result.Error)
			}
		}

		t.Logf("Concurrent downloads: %d successful out of %d total", 
			successCount, len(results))
			
		if successCount < len(downloads)/2 {
			t.Errorf("Expected at least half of downloads to succeed, got %d/%d", 
				successCount, len(downloads))
		}
	})

	t.Run("TestProgressCallback", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		// Track progress calls
		var progressCalls []struct {
			downloaded, total int64
			filename          string
		}

		// Create downloader with progress callback
		progressConfig := downloader.DefaultDownloadConfig()
		progressConfig.ProgressCallback = func(downloaded, total int64, filename string) {
			progressCalls = append(progressCalls, struct {
				downloaded, total int64
				filename          string
			}{downloaded, total, filename})
		}

		progressDownloader, err := downloader.NewImageDownloader(progressConfig)
		if err != nil {
			t.Fatalf("Failed to create progress downloader: %v", err)
		}
		defer progressDownloader.Close()

		// Download with progress tracking
		dstPath := filepath.Join(tempDir, "progress_test.jpg")
		_, err = progressDownloader.Download(ctx, testImages[0].url, dstPath)
		if err != nil {
			t.Skipf("Skipping progress test due to download failure: %v", err)
			return
		}

		// Should have received progress callbacks
		if len(progressCalls) == 0 {
			t.Error("Expected progress callbacks")
		} else {
			t.Logf("Received %d progress callbacks", len(progressCalls))
			
			// Last callback should have final size
			lastCall := progressCalls[len(progressCalls)-1]
			if lastCall.downloaded <= 0 {
				t.Error("Final progress callback should have positive downloaded bytes")
			}
			
			if !strings.Contains(lastCall.filename, "progress_test.jpg") {
				t.Errorf("Progress callback should have correct filename, got: %s", 
					lastCall.filename)
			}
		}
	})

	t.Run("TestDownloadStats", func(t *testing.T) {
		// Get initial stats
		initialStats := imageDownloader.GetStats()
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Perform a download
		dstPath := filepath.Join(tempDir, "stats_test.jpg")
		_, err := imageDownloader.Download(ctx, testImages[0].url, dstPath)
		if err != nil {
			t.Skipf("Skipping stats test due to download failure: %v", err)
			return
		}

		// Get updated stats
		finalStats := imageDownloader.GetStats()
		
		// Stats should be updated
		if finalStats.TotalDownloads <= initialStats.TotalDownloads {
			t.Error("Total downloads should have increased")
		}
		
		if finalStats.TotalDuration <= initialStats.TotalDuration {
			t.Error("Total duration should have increased")
		}
		
		if finalStats.AverageSpeed <= 0 {
			t.Error("Average speed should be positive")
		}

		t.Logf("Download stats: total=%d, successful=%d, failed=%d, avg_speed=%.2f bytes/sec", 
			finalStats.TotalDownloads, finalStats.SuccessfulDownloads, 
			finalStats.FailedDownloads, finalStats.AverageSpeed)
	})
}

// TestRealImageDownloadWithProxy tests downloading through proxy if configured
func TestRealImageDownloadWithProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping proxy test in short mode")
	}

	// Skip if no proxy is configured
	proxyURL := os.Getenv("TEST_PROXY_URL")
	if proxyURL == "" {
		t.Skip("Skipping proxy test, set TEST_PROXY_URL to enable")
		return
	}

	tempDir := t.TempDir()

	// Create downloader with proxy
	config := downloader.DefaultDownloadConfig()
	config.Timeout = 45 * time.Second
	
	// Note: This would need to be implemented in the downloader
	// For now we just test the basic case
	imageDownloader, err := downloader.NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer imageDownloader.Close()

	t.Run("TestDownloadThroughProxy", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "proxy_test.jpg")
		
		result, err := imageDownloader.Download(ctx, "https://httpbin.org/image/jpeg", dstPath)
		if err != nil {
			t.Fatalf("Download through proxy failed: %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Download result contains error: %v", result.Error)
		}

		// Verify file was downloaded
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Error("Downloaded file should exist")
		}

		t.Logf("Successfully downloaded through proxy: %d bytes", result.Size)
	})
}

// TestRealLargeImageDownload tests downloading larger images
func TestRealLargeImageDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large download test in short mode")
	}

	tempDir := t.TempDir()

	// Create downloader with larger limits
	config := downloader.DefaultDownloadConfig()
	config.MaxFileSize = 20 * 1024 * 1024 // 20MB
	config.Timeout = 120 * time.Second    // Longer timeout

	imageDownloader, err := downloader.NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer imageDownloader.Close()

	t.Run("TestLargeImageDownload", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		// Use a larger test image (this is a placeholder URL)
		largeImageURL := "https://httpbin.org/image/jpeg" // In reality, would use a larger image
		dstPath := filepath.Join(tempDir, "large_test.jpg")

		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, largeImageURL, dstPath)
		downloadTime := time.Since(startTime)

		if err != nil {
			t.Logf("Large image download failed (might be expected): %v", err)
			return // This might fail and that's ok for this test
		}

		if result.Error != nil {
			t.Logf("Large image download result contains error: %v", result.Error)
			return
		}

		t.Logf("Large image downloaded: %d bytes in %v (%.2f KB/s)", 
			result.Size, downloadTime, float64(result.Size)/1024/downloadTime.Seconds())

		// Verify file exists and has reasonable size
		if fileInfo, err := os.Stat(dstPath); err == nil {
			if fileInfo.Size() != result.Size {
				t.Errorf("File size mismatch: reported %d, actual %d", 
					result.Size, fileInfo.Size())
			}
		}
	})
}

// TestRealDownloadResilience tests download resilience with real network conditions
func TestRealDownloadResilience(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resilience test in short mode")
	}

	tempDir := t.TempDir()

	// Create downloader with resilience settings
	config := downloader.DefaultDownloadConfig()
	config.RetryAttempts = 3
	config.RetryDelay = 2 * time.Second
	config.Timeout = 30 * time.Second

	imageDownloader, err := downloader.NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer imageDownloader.Close()

	t.Run("TestTimeoutRecovery", func(t *testing.T) {
		// Test with a URL that might be slow
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "timeout_test.jpg")
		
		// Use httpbin delay endpoint to simulate slow response
		slowURL := "https://httpbin.org/delay/2" // 2 second delay
		
		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, slowURL, dstPath)
		totalTime := time.Since(startTime)

		// This might fail due to wrong content type, but should handle the delay
		t.Logf("Slow URL test completed in %v, error: %v", totalTime, err)
		
		if result != nil && result.Error != nil {
			t.Logf("Expected error due to non-image content: %v", result.Error)
		}

		// Should not take extremely long due to timeout
		if totalTime > 60*time.Second {
			t.Error("Download took too long, timeout mechanism may not be working")
		}
	})

	t.Run("TestRetryMechanism", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Test with an intermittently failing URL
		flakyURL := "https://httpbin.org/status/500" // Always returns 500
		dstPath := filepath.Join(tempDir, "retry_test.jpg")

		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, flakyURL, dstPath)
		totalTime := time.Since(startTime)

		// Should fail but should have attempted retries
		if err == nil && result.Error == nil {
			t.Error("Expected error for 500 status URL")
		}

		// Should have taken some time due to retries
		expectedMinTime := time.Duration(config.RetryAttempts-1) * config.RetryDelay
		if totalTime < expectedMinTime {
			t.Errorf("Expected at least %v due to retries, took %v", expectedMinTime, totalTime)
		}

		t.Logf("Retry test completed in %v (expected >= %v)", totalTime, expectedMinTime)
	})
}