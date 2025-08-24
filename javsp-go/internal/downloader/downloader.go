package downloader

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"javsp-go/internal/errors"
	"javsp-go/pkg/web"
)

// DownloadResult represents the result of a download operation
type DownloadResult struct {
	URL          string        `json:"url"`
	Filename     string        `json:"filename"`
	Size         int64         `json:"size"`
	ContentType  string        `json:"content_type"`
	Duration     time.Duration `json:"duration"`
	Error        error         `json:"error,omitempty"`
	Skipped      bool          `json:"skipped,omitempty"`
	SkipReason   string        `json:"skip_reason,omitempty"`
}

// ProgressCallback is called during download progress
type ProgressCallback func(downloaded, total int64, filename string)

// DownloadConfig contains configuration for the downloader
type DownloadConfig struct {
	MaxConcurrency    int                    `json:"max_concurrency"`
	Timeout           time.Duration          `json:"timeout"`
	RetryAttempts     int                    `json:"retry_attempts"`
	RetryDelay        time.Duration          `json:"retry_delay"`
	MaxFileSize       int64                  `json:"max_file_size"`
	AllowedTypes      []string               `json:"allowed_types"`
	UserAgent         string                 `json:"user_agent"`
	Headers           map[string]string      `json:"headers"`
	SkipExisting      bool                   `json:"skip_existing"`
	ResumePartial     bool                   `json:"resume_partial"`
	ProgressCallback  ProgressCallback       `json:"-"`
}

// DefaultDownloadConfig returns a default configuration
func DefaultDownloadConfig() *DownloadConfig {
	return &DownloadConfig{
		MaxConcurrency: 3,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    2 * time.Second,
		MaxFileSize:   50 * 1024 * 1024, // 50MB
		AllowedTypes: []string{
			"image/jpeg", "image/jpg", "image/png", "image/webp",
			"image/bmp", "image/gif", "image/tiff",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Headers: map[string]string{
			"Accept":          "image/*,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.5",
			"Accept-Encoding": "gzip, deflate, br",
			"DNT":             "1",
			"Connection":      "keep-alive",
		},
		SkipExisting:  true,
		ResumePartial: true,
	}
}

// ImageDownloader handles downloading images with advanced features
type ImageDownloader struct {
	client     *web.Client
	config     *DownloadConfig
	activeJobs map[string]bool
	jobMutex   sync.RWMutex
	stats      *DownloadStats
}

// DownloadStats tracks download statistics
type DownloadStats struct {
	TotalDownloads     int64         `json:"total_downloads"`
	SuccessfulDownloads int64         `json:"successful_downloads"`
	FailedDownloads    int64         `json:"failed_downloads"`
	SkippedDownloads   int64         `json:"skipped_downloads"`
	BytesDownloaded    int64         `json:"bytes_downloaded"`
	TotalDuration      time.Duration `json:"total_duration"`
	AverageSpeed       float64       `json:"average_speed"` // bytes per second
}

// NewImageDownloader creates a new image downloader
func NewImageDownloader(config *DownloadConfig) (*ImageDownloader, error) {
	if config == nil {
		config = DefaultDownloadConfig()
	}

	clientOpts := &web.ClientOptions{
		Timeout:       config.Timeout,
		EnableCookies: true,
		SkipTLSVerify: true,
		UserAgents:    []string{config.UserAgent},
	}

	client, err := web.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &ImageDownloader{
		client:     client,
		config:     config,
		activeJobs: make(map[string]bool),
		stats:      &DownloadStats{},
	}, nil
}

// Download downloads a single image from URL to destination
func (d *ImageDownloader) Download(ctx context.Context, url, dst string) (*DownloadResult, error) {
	start := time.Now()
	result := &DownloadResult{
		URL:      url,
		Filename: dst,
	}

	// Check if already downloading
	if !d.tryLockDownload(url) {
		result.Skipped = true
		result.SkipReason = "already downloading"
		return result, nil
	}
	defer d.unlockDownload(url)

	// Check if file already exists and skip if configured
	if d.config.SkipExisting {
		if info, err := os.Stat(dst); err == nil && info.Size() > 0 {
			result.Skipped = true
			result.SkipReason = "file already exists"
			result.Size = info.Size()
			return result, nil
		}
	}

	// Attempt download with retries
	var lastErr error
	for attempt := 0; attempt <= d.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				return result, ctx.Err()
			case <-time.After(d.config.RetryDelay):
			}
		}

		err := d.downloadFile(ctx, url, dst, result)
		if err == nil {
			result.Duration = time.Since(start)
			d.updateStats(result, true)
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if crawlerErr := errors.ClassifyError("downloader", err); !crawlerErr.IsRetryable() {
			break
		}
	}

	result.Error = lastErr
	result.Duration = time.Since(start)
	d.updateStats(result, false)
	return result, lastErr
}

