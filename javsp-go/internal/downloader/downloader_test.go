package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestImage creates a test image data
func createTestImage() []byte {
	// Simple JPEG header + minimal data
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xD9, // EOF
	}
}

func TestNewImageDownloader(t *testing.T) {
	downloader, err := NewImageDownloader(nil)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	if downloader.config == nil {
		t.Error("Expected default config to be set")
	}

	if downloader.client == nil {
		t.Error("Expected HTTP client to be created")
	}

	if downloader.stats == nil {
		t.Error("Expected stats to be initialized")
	}
}

func TestImageDownloader_Download_Success(t *testing.T) {
	testImage := createTestImage()
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testImage)))
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	// Create temporary directory
	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "test.jpg")

	// Create downloader
	config := DefaultDownloadConfig()
	config.MaxConcurrency = 1
	downloader, err := NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Download file
	ctx := context.Background()
	result, err := downloader.Download(ctx, server.URL, dstFile)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify result
	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}

	if result.Skipped {
		t.Error("Download should not be skipped")
	}

	if result.Size != int64(len(testImage)) {
		t.Errorf("Expected size %d, got %d", len(testImage), result.Size)
	}

	if result.ContentType != "image/jpeg" {
		t.Errorf("Expected content type 'image/jpeg', got '%s'", result.ContentType)
	}

	// Verify file was created
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}

	// Verify file content
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if len(content) != len(testImage) {
		t.Errorf("File content length mismatch: expected %d, got %d", len(testImage), len(content))
	}
}

func TestImageDownloader_Download_SkipExisting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server should not be called when file exists")
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "existing.jpg")

	// Create existing file
	testData := []byte("existing file content")
	if err := os.WriteFile(dstFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Create downloader with skip existing enabled
	config := DefaultDownloadConfig()
	config.SkipExisting = true
	downloader, err := NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Attempt download
	ctx := context.Background()
	result, err := downloader.Download(ctx, server.URL, dstFile)
	if err != nil {
		t.Errorf("Download should not fail when skipping: %v", err)
	}

	// Verify skipped
	if !result.Skipped {
		t.Error("Download should be skipped")
	}

	if result.SkipReason != "file already exists" {
		t.Errorf("Expected skip reason 'file already exists', got '%s'", result.SkipReason)
	}

	if result.Size != int64(len(testData)) {
		t.Errorf("Expected existing file size %d, got %d", len(testData), result.Size)
	}
}

func TestImageDownloader_Download_HTTPError(t *testing.T) {
	// Create server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "notfound.jpg")

	// Create downloader
	config := DefaultDownloadConfig()
	config.RetryAttempts = 1
	downloader, err := NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Attempt download
	ctx := context.Background()
	result, err := downloader.Download(ctx, server.URL, dstFile)

	// Should return error
	if err == nil {
		t.Error("Expected download to fail with 404")
	}

	if result.Error == nil {
		t.Error("Expected result to contain error")
	}

	if result.Skipped {
		t.Error("Download should not be marked as skipped")
	}

	// File should not exist
	if _, err := os.Stat(dstFile); !os.IsNotExist(err) {
		t.Error("Failed download should not create file")
	}
}

func TestImageDownloader_Download_InvalidContentType(t *testing.T) {
	// Create server that returns text/html
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Not an image</body></html>"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "notimage.jpg")

	// Create downloader
	downloader, err := NewImageDownloader(nil)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Attempt download
	ctx := context.Background()
	result, err := downloader.Download(ctx, server.URL, dstFile)

	// Should return error
	if err == nil {
		t.Error("Expected download to fail with invalid content type")
	}

	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("Expected content type error, got: %v", err)
	}

	if result.Error == nil {
		t.Error("Expected result to contain error")
	}
}

