package crawler

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"javsp-go/internal/config"
	"javsp-go/internal/datatype"
	"javsp-go/pkg/web"
)

// AVWikiCrawler implements the Crawler interface for AV-Wiki.net
type AVWikiCrawler struct {
	name    string
	baseURL string
	client  *web.Client
	config  *CrawlerConfig
}

// NewAVWikiCrawler creates a new AV-Wiki crawler
func NewAVWikiCrawler(config *CrawlerConfig) (*AVWikiCrawler, error) {
	return NewAVWikiCrawlerWithConfig(config, nil)
}

// NewAVWikiCrawlerWithConfig creates a new AV-Wiki crawler with full config
func NewAVWikiCrawlerWithConfig(crawlerConfig *CrawlerConfig, fullConfig interface{}) (*AVWikiCrawler, error) {
	if crawlerConfig == nil {
		crawlerConfig = &CrawlerConfig{
			BaseURL:    "https://av-wiki.net",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			RetryDelay: 2 * time.Second,
			RateLimit:  1 * time.Second,
		}
	}

	var client *web.Client
	var err error

	// Try to use NewClientWithConfig if full config is available
	if cfg, ok := fullConfig.(*config.Config); ok && cfg != nil {
		client, err = web.NewClientWithConfig(cfg)
	} else {
		// Fallback to manual client options
		clientOpts := &web.ClientOptions{
			Timeout:       crawlerConfig.Timeout,
			EnableCookies: true,
			SkipTLSVerify: true,
			RateLimit:     crawlerConfig.RateLimit,
		}

		if crawlerConfig.ProxyURL != "" {
			clientOpts.ProxyURL = crawlerConfig.ProxyURL
		}

		client, err = web.NewClient(clientOpts)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &AVWikiCrawler{
		name:    "avwiki",
		baseURL: crawlerConfig.BaseURL,
		client:  client,
		config:  crawlerConfig,
	}, nil
}

// Name returns the crawler name
func (a *AVWikiCrawler) Name() string {
	return a.name
}

// GetBaseURL returns the base URL
func (a *AVWikiCrawler) GetBaseURL() string {
	return a.baseURL
}

// GetSupportedTypes returns supported movie types
func (a *AVWikiCrawler) GetSupportedTypes() []string {
	return []string{"normal", "amateur"}
}

// FetchMovieInfo fetches movie information by ID
func (a *AVWikiCrawler) FetchMovieInfo(ctx context.Context, movieID string) (*datatype.MovieInfo, error) {
	// Construct URL
	movieURL := fmt.Sprintf("%s/%s", a.baseURL, movieID)
	
	// Fetch the page
	resp, err := a.client.Get(ctx, movieURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("movie not found: %s", movieID)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Parse the HTML
	parser, err := web.NewParser(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract movie information
	movieInfo, err := a.extractMovieInfo(parser, movieID, movieURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract movie info: %w", err)
	}

	return movieInfo, nil
}

// extractMovieInfo extracts movie information from the parsed page
func (a *AVWikiCrawler) extractMovieInfo(parser *web.Parser, movieID, movieURL string) (*datatype.MovieInfo, error) {
	info := datatype.NewMovieInfo(movieID)
	info.Source = a.name
	info.SourceURL = movieURL

	// Check if page exists - look for 404 in title
	title := parser.ExtractText("title")
	if strings.Contains(title, "404") || strings.Contains(title, "Not Found") {
		return nil, fmt.Errorf("movie not found: %s", movieID)
	}

	// Extract title from h1 tag
	h1Title := parser.ExtractText("h1")
	if h1Title != "" {
		// Clean title - remove Japanese suffixes
		cleanedTitle := a.cleanTitle(h1Title)
		info.Title = cleanedTitle
	}

	// Extract cover image
	coverURL := parser.ExtractAttr("img[src*='dmm.co.jp'], img[src*='mgstage.com']", "src")
	if coverURL != "" {
		info.Cover = coverURL
	}

	// Extract actresses from title
	if info.Title != "" {
		actresses := a.extractActressesFromTitle(info.Title)
		if len(actresses) > 0 {
			info.Actress = actresses
		}
	}

	// Extract content from entry-content div
	contentText := parser.ExtractText(".entry-content")
	if contentText != "" {
		a.extractFromContent(contentText, info)
	}

	// Set additional properties
	info.Uncensored = false // AV-Wiki mainly has censored content

	// Validate required fields
	if err := info.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return info, nil
}

// cleanTitle cleans the title by removing Japanese suffixes
func (a *AVWikiCrawler) cleanTitle(title string) string {
	// Remove common Japanese suffixes
	title = regexp.MustCompile(`に出てる.*$`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`まとめ$`).ReplaceAllString(title, "")
	
	return strings.TrimSpace(title)
}

// extractActressesFromTitle extracts actress names from the title
func (a *AVWikiCrawler) extractActressesFromTitle(title string) []string {
	var actresses []string
	
	// Split by Japanese "と" (and) and other separators
	parts := regexp.MustCompile(`[とと、,]`).Split(title, -1)
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// Filter out obvious non-names
		if len(part) >= 2 && !a.isNotActressName(part) {
			actresses = append(actresses, part)
		}
	}
	
	return actresses
}

// isNotActressName checks if a string is obviously not an actress name
func (a *AVWikiCrawler) isNotActressName(name string) bool {
	// Check for obvious non-names
	if strings.Contains(name, "女優") {
		return true
	}
	
	// Check for series codes
	excludePatterns := []string{
		"AV", "まとめ", "LUXU", "MIUM", "GANA", "SIRO", 
		"ABP", "SSIS", "STARS", "PRED", "JUL", "JUX", "MEYD",
		"FC2", "PPV", "素人",
	}
	
	for _, pattern := range excludePatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	
	return false
}

// extractFromContent extracts information from page content
func (a *AVWikiCrawler) extractFromContent(content string, info *datatype.MovieInfo) {
	// Extract producer (メーカー takes precedence over スタジオ)
	if producerMatch := regexp.MustCompile(`メーカー[：:]\s*([^\n]+)`).FindStringSubmatch(content); len(producerMatch) > 1 {
		info.Producer = strings.TrimSpace(producerMatch[1])
	} else if studioMatch := regexp.MustCompile(`スタジオ[：:]\s*([^\n]+)`).FindStringSubmatch(content); len(studioMatch) > 1 {
		info.Producer = strings.TrimSpace(studioMatch[1])
	}
	
	// Extract release date
	if dateMatch := regexp.MustCompile(`配信開始日[：:]?\s*(\d{4}-\d{1,2}-\d{1,2})`).FindStringSubmatch(content); len(dateMatch) > 1 {
		info.ReleaseDate = dateMatch[1]
	}
	
	// Extract series
	if seriesMatch := regexp.MustCompile(`シリーズ[：:]\s*([^\n]+)`).FindStringSubmatch(content); len(seriesMatch) > 1 {
		info.Series = strings.TrimSpace(seriesMatch[1])
	}
}

// Search searches for movies by keyword
func (a *AVWikiCrawler) Search(ctx context.Context, keyword string) ([]*datatype.MovieInfo, error) {
	// AV-Wiki doesn't have a dedicated search functionality
	// We'll try to fetch the movie directly using the keyword as movie ID
	movieInfo, err := a.FetchMovieInfo(ctx, keyword)
	if err != nil {
		return nil, err
	}
	
	return []*datatype.MovieInfo{movieInfo}, nil
}

// IsAvailable checks if the crawler is available
func (a *AVWikiCrawler) IsAvailable(ctx context.Context) bool {
	resp, err := a.client.Get(ctx, a.baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == 200
}

// Close cleans up resources
func (a *AVWikiCrawler) Close() error {
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}