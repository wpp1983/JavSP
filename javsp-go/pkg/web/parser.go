package web

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Parser provides HTML parsing utilities
type Parser struct {
	doc *goquery.Document
}

// NewParser creates a new parser from HTTP response
func NewParser(resp *http.Response) (*Parser, error) {
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return &Parser{doc: doc}, nil
}

// NewParserFromReader creates a new parser from io.Reader
func NewParserFromReader(r io.Reader) (*Parser, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return &Parser{doc: doc}, nil
}

// NewParserFromString creates a new parser from HTML string
func NewParserFromString(html string) (*Parser, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return &Parser{doc: doc}, nil
}

// Find returns a selection based on CSS selector
func (p *Parser) Find(selector string) *goquery.Selection {
	return p.doc.Find(selector)
}

// ExtractText extracts text content from a CSS selector
func (p *Parser) ExtractText(selector string) string {
	return strings.TrimSpace(p.doc.Find(selector).First().Text())
}

// ExtractTexts extracts all text content from a CSS selector
func (p *Parser) ExtractTexts(selector string) []string {
	var texts []string
	p.doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			texts = append(texts, text)
		}
	})
	return texts
}

// ExtractAttr extracts attribute value from first matching element
func (p *Parser) ExtractAttr(selector, attr string) string {
	val, exists := p.doc.Find(selector).First().Attr(attr)
	if !exists {
		return ""
	}
	return strings.TrimSpace(val)
}

// ExtractAttrs extracts attribute values from all matching elements
func (p *Parser) ExtractAttrs(selector, attr string) []string {
	var attrs []string
	p.doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr(attr); exists {
			val = strings.TrimSpace(val)
			if val != "" {
				attrs = append(attrs, val)
			}
		}
	})
	return attrs
}

// ExtractLinks extracts href attributes from anchor tags
func (p *Parser) ExtractLinks(selector string) []string {
	return p.ExtractAttrs(selector+" a", "href")
}

// ExtractImages extracts src attributes from image tags
func (p *Parser) ExtractImages(selector string) []string {
	return p.ExtractAttrs(selector+" img", "src")
}

// ExtractTable extracts table data as a 2D string slice
func (p *Parser) ExtractTable(selector string) [][]string {
	var table [][]string
	
	p.doc.Find(selector + " tr").Each(func(i int, tr *goquery.Selection) {
		var row []string
		tr.Find("td, th").Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())
			row = append(row, text)
		})
		if len(row) > 0 {
			table = append(table, row)
		}
	})
	
	return table
}

// ExtractKeyValuePairs extracts key-value pairs from a definition list or similar structure
func (p *Parser) ExtractKeyValuePairs(containerSelector, keySelector, valueSelector string) map[string]string {
	pairs := make(map[string]string)
	
	p.doc.Find(containerSelector).Each(func(i int, container *goquery.Selection) {
		key := strings.TrimSpace(container.Find(keySelector).Text())
		value := strings.TrimSpace(container.Find(valueSelector).Text())
		
		if key != "" {
			pairs[key] = value
		}
	})
	
	return pairs
}

// ExtractMovieInfo extracts common movie information fields
func (p *Parser) ExtractMovieInfo() *MovieExtraction {
	extraction := &MovieExtraction{
		Fields:    make(map[string]interface{}),
		KeyValues: make(map[string]string),
		Links:     make(map[string][]string),
		Images:    make(map[string][]string),
	}
	
	// Common selectors for movie information
	selectors := map[string][]string{
		"title": {
			"h1", ".title", "#title", "[class*=title]",
			".movie-title", ".video-title", ".content-title",
		},
		"cover": {
			".cover img", ".poster img", ".thumbnail img", 
			"#cover img", "[class*=cover] img", "[class*=poster] img",
		},
		"actresses": {
			".actress a", ".performer a", ".actor a",
			"[class*=actress] a", "[class*=performer] a",
		},
		"genres": {
			".genre a", ".tag a", ".category a",
			"[class*=genre] a", "[class*=tag] a", "[class*=category] a",
		},
		"runtime": {
			".runtime", ".duration", ".length", "[class*=runtime]", "[class*=duration]",
		},
		"release_date": {
			".release-date", ".date", "[class*=release]", "[class*=date]",
		},
		"director": {
			".director a", "[class*=director] a",
		},
		"producer": {
			".producer a", ".studio a", "[class*=producer] a", "[class*=studio] a",
		},
	}
	
	// Extract using multiple selectors with fallback
	for field, selectorList := range selectors {
		for _, selector := range selectorList {
			var result interface{}
			
			switch field {
			case "cover":
				if src := p.ExtractAttr(selector, "src"); src != "" {
					result = src
				}
			case "actresses", "genres":
				if texts := p.ExtractTexts(selector); len(texts) > 0 {
					result = texts
				}
			default:
				if text := p.ExtractText(selector); text != "" {
					result = text
				}
			}
			
			if result != nil {
				extraction.Fields[field] = result
				break // Use first successful extraction
			}
		}
	}
	
	return extraction
}

