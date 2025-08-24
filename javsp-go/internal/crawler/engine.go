package crawler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"javsp-go/internal/config"
	"javsp-go/internal/datatype"
	"javsp-go/pkg/progress"
)

// EngineConfig contains configuration for the crawler engine
type EngineConfig struct {
	DefaultTimeout        time.Duration `json:"default_timeout"`
	RetryEnabled          bool          `json:"retry_enabled"`
	MaxRetries            int           `json:"max_retries"`
	RetryDelay            time.Duration `json:"retry_delay"`
	FailFast              bool          `json:"fail_fast"`
}

// CrawlResult represents the result from a crawler
type CrawlResult struct {
	MovieInfo *datatype.MovieInfo `json:"movie_info"`
	Source    string              `json:"source"`
	Error     error               `json:"error,omitempty"`
	Duration  time.Duration       `json:"duration"`
	Timestamp time.Time           `json:"timestamp"`
}

// CrawlerProgressCallback is called with crawler progress updates
type CrawlerProgressCallback func(crawlerName, message string, progress float64, elapsed time.Duration, remaining time.Duration)

// Engine coordinates multiple crawlers for movie information extraction
type Engine struct {
	registry         *CrawlerRegistry
	config           *EngineConfig
	stats            *EngineStats
	mu               sync.RWMutex
	progressCallback CrawlerProgressCallback
}

// EngineStats tracks engine performance statistics
type EngineStats struct {
	TotalRequests    int64                    `json:"total_requests"`
	SuccessfulCrawls int64                    `json:"successful_crawls"`
	FailedCrawls     int64                    `json:"failed_crawls"`
	AverageLatency   time.Duration            `json:"average_latency"`
	CrawlerStats     map[string]*CrawlerStats `json:"crawler_stats"`
	LastReset        time.Time                `json:"last_reset"`
}

// NewEngine creates a new crawler engine
func NewEngine(cfg *config.Config) (*Engine, error) {
	registry := NewCrawlerRegistry()
	
	// Create default engine config
	engineConfig := &EngineConfig{
		DefaultTimeout:        30 * time.Second,
		RetryEnabled:          true,
		MaxRetries:            3,
		RetryDelay:            2 * time.Second,
		FailFast:              false,
	}

	// Override with config values if available
	if cfg != nil {
		if cfg.Network.Timeout > 0 {
			engineConfig.DefaultTimeout = cfg.Network.Timeout
		}
		if cfg.Network.Retry > 0 {
			engineConfig.MaxRetries = cfg.Network.Retry
		}
	}

	stats := &EngineStats{
		CrawlerStats: make(map[string]*CrawlerStats),
		LastReset:    time.Now(),
	}

	engine := &Engine{
		registry: registry,
		config:   engineConfig,
		stats:    stats,
	}

	// Initialize default crawlers
	if err := engine.initDefaultCrawlers(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize default crawlers: %w", err)
	}

	return engine, nil
}

// initDefaultCrawlers initializes the default set of crawlers
func (e *Engine) initDefaultCrawlers(cfg *config.Config) error {
	var crawlerConfig *CrawlerConfig
	if cfg != nil {
		crawlerConfig = &CrawlerConfig{
			BaseURL:    "",
			Timeout:    cfg.Network.Timeout,
			MaxRetries: cfg.Network.Retry,
			RetryDelay: 2 * time.Second,
			RateLimit:  cfg.Crawler.SleepAfterScraping,
		}
		if cfg.Network.ProxyServer != nil {
			crawlerConfig.ProxyURL = *cfg.Network.ProxyServer
		}
	}

	// Initialize JavBus crawler
	javbusCrawler, err := NewJavBusCrawlerWithConfig(crawlerConfig, cfg)
	if err != nil {
		return fmt.Errorf("failed to create JavBus crawler: %w", err)
	}
	e.registry.Register("javbus2", javbusCrawler)

	// Initialize AVWiki crawler
	avwikiCrawler, err := NewAVWikiCrawlerWithConfig(crawlerConfig, cfg)
	if err != nil {
		return fmt.Errorf("failed to create AVWiki crawler: %w", err)
	}
	e.registry.Register("avwiki", avwikiCrawler)

	return nil
}

