package crawler

import (
	"context"
	"time"

	"javsp-go/internal/datatype"
)

// Crawler defines the interface for all crawlers
type Crawler interface {
	// Name returns the crawler name
	Name() string
	
	// FetchMovieInfo fetches movie information by ID
	FetchMovieInfo(ctx context.Context, movieID string) (*datatype.MovieInfo, error)
	
	// Search searches for movies by keyword
	Search(ctx context.Context, keyword string) ([]*datatype.MovieInfo, error)
	
	// IsAvailable checks if the crawler is available
	IsAvailable(ctx context.Context) bool
	
	// GetBaseURL returns the base URL of the crawler
	GetBaseURL() string
	
	// GetSupportedTypes returns supported movie types
	GetSupportedTypes() []string
	
	// Close cleans up crawler resources
	Close() error
}

// CrawlerResult represents the result of a crawl operation
type CrawlerResult struct {
	MovieInfo *datatype.MovieInfo `json:"movie_info"`
	Source    string              `json:"source"`
	URL       string              `json:"url"`
	Timestamp time.Time           `json:"timestamp"`
	Error     error               `json:"error,omitempty"`
	Duration  time.Duration       `json:"duration"`
}

// CrawlerStats contains crawler statistics
type CrawlerStats struct {
	Name           string        `json:"name"`
	RequestCount   int64         `json:"request_count"`
	SuccessCount   int64         `json:"success_count"`
	ErrorCount     int64         `json:"error_count"`
	AverageLatency time.Duration `json:"average_latency"`
	LastRequest    time.Time     `json:"last_request"`
	IsHealthy      bool          `json:"is_healthy"`
}

// CrawlerConfig contains common crawler configuration
type CrawlerConfig struct {
	BaseURL          string        `json:"base_url"`
	Timeout          time.Duration `json:"timeout"`
	MaxRetries       int           `json:"max_retries"`
	RetryDelay       time.Duration `json:"retry_delay"`
	RateLimit        time.Duration `json:"rate_limit"`
	UserAgent        string        `json:"user_agent"`
	EnableJavaScript bool          `json:"enable_javascript"`
	ProxyURL         string        `json:"proxy_url,omitempty"`
}

// CrawlerFactory creates crawlers by name
type CrawlerFactory interface {
	CreateCrawler(name string, config *CrawlerConfig) (Crawler, error)
	GetAvailableCrawlers() []string
}

// CrawlerRegistry manages crawler instances
type CrawlerRegistry struct {
	crawlers map[string]Crawler
	stats    map[string]*CrawlerStats
}

// NewCrawlerRegistry creates a new crawler registry
func NewCrawlerRegistry() *CrawlerRegistry {
	return &CrawlerRegistry{
		crawlers: make(map[string]Crawler),
		stats:    make(map[string]*CrawlerStats),
	}
}

// Register registers a crawler
func (r *CrawlerRegistry) Register(name string, crawler Crawler) {
	r.crawlers[name] = crawler
	r.stats[name] = &CrawlerStats{
		Name:        name,
		LastRequest: time.Now(),
		IsHealthy:   true,
	}
}

// Get returns a crawler by name
func (r *CrawlerRegistry) Get(name string) (Crawler, bool) {
	crawler, exists := r.crawlers[name]
	return crawler, exists
}

// GetAll returns all registered crawlers
func (r *CrawlerRegistry) GetAll() map[string]Crawler {
	return r.crawlers
}

// GetStats returns statistics for a crawler
func (r *CrawlerRegistry) GetStats(name string) (*CrawlerStats, bool) {
	stats, exists := r.stats[name]
	return stats, exists
}

// UpdateStats updates statistics for a crawler
func (r *CrawlerRegistry) UpdateStats(name string, success bool, duration time.Duration) {
	if stats, exists := r.stats[name]; exists {
		stats.RequestCount++
		stats.LastRequest = time.Now()
		
		if success {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}
		
		// Calculate average latency
		if stats.RequestCount > 0 {
			if stats.AverageLatency == 0 {
				stats.AverageLatency = duration
			} else {
				stats.AverageLatency = (stats.AverageLatency + duration) / 2
			}
		}
		
		// Update health status
		errorRate := float64(stats.ErrorCount) / float64(stats.RequestCount)
		stats.IsHealthy = errorRate < 0.5 // Healthy if error rate < 50%
	}
}

// Close closes all registered crawlers
func (r *CrawlerRegistry) Close() error {
	for _, crawler := range r.crawlers {
		if err := crawler.Close(); err != nil {
			// Log error but continue closing others
			continue
		}
	}
	return nil
}