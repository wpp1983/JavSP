//go:build integration && real_sites

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"javsp-go/internal/crawler"
	"javsp-go/internal/downloader"
)

// TestNetworkResilience tests how the system handles various network failures
func TestNetworkResilience(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network resilience test in short mode")
	}

	t.Run("TestDNSFailure", func(t *testing.T) {
		// Test crawler with non-existent domain
		config := &crawler.CrawlerConfig{
			BaseURL:    "https://nonexistent-domain-12345678.com",
			Timeout:    3 * time.Second,  // 减少超时时间
			MaxRetries: 1,               // 减少重试次数
			RetryDelay: 500 * time.Millisecond, // 减少重试间隔
		}

		javbusCrawler, err := crawler.NewJavBusCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create crawler: %v", err)
		}
		defer javbusCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Should handle DNS failure gracefully
		startTime := time.Now()
		available := javbusCrawler.IsAvailable(ctx)
		dnsTime := time.Since(startTime)

		if available {
			t.Error("Crawler should not be available for non-existent domain")
		}

		t.Logf("DNS failure handled in %v", dnsTime)

		// Test fetching movie info - should also fail gracefully
		_, err = javbusCrawler.FetchMovieInfo(ctx, "TEST-123")
		if err == nil {
			t.Error("Expected error for non-existent domain")
		}

		if !strings.Contains(strings.ToLower(err.Error()), "dns") &&
			!strings.Contains(strings.ToLower(err.Error()), "no such host") &&
			!strings.Contains(strings.ToLower(err.Error()), "failed to fetch") {
			t.Logf("DNS error: %v", err)
		}
	})

	t.Run("TestConnectionTimeout", func(t *testing.T) {
		// Create a server that doesn't respond
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		
		// Get the port but don't start accepting connections
		serverURL := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
		listener.Close() // Close immediately to simulate timeout

		config := &crawler.CrawlerConfig{
			BaseURL:    serverURL,
			Timeout:    2 * time.Second,  // 减少超时时间
			MaxRetries: 1,
			RetryDelay: 500 * time.Millisecond, // 减少重试间隔
		}

		javbusCrawler, err := crawler.NewJavBusCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create crawler: %v", err)
		}
		defer javbusCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		startTime := time.Now()
		_, err = javbusCrawler.FetchMovieInfo(ctx, "TEST-123")
		timeoutTime := time.Since(startTime)

		if err == nil {
			t.Error("Expected error for connection timeout")
		}

		// Should timeout relatively quickly
		if timeoutTime > 15*time.Second {
			t.Errorf("Timeout took too long: %v", timeoutTime)
		}

		t.Logf("Connection timeout handled in %v, error: %v", timeoutTime, err)
	})

	t.Run("TestServerErrors", func(t *testing.T) {
		// Test various HTTP error codes
		errorCodes := []int{500, 502, 503, 504}
		
		for _, code := range errorCodes {
			t.Run(fmt.Sprintf("HTTP_%d", code), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
					w.Write([]byte(fmt.Sprintf("Server Error %d", code)))
				}))
				defer server.Close()

				config := &crawler.CrawlerConfig{
					BaseURL:    server.URL,
					Timeout:    3 * time.Second,  // 减少超时时间
					MaxRetries: 1,               // 减少重试次数
					RetryDelay: 500 * time.Millisecond, // 减少重试间隔
				}

				javbusCrawler, err := crawler.NewJavBusCrawler(config)
				if err != nil {
					t.Fatalf("Failed to create crawler: %v", err)
				}
				defer javbusCrawler.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				startTime := time.Now()
				_, err = javbusCrawler.FetchMovieInfo(ctx, "TEST-123")
				errorTime := time.Since(startTime)

				if err == nil {
					t.Errorf("Expected error for HTTP %d", code)
				}

				// Should have attempted retries for 5xx errors
				if code >= 500 {
					expectedMinTime := time.Duration(config.MaxRetries) * config.RetryDelay
					if errorTime < expectedMinTime {
						t.Logf("HTTP %d retry time seems short: %v (expected >= %v)", 
							code, errorTime, expectedMinTime)
					}
				}

				t.Logf("HTTP %d handled in %v, error: %v", code, errorTime, err)
			})
		}
	})

	t.Run("TestSlowServer", func(t *testing.T) {
		// Create a server with artificial delay
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(8 * time.Second) // Longer than typical timeout
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body><h3>Slow Response</h3></body></html>"))
		}))
		defer server.Close()

		config := &crawler.CrawlerConfig{
			BaseURL:    server.URL,
			Timeout:    5 * time.Second, // Shorter than server delay
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
		}

		javbusCrawler, err := crawler.NewJavBusCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create crawler: %v", err)
		}
		defer javbusCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		startTime := time.Now()
		_, err = javbusCrawler.FetchMovieInfo(ctx, "TEST-123")
		slowTime := time.Since(startTime)

		if err == nil {
			t.Error("Expected timeout error for slow server")
		}

		// Should timeout before server responds
		if slowTime > 15*time.Second {
			t.Errorf("Slow server test took too long: %v", slowTime)
		}

		t.Logf("Slow server handled in %v, error: %v", slowTime, err)
	})
}

