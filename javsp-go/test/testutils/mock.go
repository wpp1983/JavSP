package testutils

import (
	"context"
	"time"

	"javsp-go/internal/crawler"
	"javsp-go/internal/datatype"
)

// MockCrawler implements the crawler.Crawler interface for testing
type MockCrawler struct {
	name          string
	baseURL       string
	movieData     map[string]*datatype.MovieInfo
	searchResults map[string][]*datatype.MovieInfo
	isAvailable   bool
	shouldFail    bool
	delay         time.Duration
}

// NewMockCrawler creates a new mock crawler
func NewMockCrawler(name, baseURL string) *MockCrawler {
	return &MockCrawler{
		name:          name,
		baseURL:       baseURL,
		movieData:     make(map[string]*datatype.MovieInfo),
		searchResults: make(map[string][]*datatype.MovieInfo),
		isAvailable:   true,
		shouldFail:    false,
		delay:         0,
	}
}

// Name returns the crawler name
func (m *MockCrawler) Name() string {
	return m.name
}

// GetBaseURL returns the base URL
func (m *MockCrawler) GetBaseURL() string {
	return m.baseURL
}

// GetSupportedTypes returns supported types
func (m *MockCrawler) GetSupportedTypes() []string {
	return []string{"normal", "amateur"}
}

// FetchMovieInfo fetches movie information by ID
func (m *MockCrawler) FetchMovieInfo(ctx context.Context, movieID string) (*datatype.MovieInfo, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldFail {
		return nil, &MockError{Message: "mock fetch error"}
	}

	info, exists := m.movieData[movieID]
	if !exists {
		return nil, &MockError{Message: "movie not found"}
	}

	return info, nil
}

// Search searches for movies by keyword
func (m *MockCrawler) Search(ctx context.Context, keyword string) ([]*datatype.MovieInfo, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldFail {
		return nil, &MockError{Message: "mock search error"}
	}

	results, exists := m.searchResults[keyword]
	if !exists {
		return []*datatype.MovieInfo{}, nil
	}

	return results, nil
}

// IsAvailable checks if the crawler is available
func (m *MockCrawler) IsAvailable(ctx context.Context) bool {
	return m.isAvailable
}

// Close cleans up resources
func (m *MockCrawler) Close() error {
	return nil
}

// SetMovieData sets mock movie data
func (m *MockCrawler) SetMovieData(movieID string, info *datatype.MovieInfo) {
	m.movieData[movieID] = info
}

// SetSearchResults sets mock search results
func (m *MockCrawler) SetSearchResults(keyword string, results []*datatype.MovieInfo) {
	m.searchResults[keyword] = results
}

// SetAvailable sets availability status
func (m *MockCrawler) SetAvailable(available bool) {
	m.isAvailable = available
}

// SetShouldFail sets failure mode
func (m *MockCrawler) SetShouldFail(shouldFail bool) {
	m.shouldFail = shouldFail
}

// SetDelay sets artificial delay
func (m *MockCrawler) SetDelay(delay time.Duration) {
	m.delay = delay
}

// MockError represents a mock error
type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}

// MockCrawlerFactory creates mock crawlers
type MockCrawlerFactory struct {
	crawlers map[string]*MockCrawler
}

// NewMockCrawlerFactory creates a new mock crawler factory
func NewMockCrawlerFactory() *MockCrawlerFactory {
	return &MockCrawlerFactory{
		crawlers: make(map[string]*MockCrawler),
	}
}

// CreateCrawler creates a mock crawler
func (f *MockCrawlerFactory) CreateCrawler(name string, config *crawler.CrawlerConfig) (crawler.Crawler, error) {
	if crawler, exists := f.crawlers[name]; exists {
		return crawler, nil
	}

	mockCrawler := NewMockCrawler(name, config.BaseURL)
	f.crawlers[name] = mockCrawler
	return mockCrawler, nil
}

// GetAvailableCrawlers returns available crawler names
func (f *MockCrawlerFactory) GetAvailableCrawlers() []string {
	names := make([]string, 0, len(f.crawlers))
	for name := range f.crawlers {
		names = append(names, name)
	}
	return names
}

// RegisterCrawler registers a mock crawler
func (f *MockCrawlerFactory) RegisterCrawler(name string, crawler *MockCrawler) {
	f.crawlers[name] = crawler
}

// GetCrawler gets a mock crawler by name
func (f *MockCrawlerFactory) GetCrawler(name string) *MockCrawler {
	return f.crawlers[name]
}