// RegisterCrawler registers a new crawler
func (e *Engine) RegisterCrawler(name string, crawler Crawler) {
	e.registry.Register(name, crawler)
}

// SetProgressCallback sets the progress callback for crawler operations
func (e *Engine) SetProgressCallback(callback CrawlerProgressCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.progressCallback = callback
}

// GetAvailableCrawlers returns a list of available crawler names
func (e *Engine) GetAvailableCrawlers() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var names []string
	for name := range e.registry.GetAll() {
		names = append(names, name)
	}
	return names
}

// CrawlMovie fetches movie information using multiple crawlers serially
func (e *Engine) CrawlMovie(ctx context.Context, movieID string, crawlerNames ...string) ([]*CrawlResult, error) {
	if len(crawlerNames) == 0 {
		crawlerNames = e.GetAvailableCrawlers()
	}

	results := make([]*CrawlResult, 0, len(crawlerNames))
	
	// Create context with timeout
	crawlCtx, cancel := context.WithTimeout(ctx, e.config.DefaultTimeout)
	defer cancel()

	// Execute crawlers serially
	for _, crawlerName := range crawlerNames {
		crawler, exists := e.registry.Get(crawlerName)
		if !exists {
			continue
		}

		result := e.crawlSingle(crawlCtx, crawlerName, crawler, movieID)
		results = append(results, result)
		e.updateStats(result)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no crawlers available for movie ID: %s", movieID)
	}

	return results, nil
}

// crawlSingle performs crawling with a single crawler
func (e *Engine) crawlSingle(ctx context.Context, crawlerName string, crawler Crawler, movieID string) *CrawlResult {
	start := time.Now()
	result := &CrawlResult{
		Source:    crawlerName,
		Timestamp: start,
	}

	// Create progress tracker if callback is set
	var tracker *progress.ProgressTracker
	if e.progressCallback != nil {
		timeout := e.config.DefaultTimeout
		message := fmt.Sprintf("Fetching from %s", crawlerName)
		
		tracker = progress.NewProgressTracker(message, timeout, func(msg string, prog float64, remaining time.Duration) {
			elapsed := time.Since(start)
			e.progressCallback(crawlerName, msg, prog, elapsed, remaining)
		})
		defer tracker.Done()
	}

	// Set up web client progress callback if the crawler supports it
	// This will be handled by the crawler's own progress reporting

	// Perform crawling with retry logic
	var movieInfo *datatype.MovieInfo
	var err error

	maxAttempts := 1
	if e.config.RetryEnabled {
		maxAttempts = e.config.MaxRetries
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if tracker != nil {
				tracker.UpdateMessage(fmt.Sprintf("Retrying %s (attempt %d/%d)", crawlerName, attempt+1, maxAttempts))
			}
			
			// Wait before retry
			select {
			case <-ctx.Done():
				err = ctx.Err()
				break
			case <-time.After(e.config.RetryDelay):
			}
		} else if tracker != nil {
			tracker.UpdateMessage(fmt.Sprintf("Connecting to %s", crawlerName))
		}

		movieInfo, err = crawler.FetchMovieInfo(ctx, movieID)
		if err == nil {
			if tracker != nil {
				tracker.UpdateMessage(fmt.Sprintf("Successfully fetched from %s", crawlerName))
			}
			break
		}

		// Check if we should continue retrying
		if !e.shouldRetry(err) {
			break
		}
	}

	result.Duration = time.Since(start)
	result.MovieInfo = movieInfo
	result.Error = err

	// Update progress with final result
	if tracker != nil {
		if err != nil {
			tracker.Fail(err)
		} else {
			tracker.Done()
		}
	}

	// Update crawler statistics
	e.registry.UpdateStats(crawlerName, err == nil, result.Duration)

	return result
}