func TestImageDownloader_Download_FileTooLarge(t *testing.T) {
	// Create server with large content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", "999999999") // 999MB
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("large file"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "large.jpg")

	// Create downloader with small max file size
	config := DefaultDownloadConfig()
	config.MaxFileSize = 1024 // 1KB
	downloader, err := NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Attempt download
	ctx := context.Background()
	result, err := downloader.Download(ctx, server.URL, dstFile)

	// Should return error
	if err == nil {
		t.Error("Expected download to fail with file too large")
	}

	if !strings.Contains(err.Error(), "file too large") {
		t.Errorf("Expected file size error, got: %v", err)
	}

	if result.Error == nil {
		t.Error("Expected result to contain error")
	}
}

func TestImageDownloader_DownloadBatch(t *testing.T) {
	testImage := createTestImage()
	requestCount := 0
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	tempDir := t.TempDir()

	// Create batch download map
	downloads := map[string]string{
		server.URL + "/image1.jpg": filepath.Join(tempDir, "image1.jpg"),
		server.URL + "/image2.jpg": filepath.Join(tempDir, "image2.jpg"),
		server.URL + "/image3.jpg": filepath.Join(tempDir, "image3.jpg"),
	}

	// Create downloader
	config := DefaultDownloadConfig()
	config.MaxConcurrency = 2
	downloader, err := NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Download batch
	ctx := context.Background()
	results, err := downloader.DownloadBatch(ctx, downloads)
	if err != nil {
		t.Fatalf("Batch download failed: %v", err)
	}

	// Verify results
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Download failed for %s: %v", result.URL, result.Error)
		}
		
		if result.Skipped {
			t.Errorf("Download skipped unexpectedly for %s", result.URL)
		}

		if result.Size != int64(len(testImage)) {
			t.Errorf("Size mismatch for %s: expected %d, got %d", 
				result.URL, len(testImage), result.Size)
		}

		// Verify file exists
		if _, err := os.Stat(result.Filename); os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", result.Filename)
		}
	}

	// Verify all requests were made
	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}
}

func TestImageDownloader_ConcurrentDownloadLocking(t *testing.T) {
	testImage := createTestImage()
	requestCount := 0
	
	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		time.Sleep(100 * time.Millisecond) // Add delay to test concurrency
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "concurrent.jpg")

	downloader, err := NewImageDownloader(nil)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Start multiple downloads of the same URL
	ctx := context.Background()
	results := make(chan *DownloadResult, 3)

	for i := 0; i < 3; i++ {
		go func() {
			result, _ := downloader.Download(ctx, server.URL, dstFile)
			results <- result
		}()
	}

	// Collect results
	var allResults []*DownloadResult
	for i := 0; i < 3; i++ {
		result := <-results
		allResults = append(allResults, result)
	}

	// Only one should actually download, others should be skipped
	downloadCount := 0
	skipCount := 0
	
	for _, result := range allResults {
		if result.Skipped && result.SkipReason == "already downloading" {
			skipCount++
		} else if result.Error == nil {
			downloadCount++
		}
	}

	if downloadCount != 1 {
		t.Errorf("Expected exactly 1 download, got %d", downloadCount)
	}

	if skipCount != 2 {
		t.Errorf("Expected 2 skipped downloads, got %d", skipCount)
	}

	// Server should only receive one request
	if requestCount != 1 {
		t.Errorf("Expected 1 server request, got %d", requestCount)
	}
}

func TestImageDownloader_Stats(t *testing.T) {
	testImage := createTestImage()
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	tempDir := t.TempDir()

	downloader, err := NewImageDownloader(nil)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Initial stats should be zero
	stats := downloader.GetStats()
	if stats.TotalDownloads != 0 {
		t.Error("Initial stats should be zero")
	}

	// Download a file
	ctx := context.Background()
	dstFile := filepath.Join(tempDir, "stats.jpg")
	_, err = downloader.Download(ctx, server.URL, dstFile)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Check updated stats
	stats = downloader.GetStats()
	if stats.TotalDownloads != 1 {
		t.Errorf("Expected 1 total download, got %d", stats.TotalDownloads)
	}

	if stats.SuccessfulDownloads != 1 {
		t.Errorf("Expected 1 successful download, got %d", stats.SuccessfulDownloads)
	}

	if stats.BytesDownloaded != int64(len(testImage)) {
		t.Errorf("Expected %d bytes downloaded, got %d", len(testImage), stats.BytesDownloaded)
	}

	if stats.AverageSpeed <= 0 {
		t.Error("Average speed should be positive")
	}

	// Reset stats
	downloader.ResetStats()
	stats = downloader.GetStats()
	if stats.TotalDownloads != 0 {
		t.Error("Stats should be reset to zero")
	}
}

func TestGetFileExtensionFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"http://example.com/image.jpg", ".jpg"},
		{"http://example.com/image.png?param=1", ".png"},
		{"http://example.com/path/image.webp", ".webp"},
		{"http://example.com/noextension", ".jpg"}, // Default
		{"http://example.com/path/", ".jpg"},       // Default
	}

	for _, test := range tests {
		result := GetFileExtensionFromURL(test.url)
		if result != test.expected {
			t.Errorf("GetFileExtensionFromURL(%s): expected %s, got %s", 
				test.url, test.expected, result)
		}
	}
}

func TestGenerateFilename(t *testing.T) {
	url := "http://example.com/image.jpg"
	
	// Test without prefix
	filename1 := GenerateFilename(url, "")
	if !strings.HasSuffix(filename1, ".jpg") {
		t.Errorf("Generated filename should have .jpg extension: %s", filename1)
	}

	// Test with prefix
	filename2 := GenerateFilename(url, "cover")
	if !strings.HasPrefix(filename2, "cover_") {
		t.Errorf("Generated filename should start with prefix: %s", filename2)
	}

	if !strings.HasSuffix(filename2, ".jpg") {
		t.Errorf("Generated filename should have .jpg extension: %s", filename2)
	}

	// Same URL should generate same filename
	filename3 := GenerateFilename(url, "")
	if filename1 != filename3 {
		t.Errorf("Same URL should generate same filename: %s vs %s", filename1, filename3)
	}
}

func TestImageDownloader_ProgressCallback(t *testing.T) {
	testImage := createTestImage()
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testImage)))
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "progress.jpg")

	// Track progress calls
	var progressCalls []struct {
		downloaded, total int64
		filename          string
	}

	config := DefaultDownloadConfig()
	config.ProgressCallback = func(downloaded, total int64, filename string) {
		progressCalls = append(progressCalls, struct {
			downloaded, total int64
			filename          string
		}{downloaded, total, filename})
	}

	downloader, err := NewImageDownloader(config)
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	// Download with progress tracking
	ctx := context.Background()
	_, err = downloader.Download(ctx, server.URL, dstFile)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Should have received progress callbacks
	if len(progressCalls) == 0 {
		t.Error("Expected progress callbacks")
	}

	// Last callback should have total bytes
	lastCall := progressCalls[len(progressCalls)-1]
	if lastCall.downloaded != int64(len(testImage)) {
		t.Errorf("Expected final downloaded %d, got %d", len(testImage), lastCall.downloaded)
	}

	if lastCall.filename != dstFile {
		t.Errorf("Expected filename %s, got %s", dstFile, lastCall.filename)
	}
}

func BenchmarkImageDownloader_Download(b *testing.B) {
	testImage := createTestImage()
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testImage)
	}))
	defer server.Close()

	tempDir := b.TempDir()
	
	config := DefaultDownloadConfig()
	config.SkipExisting = false // Force download each time
	downloader, err := NewImageDownloader(config)
	if err != nil {
		b.Fatalf("Failed to create downloader: %v", err)
	}
	defer downloader.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dstFile := filepath.Join(tempDir, fmt.Sprintf("bench_%d.jpg", i))
		_, err := downloader.Download(ctx, server.URL, dstFile)
		if err != nil {
			b.Errorf("Download failed: %v", err)
		}
	}
}