// MovieExtraction contains extracted movie information
type MovieExtraction struct {
	Fields    map[string]interface{} `json:"fields"`
	KeyValues map[string]string      `json:"key_values"`
	Links     map[string][]string    `json:"links"`
	Images    map[string][]string    `json:"images"`
}

// CleanText cleans and normalizes text content
func CleanText(text string) string {
	// Remove extra whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)
	
	// Remove common prefixes/suffixes
	text = strings.TrimPrefix(text, "・")
	text = strings.TrimSuffix(text, "・")
	
	return text
}

// ExtractNumber extracts the first number from a string
func ExtractNumber(text string) int {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(text)
	if match == "" {
		return 0
	}
	
	num, _ := strconv.Atoi(match)
	return num
}

// ExtractFloat extracts the first float from a string
func ExtractFloat(text string) float64 {
	re := regexp.MustCompile(`\d+\.?\d*`)
	match := re.FindString(text)
	if match == "" {
		return 0.0
	}
	
	num, _ := strconv.ParseFloat(match, 64)
	return num
}

// NormalizeURL converts relative URLs to absolute URLs
func NormalizeURL(baseURL, relativeURL string) string {
	if strings.HasPrefix(relativeURL, "http") {
		return relativeURL
	}
	
	if strings.HasPrefix(relativeURL, "//") {
		return "https:" + relativeURL
	}
	
	if strings.HasPrefix(relativeURL, "/") {
		// Remove trailing slash from base URL
		baseURL = strings.TrimSuffix(baseURL, "/")
		return baseURL + relativeURL
	}
	
	// Relative path
	baseURL = strings.TrimSuffix(baseURL, "/")
	return baseURL + "/" + relativeURL
}

// ExtractDomainAndID extracts domain and ID from various URL patterns
func ExtractDomainAndID(urlStr string) (domain, id string) {
	// Common patterns for JAV sites
	patterns := []struct {
		regex  *regexp.Regexp
		domain string
	}{
		{regexp.MustCompile(`javbus\.com/.+/([A-Z]+-\d+)`), "javbus"},
		{regexp.MustCompile(`javdb\.com/v/([a-z0-9]+)`), "javdb"},
		{regexp.MustCompile(`javlib\.com/.+\?v=([a-z0-9]+)`), "javlib"},
		{regexp.MustCompile(`dmm\.co\.jp/.+/cid=([a-z0-9]+)`), "dmm"},
	}
	
	for _, pattern := range patterns {
		if matches := pattern.regex.FindStringSubmatch(urlStr); len(matches) > 1 {
			return pattern.domain, matches[1]
		}
	}
	
	return "", ""
}

// Document returns the underlying goquery document
func (p *Parser) Document() *goquery.Document {
	return p.doc
}

// HTML returns the HTML content as string
func (p *Parser) HTML() (string, error) {
	return p.doc.Html()
}

// HasElement checks if an element exists
func (p *Parser) HasElement(selector string) bool {
	return p.doc.Find(selector).Length() > 0
}

// Count returns the number of elements matching the selector
func (p *Parser) Count(selector string) int {
	return p.doc.Find(selector).Length()
}