// shouldRetry determines if an error is retryable
func (e *Engine) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	
	// Don't retry on certain errors
	nonRetryableErrors := []string{
		"movie not found",
		"validation failed",
		"context canceled",
		"context deadline exceeded",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if strings.Contains(errorStr, nonRetryable) {
			return false
		}
	}

	return true
}

// CrawlBatch performs batch crawling for multiple movie IDs serially
func (e *Engine) CrawlBatch(ctx context.Context, movieIDs []string, crawlerNames ...string) (map[string][]*CrawlResult, error) {
	results := make(map[string][]*CrawlResult)
	
	// Process each movie ID serially
	for _, movieID := range movieIDs {
		crawlResults, err := e.CrawlMovie(ctx, movieID, crawlerNames...)
		if err != nil {
			// Create error result
			errorResult := &CrawlResult{
				Source:    "engine",
				Error:     err,
				Timestamp: time.Now(),
			}
			crawlResults = []*CrawlResult{errorResult}
		}
		
		results[movieID] = crawlResults
	}

	return results, nil
}

// GetStats returns current engine statistics
func (e *Engine) GetStats() *EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Create a copy of stats to avoid data races
	statsCopy := &EngineStats{
		TotalRequests:    e.stats.TotalRequests,
		SuccessfulCrawls: e.stats.SuccessfulCrawls,
		FailedCrawls:     e.stats.FailedCrawls,
		AverageLatency:   e.stats.AverageLatency,
		LastReset:        e.stats.LastReset,
		CrawlerStats:     make(map[string]*CrawlerStats),
	}

	// Copy crawler stats
	for name, stats := range e.stats.CrawlerStats {
		statsCopy.CrawlerStats[name] = &CrawlerStats{
			Name:           stats.Name,
			RequestCount:   stats.RequestCount,
			SuccessCount:   stats.SuccessCount,
			ErrorCount:     stats.ErrorCount,
			AverageLatency: stats.AverageLatency,
			LastRequest:    stats.LastRequest,
			IsHealthy:      stats.IsHealthy,
		}
	}

	return statsCopy
}

// ResetStats resets all statistics
func (e *Engine) ResetStats() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.TotalRequests = 0
	e.stats.SuccessfulCrawls = 0
	e.stats.FailedCrawls = 0
	e.stats.AverageLatency = 0
	e.stats.LastReset = time.Now()
	e.stats.CrawlerStats = make(map[string]*CrawlerStats)
}

// updateStats updates engine statistics
func (e *Engine) updateStats(result *CrawlResult) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.TotalRequests++
	
	if result.Error == nil {
		e.stats.SuccessfulCrawls++
	} else {
		e.stats.FailedCrawls++
	}

	// Update average latency
	if e.stats.AverageLatency == 0 {
		e.stats.AverageLatency = result.Duration
	} else {
		e.stats.AverageLatency = (e.stats.AverageLatency + result.Duration) / 2
	}

	// Update crawler-specific stats
	if _, exists := e.stats.CrawlerStats[result.Source]; !exists {
		e.stats.CrawlerStats[result.Source] = &CrawlerStats{
			Name: result.Source,
		}
	}
}

// IsHealthy returns true if the engine is healthy
func (e *Engine) IsHealthy(ctx context.Context) bool {
	crawlers := e.GetAvailableCrawlers()
	if len(crawlers) == 0 {
		return false
	}

	// Check if at least one crawler is available
	for _, name := range crawlers {
		if crawler, exists := e.registry.Get(name); exists {
			if crawler.IsAvailable(ctx) {
				return true
			}
		}
	}

	return false
}

// Close closes the engine and all registered crawlers
func (e *Engine) Close() error {
	return e.registry.Close()
}