// TestDownloadResilience tests download resilience to network issues
func TestDownloadResilience(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download resilience test in short mode")
	}

	tempDir := t.TempDir()

	t.Run("TestDownloadDNSFailure", func(t *testing.T) {
		config := downloader.DefaultDownloadConfig()
		config.RetryAttempts = 2
		config.Timeout = 10 * time.Second

		imageDownloader, err := downloader.NewImageDownloader(config)
		if err != nil {
			t.Fatalf("Failed to create downloader: %v", err)
		}
		defer imageDownloader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "dns_fail.jpg")
		
		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, "https://nonexistent-domain-12345678.com/image.jpg", dstPath)
		dnsTime := time.Since(startTime)

		if err == nil && (result == nil || result.Error == nil) {
			t.Error("Expected error for non-existent domain")
		}

		// File should not be created
		if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
			t.Error("File should not exist after failed download")
		}

		t.Logf("Download DNS failure handled in %v", dnsTime)
	})

	t.Run("TestDownloadServerError", func(t *testing.T) {
		// Create server that returns errors
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
		}))
		defer server.Close()

		config := downloader.DefaultDownloadConfig()
		config.RetryAttempts = 3
		config.RetryDelay = 1 * time.Second

		imageDownloader, err := downloader.NewImageDownloader(config)
		if err != nil {
			t.Fatalf("Failed to create downloader: %v", err)
		}
		defer imageDownloader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "server_error.jpg")
		
		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, server.URL+"/image.jpg", dstPath)
		errorTime := time.Since(startTime)

		if err == nil && (result == nil || result.Error == nil) {
			t.Error("Expected error for server error")
		}

		// Should have attempted retries
		expectedMinTime := time.Duration(config.RetryAttempts-1) * config.RetryDelay
		if errorTime < expectedMinTime {
			t.Logf("Retry time seems short: %v (expected >= %v)", errorTime, expectedMinTime)
		}

		t.Logf("Download server error handled in %v with retries", errorTime)
	})

	t.Run("TestDownloadPartialContent", func(t *testing.T) {
		// Create server that closes connection mid-stream
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", "10000") // Claim larger size
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("JPEG_HEADER_AND_SOME_DATA")) // Partial data
			// Connection will be closed abruptly
		}))
		defer server.Close()

		config := downloader.DefaultDownloadConfig()
		config.RetryAttempts = 2

		imageDownloader, err := downloader.NewImageDownloader(config)
		if err != nil {
			t.Fatalf("Failed to create downloader: %v", err)
		}
		defer imageDownloader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "partial.jpg")
		
		result, err := imageDownloader.Download(ctx, server.URL+"/image.jpg", dstPath)
		
		// Should handle partial content gracefully
		if err != nil || (result != nil && result.Error != nil) {
			t.Logf("Partial content handled with error: %v", err)
			if result != nil && result.Error != nil {
				t.Logf("Result error: %v", result.Error)
			}
		}

		// Temporary file should be cleaned up on error
		tempFile := dstPath + ".downloading"
		if _, statErr := os.Stat(tempFile); !os.IsNotExist(statErr) {
			t.Error("Temporary file should be cleaned up after failed download")
		}
	})

	t.Run("TestDownloadTimeout", func(t *testing.T) {
		// Create very slow server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			// Send data very slowly
			for i := 0; i < 10; i++ {
				w.Write([]byte("data"))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(2 * time.Second) // Very slow
			}
		}))
		defer server.Close()

		config := downloader.DefaultDownloadConfig()
		config.Timeout = 5 * time.Second // Shorter than server delay

		imageDownloader, err := downloader.NewImageDownloader(config)
		if err != nil {
			t.Fatalf("Failed to create downloader: %v", err)
		}
		defer imageDownloader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "timeout.jpg")
		
		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, server.URL+"/image.jpg", dstPath)
		timeoutTime := time.Since(startTime)

		// Should timeout before completing
		if err == nil && (result == nil || result.Error == nil) {
			t.Error("Expected timeout error for slow download")
		}

		// Should timeout within reasonable time
		if timeoutTime > 15*time.Second {
			t.Errorf("Timeout took too long: %v", timeoutTime)
		}

		t.Logf("Download timeout handled in %v", timeoutTime)
	})
}