// downloadFile performs the actual download
func (d *ImageDownloader) downloadFile(ctx context.Context, url, dst string, result *DownloadResult) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range d.config.Headers {
		req.Header.Set(key, value)
	}

	// Check for partial download support
	var resumeOffset int64
	tempFile := dst + ".downloading"
	
	if d.config.ResumePartial {
		if info, err := os.Stat(tempFile); err == nil {
			resumeOffset = info.Size()
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
		}
	}

	// Perform request
	resp, err := d.client.DoWithRetry(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Validate response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !d.isAllowedContentType(contentType) {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}

	// Check content length
	contentLength := resp.ContentLength
	if contentLength > 0 && contentLength > d.config.MaxFileSize {
		return fmt.Errorf("file too large: %d bytes (max: %d)", contentLength, d.config.MaxFileSize)
	}

	// Open destination file
	var file *os.File
	if resumeOffset > 0 && resp.StatusCode == 206 { // Partial content
		file, err = os.OpenFile(tempFile, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(tempFile)
		resumeOffset = 0
	}
	
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer file.Close()

	// Download with progress tracking
	totalSize := resumeOffset + contentLength
	downloaded := resumeOffset
	
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			os.Remove(tempFile)
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				os.Remove(tempFile)
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			
			downloaded += int64(n)
			
			// Call progress callback
			if d.config.ProgressCallback != nil {
				d.config.ProgressCallback(downloaded, totalSize, dst)
			}
		}
		
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tempFile)
			return fmt.Errorf("download interrupted: %w", err)
		}
	}

	// Finalize download
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Move temp file to final destination
	if err := os.Rename(tempFile, dst); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to finalize file: %w", err)
	}

	// Update result
	result.Size = downloaded
	result.ContentType = contentType

	return nil
}

// DownloadBatch downloads multiple images concurrently
func (d *ImageDownloader) DownloadBatch(ctx context.Context, downloads map[string]string) ([]*DownloadResult, error) {
	if len(downloads) == 0 {
		return []*DownloadResult{}, nil
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, d.config.MaxConcurrency)
	results := make(chan *DownloadResult, len(downloads))
	var wg sync.WaitGroup

	// Start download workers
	for url, dst := range downloads {
		wg.Add(1)
		go func(u, dstPath string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, _ := d.Download(ctx, u, dstPath)
			results <- result
		}(url, dst)
	}

	// Wait for all downloads to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []*DownloadResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults, nil
}

// isAllowedContentType checks if content type is allowed
func (d *ImageDownloader) isAllowedContentType(contentType string) bool {
	if len(d.config.AllowedTypes) == 0 {
		return true
	}

	// Normalize content type
	contentType = strings.ToLower(strings.Split(contentType, ";")[0])

	for _, allowed := range d.config.AllowedTypes {
		if strings.ToLower(allowed) == contentType {
			return true
		}
	}
	return false
}

// tryLockDownload attempts to lock a URL for downloading
func (d *ImageDownloader) tryLockDownload(url string) bool {
	d.jobMutex.Lock()
	defer d.jobMutex.Unlock()

	if d.activeJobs[url] {
		return false
	}
	d.activeJobs[url] = true
	return true
}

// unlockDownload unlocks a URL after downloading
func (d *ImageDownloader) unlockDownload(url string) {
	d.jobMutex.Lock()
	defer d.jobMutex.Unlock()
	delete(d.activeJobs, url)
}

// updateStats updates download statistics
func (d *ImageDownloader) updateStats(result *DownloadResult, success bool) {
	d.stats.TotalDownloads++
	
	if result.Skipped {
		d.stats.SkippedDownloads++
	} else if success {
		d.stats.SuccessfulDownloads++
		d.stats.BytesDownloaded += result.Size
	} else {
		d.stats.FailedDownloads++
	}
	
	d.stats.TotalDuration += result.Duration
	
	// Calculate average speed
	if d.stats.TotalDuration > 0 {
		d.stats.AverageSpeed = float64(d.stats.BytesDownloaded) / d.stats.TotalDuration.Seconds()
	}
}

// GetStats returns current download statistics
func (d *ImageDownloader) GetStats() *DownloadStats {
	// Return a copy to avoid data races
	return &DownloadStats{
		TotalDownloads:      d.stats.TotalDownloads,
		SuccessfulDownloads: d.stats.SuccessfulDownloads,
		FailedDownloads:     d.stats.FailedDownloads,
		SkippedDownloads:    d.stats.SkippedDownloads,
		BytesDownloaded:     d.stats.BytesDownloaded,
		TotalDuration:       d.stats.TotalDuration,
		AverageSpeed:        d.stats.AverageSpeed,
	}
}

// ResetStats resets all download statistics
func (d *ImageDownloader) ResetStats() {
	d.stats = &DownloadStats{}
}

// Close cleans up the downloader
func (d *ImageDownloader) Close() error {
	return d.client.Close()
}

// GetFileExtensionFromURL extracts file extension from URL
func GetFileExtensionFromURL(url string) string {
	// Try to get extension from URL path
	path := strings.Split(url, "?")[0] // Remove query parameters
	if ext := filepath.Ext(path); ext != "" {
		return ext
	}
	
	// Default to .jpg if no extension found
	return ".jpg"
}

// GenerateFilename generates a filename from URL and optional prefix
func GenerateFilename(url, prefix string) string {
	// Create a hash of the URL for uniqueness
	hash := md5.Sum([]byte(url))
	hashStr := fmt.Sprintf("%x", hash)[:8]
	
	ext := GetFileExtensionFromURL(url)
	
	if prefix != "" {
		return fmt.Sprintf("%s_%s%s", prefix, hashStr, ext)
	}
	return fmt.Sprintf("%s%s", hashStr, ext)
}