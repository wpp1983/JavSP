package crawler

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"javsp-go/internal/config"
	"javsp-go/internal/datatype"
	"javsp-go/pkg/web"
)

// JavBusCrawler implements the Crawler interface for JavBus
type JavBusCrawler struct {
	name       string
	baseURL    string
	client     *web.Client
	config     *CrawlerConfig
	userAgents []string
}

// NewJavBusCrawler creates a new JavBus crawler
func NewJavBusCrawler(config *CrawlerConfig) (*JavBusCrawler, error) {
	return NewJavBusCrawlerWithConfig(config, nil)
}

// NewJavBusCrawlerWithConfig creates a new JavBus crawler with full config
func NewJavBusCrawlerWithConfig(crawlerConfig *CrawlerConfig, fullConfig interface{}) (*JavBusCrawler, error) {
	if crawlerConfig == nil {
		crawlerConfig = &CrawlerConfig{
			BaseURL:    "https://www.javbus.com",
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

	return &JavBusCrawler{
		name:    "javbus2",
		baseURL: crawlerConfig.BaseURL,
		client:  client,
		config:  crawlerConfig,
	}, nil
}

// Name returns the crawler name
func (j *JavBusCrawler) Name() string {
	return j.name
}

// GetBaseURL returns the base URL
func (j *JavBusCrawler) GetBaseURL() string {
	return j.baseURL
}

// GetSupportedTypes returns supported movie types
func (j *JavBusCrawler) GetSupportedTypes() []string {
	return []string{"normal", "fc2", "amateur"}
}

// FetchMovieInfo fetches movie information by ID
func (j *JavBusCrawler) FetchMovieInfo(ctx context.Context, movieID string) (*datatype.MovieInfo, error) {
	// Construct URL
	movieURL := j.constructMovieURL(movieID)
	
	// Fetch the page
	resp, err := j.client.Get(ctx, movieURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Parse the HTML
	parser, err := web.NewParser(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract movie information
	movieInfo, err := j.extractMovieInfo(parser, movieID, movieURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract movie info: %w", err)
	}

	return movieInfo, nil
}

// constructMovieURL constructs the movie URL based on movie ID
func (j *JavBusCrawler) constructMovieURL(movieID string) string {
	// Handle different ID formats
	if strings.HasPrefix(movieID, "FC2-") {
		// FC2 format: https://www.javbus.com/FC2-1234567
		return fmt.Sprintf("%s/%s", j.baseURL, movieID)
	}
	
	// Standard format: https://www.javbus.com/STARS-123
	return fmt.Sprintf("%s/%s", j.baseURL, movieID)
}

// extractMovieInfo extracts movie information from the parsed page
func (j *JavBusCrawler) extractMovieInfo(parser *web.Parser, movieID, movieURL string) (*datatype.MovieInfo, error) {
	info := datatype.NewMovieInfo(movieID)
	info.Source = j.name
	info.SourceURL = movieURL

	// Extract title
	title := parser.ExtractText("h3")
	if title == "" {
		title = parser.ExtractText(".title")
	}
	if title != "" {
		info.Title = j.cleanTitle(title)
	}

	// Extract cover image
	coverURL := parser.ExtractAttr(".bigImage img", "src")
	if coverURL == "" {
		coverURL = parser.ExtractAttr("img.poster", "src")
	}
	if coverURL != "" {
		info.Cover = j.normalizeImageURL(coverURL)
	}

	// Extract basic information from info table
	j.extractInfoTable(parser, info)

	// Extract actresses
	actresses := parser.ExtractTexts(".star-name a")
	if len(actresses) == 0 {
		actresses = parser.ExtractTexts(".performer a")
	}
	info.Actress = j.cleanActresses(actresses)

	// Extract genres
	genres := parser.ExtractTexts(".genre a")
	info.Genre = j.cleanGenres(genres)

	// Extract preview images
	previews := parser.ExtractAttrs(".sample-box img", "src")
	if len(previews) == 0 {
		previews = parser.ExtractAttrs(".preview img", "src")
	}
	info.Preview = j.normalizeImageURLs(previews)

	// Validate required fields
	if err := info.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return info, nil
}

// extractInfoTable extracts information from the details table
func (j *JavBusCrawler) extractInfoTable(parser *web.Parser, info *datatype.MovieInfo) {
	// Extract info using table rows
	parser.Find("div.info").Each(func(i int, s *goquery.Selection) {
		s.Find("p").Each(func(_ int, p *goquery.Selection) {
			text := strings.TrimSpace(p.Text())
			
			// Parse different types of information
			if strings.Contains(text, "發行日期:") || strings.Contains(text, "Release Date:") {
				date := j.extractAfterColon(text)
				if date != "" {
					info.ReleaseDate = j.normalizeDate(date)
				}
			} else if strings.Contains(text, "長度:") || strings.Contains(text, "Runtime:") {
				runtime := j.extractAfterColon(text)
				if runtime != "" {
					info.Runtime = j.normalizeRuntime(runtime)
				}
			} else if strings.Contains(text, "導演:") || strings.Contains(text, "Director:") {
				director := j.extractAfterColon(text)
				if director != "" {
					info.Director = director
				}
			} else if strings.Contains(text, "製作商:") || strings.Contains(text, "Studio:") {
				producer := j.extractAfterColon(text)
				if producer != "" {
					info.Producer = producer
				}
			} else if strings.Contains(text, "發行商:") || strings.Contains(text, "Publisher:") {
				publisher := j.extractAfterColon(text)
				if publisher != "" {
					info.Publisher = publisher
				}
			} else if strings.Contains(text, "系列:") || strings.Contains(text, "Series:") {
				series := j.extractAfterColon(text)
				if series != "" {
					info.Series = series
				}
			}
		})
	})

	// Alternative extraction using definition list
	keyValues := parser.ExtractKeyValuePairs("dl", "dt", "dd")
	for key, value := range keyValues {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		
		switch {
		case strings.Contains(key, "發行日期") || strings.Contains(key, "Release Date"):
			if info.ReleaseDate == "" && value != "" {
				info.ReleaseDate = j.normalizeDate(value)
			}
		case strings.Contains(key, "長度") || strings.Contains(key, "Runtime"):
			if info.Runtime == "" && value != "" {
				info.Runtime = j.normalizeRuntime(value)
			}
		case strings.Contains(key, "導演") || strings.Contains(key, "Director"):
			if info.Director == "" && value != "" {
				info.Director = value
			}
		}
	}
}

// Search searches for movies by keyword
func (j *JavBusCrawler) Search(ctx context.Context, keyword string) ([]*datatype.MovieInfo, error) {
	// Construct search URL
	searchURL := fmt.Sprintf("%s/search/%s", j.baseURL, url.QueryEscape(keyword))
	
	resp, err := j.client.Get(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search page: %w", err)
	}
	defer resp.Body.Close()

	parser, err := web.NewParser(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	var results []*datatype.MovieInfo
	
	// Extract movie links from search results
	movieLinks := parser.ExtractAttrs(".movie-box", "href")
	for _, link := range movieLinks {
		if movieID := j.extractMovieIDFromURL(link); movieID != "" {
			info, err := j.FetchMovieInfo(ctx, movieID)
			if err != nil {
				continue // Skip failed entries
			}
			results = append(results, info)
		}
	}

	return results, nil
}

// IsAvailable checks if the crawler is available
func (j *JavBusCrawler) IsAvailable(ctx context.Context) bool {
	resp, err := j.client.Get(ctx, j.baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == 200
}

// Close cleans up resources
func (j *JavBusCrawler) Close() error {
	if j.client != nil {
		return j.client.Close()
	}
	return nil
}

// Helper methods for data cleaning and normalization

func (j *JavBusCrawler) cleanTitle(title string) string {
	// Remove movie ID from title if present
	title = regexp.MustCompile(`^[A-Z]+-\d+\s*`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`^FC2-PPV-\d+\s*`).ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func (j *JavBusCrawler) cleanActresses(actresses []string) []string {
	var cleaned []string
	for _, actress := range actresses {
		actress = strings.TrimSpace(actress)
		if actress != "" && actress != "-" {
			cleaned = append(cleaned, actress)
		}
	}
	return cleaned
}

func (j *JavBusCrawler) cleanGenres(genres []string) []string {
	var cleaned []string
	for _, genre := range genres {
		genre = strings.TrimSpace(genre)
		if genre != "" {
			cleaned = append(cleaned, genre)
		}
	}
	return cleaned
}

func (j *JavBusCrawler) extractAfterColon(text string) string {
	parts := strings.Split(text, ":")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func (j *JavBusCrawler) normalizeDate(dateStr string) string {
	// JavBus uses YYYY-MM-DD format, which is already normalized
	dateStr = strings.TrimSpace(dateStr)
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, dateStr); matched {
		return dateStr
	}
	return dateStr
}

func (j *JavBusCrawler) normalizeRuntime(runtime string) string {
	// Extract minutes from runtime string
	runtime = strings.TrimSpace(runtime)
	if strings.Contains(runtime, "分鐘") || strings.Contains(runtime, "minutes") {
		return runtime
	}
	
	// Add "分鐘" suffix if it's just a number
	if matched, _ := regexp.MatchString(`^\d+$`, runtime); matched {
		return runtime + "分鐘"
	}
	
	return runtime
}

func (j *JavBusCrawler) normalizeImageURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	
	if strings.HasPrefix(imageURL, "//") {
		return "https:" + imageURL
	}
	
	if strings.HasPrefix(imageURL, "/") {
		return j.baseURL + imageURL
	}
	
	return imageURL
}

func (j *JavBusCrawler) normalizeImageURLs(imageURLs []string) []string {
	var normalized []string
	for _, url := range imageURLs {
		if normalizedURL := j.normalizeImageURL(url); normalizedURL != "" {
			normalized = append(normalized, normalizedURL)
		}
	}
	return normalized
}

func (j *JavBusCrawler) extractMovieIDFromURL(urlStr string) string {
	// Extract movie ID from URL like https://www.javbus.com/STARS-123
	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}