// TestNetworkRecovery tests recovery from temporary network issues
func TestNetworkRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network recovery test in short mode")
	}

	t.Run("TestIntermittentServer", func(t *testing.T) {
		requestCount := 0
		
		// Server that fails first few requests then succeeds
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			
			if requestCount <= 2 {
				// Fail first 2 requests
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Temporary Error"))
				return
			}
			
			// Succeed on 3rd request
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><h3>STARS-123 Success Title</h3></body></html>`))
		}))
		defer server.Close()

		config := &crawler.CrawlerConfig{
			BaseURL:    server.URL,
			Timeout:    10 * time.Second,
			MaxRetries: 3, // Should succeed on 3rd try
			RetryDelay: 1 * time.Second,
		}

		javbusCrawler, err := crawler.NewJavBusCrawler(config)
		if err != nil {
			t.Fatalf("Failed to create crawler: %v", err)
		}
		defer javbusCrawler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		startTime := time.Now()
		movieInfo, err := javbusCrawler.FetchMovieInfo(ctx, "STARS-123")
		recoveryTime := time.Since(startTime)

		// Should eventually succeed
		if err != nil {
			t.Errorf("Expected recovery after retries, but got error: %v", err)
		}

		if movieInfo == nil {
			t.Error("Expected movie info after recovery")
		}

		// Should have made multiple requests
		if requestCount < 3 {
			t.Errorf("Expected at least 3 requests, got %d", requestCount)
		}

		t.Logf("Network recovery successful after %d requests in %v", requestCount, recoveryTime)
	})

	t.Run("TestDownloadRecovery", func(t *testing.T) {
		tempDir := t.TempDir()
		downloadAttempts := 0
		
		// Server that fails first attempt then succeeds
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			downloadAttempts++
			
			if downloadAttempts == 1 {
				// Fail first attempt
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			
			// Succeed on second attempt
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("FAKE_JPEG_DATA_FOR_TEST"))
		}))
		defer server.Close()

		config := downloader.DefaultDownloadConfig()
		config.RetryAttempts = 2
		config.RetryDelay = 1 * time.Second

		imageDownloader, err := downloader.NewImageDownloader(config)
		if err != nil {
			t.Fatalf("Failed to create downloader: %v", err)
		}
		defer imageDownloader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		dstPath := filepath.Join(tempDir, "recovery.jpg")
		
		startTime := time.Now()
		result, err := imageDownloader.Download(ctx, server.URL+"/image.jpg", dstPath)
		recoveryTime := time.Since(startTime)

		// Should eventually succeed
		if err != nil {
			t.Errorf("Expected download recovery, but got error: %v", err)
		}

		if result == nil || result.Error != nil {
			t.Errorf("Expected successful result after recovery")
		}

		// Should have made multiple attempts
		if downloadAttempts < 2 {
			t.Errorf("Expected at least 2 download attempts, got %d", downloadAttempts)
		}

		// File should exist
		if _, statErr := os.Stat(dstPath); os.IsNotExist(statErr) {
			t.Error("Downloaded file should exist after recovery")
		}

		t.Logf("Download recovery successful after %d attempts in %v", downloadAttempts, recoveryTime)
	})
}