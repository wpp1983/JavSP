//go:build unit

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewParser(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><h1>Test</h1></body></html>`))
	}))
	defer server.Close()

	// Create request and response
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}

	parser, err := NewParser(resp)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	if parser.doc == nil {
		t.Error("Parser document should not be nil")
	}

	// Test text extraction
	title := parser.ExtractText("h1")
	if title != "Test" {
		t.Errorf("Expected title 'Test', got '%s'", title)
	}
}

func TestNewParserFromString(t *testing.T) {
	html := `
	<html>
		<body>
			<h1>Title</h1>
			<p class="content">Content text</p>
			<div class="info">
				<span class="label">Label:</span>
				<span class="value">Value</span>
			</div>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Test basic text extraction
	title := parser.ExtractText("h1")
	if title != "Title" {
		t.Errorf("Expected 'Title', got '%s'", title)
	}

	content := parser.ExtractText(".content")
	if content != "Content text" {
		t.Errorf("Expected 'Content text', got '%s'", content)
	}

	// Test attribute extraction
	if !parser.HasElement(".info") {
		t.Error("Expected .info element to exist")
	}

	count := parser.Count("span")
	if count != 2 {
		t.Errorf("Expected 2 span elements, got %d", count)
	}
}

func TestExtractTexts(t *testing.T) {
	html := `
	<html>
		<body>
			<ul>
				<li class="item">Item 1</li>
				<li class="item">Item 2</li>
				<li class="item">Item 3</li>
			</ul>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	texts := parser.ExtractTexts(".item")
	expected := []string{"Item 1", "Item 2", "Item 3"}

	if len(texts) != len(expected) {
		t.Errorf("Expected %d items, got %d", len(expected), len(texts))
	}

	for i, text := range texts {
		if text != expected[i] {
			t.Errorf("Expected '%s' at index %d, got '%s'", expected[i], i, text)
		}
	}
}

func TestExtractAttrs(t *testing.T) {
	html := `
	<html>
		<body>
			<div>
				<a href="/link1" class="link">Link 1</a>
				<a href="/link2" class="link">Link 2</a>
				<img src="/image1.jpg" alt="Image 1">
				<img src="/image2.jpg" alt="Image 2">
			</div>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	// Test link extraction
	links := parser.ExtractLinks("div")
	expected := []string{"/link1", "/link2"}

	if len(links) != len(expected) {
		t.Errorf("Expected %d links, got %d", len(expected), len(links))
	}

	for i, link := range links {
		if link != expected[i] {
			t.Errorf("Expected link '%s' at index %d, got '%s'", expected[i], i, link)
		}
	}

	// Test image extraction
	images := parser.ExtractImages("div")
	expectedImages := []string{"/image1.jpg", "/image2.jpg"}

	if len(images) != len(expectedImages) {
		t.Errorf("Expected %d images, got %d", len(expectedImages), len(images))
	}

	for i, img := range images {
		if img != expectedImages[i] {
			t.Errorf("Expected image '%s' at index %d, got '%s'", expectedImages[i], i, img)
		}
	}
}

func TestExtractTable(t *testing.T) {
	html := `
	<html>
		<body>
			<table id="info">
				<tr>
					<th>Name</th>
					<th>Value</th>
				</tr>
				<tr>
					<td>Title</td>
					<td>Test Movie</td>
				</tr>
				<tr>
					<td>Year</td>
					<td>2023</td>
				</tr>
			</table>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	table := parser.ExtractTable("#info")
	expected := [][]string{
		{"Name", "Value"},
		{"Title", "Test Movie"},
		{"Year", "2023"},
	}

	if len(table) != len(expected) {
		t.Errorf("Expected %d rows, got %d", len(expected), len(table))
	}

	for i, row := range table {
		if len(row) != len(expected[i]) {
			t.Errorf("Row %d: expected %d columns, got %d", i, len(expected[i]), len(row))
		}

		for j, cell := range row {
			if cell != expected[i][j] {
				t.Errorf("Row %d, Col %d: expected '%s', got '%s'", i, j, expected[i][j], cell)
			}
		}
	}
}

func TestExtractKeyValuePairs(t *testing.T) {
	html := `
	<html>
		<body>
			<div class="info-row">
				<span class="label">Title:</span>
				<span class="value">Test Movie</span>
			</div>
			<div class="info-row">
				<span class="label">Year:</span>
				<span class="value">2023</span>
			</div>
			<div class="info-row">
				<span class="label">Genre:</span>
				<span class="value">Action</span>
			</div>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	pairs := parser.ExtractKeyValuePairs(".info-row", ".label", ".value")
	expected := map[string]string{
		"Title:": "Test Movie",
		"Year:":  "2023",
		"Genre:": "Action",
	}

	if len(pairs) != len(expected) {
		t.Errorf("Expected %d pairs, got %d", len(expected), len(pairs))
	}

	for key, value := range expected {
		if pairs[key] != value {
			t.Errorf("Expected '%s' = '%s', got '%s'", key, value, pairs[key])
		}
	}
}

func TestExtractMovieInfo(t *testing.T) {
	html := `
	<html>
		<head>
			<title>STARS-123 Test Movie</title>
		</head>
		<body>
			<h1>Test Movie Title</h1>
			<div class="cover">
				<img src="/covers/STARS-123.jpg" alt="Cover">
			</div>
			<div class="actress">
				<a href="/actress/1">Actress 1</a>
				<a href="/actress/2">Actress 2</a>
			</div>
			<div class="genre">
				<a href="/genre/1">Genre 1</a>
				<a href="/genre/2">Genre 2</a>
			</div>
			<div class="runtime">120 minutes</div>
			<div class="release-date">2023-12-01</div>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	extraction := parser.ExtractMovieInfo()

	// Test title extraction
	if title, ok := extraction.Fields["title"].(string); !ok || title != "Test Movie Title" {
		t.Errorf("Expected title 'Test Movie Title', got %v", extraction.Fields["title"])
	}

	// Test cover extraction
	if cover, ok := extraction.Fields["cover"].(string); !ok || cover != "/covers/STARS-123.jpg" {
		t.Errorf("Expected cover '/covers/STARS-123.jpg', got %v", extraction.Fields["cover"])
	}

	// Test actresses extraction
	if actresses, ok := extraction.Fields["actresses"].([]string); ok {
		expected := []string{"Actress 1", "Actress 2"}
		if len(actresses) != len(expected) {
			t.Errorf("Expected %d actresses, got %d", len(expected), len(actresses))
		}
		for i, actress := range actresses {
			if actress != expected[i] {
				t.Errorf("Expected actress '%s' at index %d, got '%s'", expected[i], i, actress)
			}
		}
	} else {
		t.Errorf("Expected actresses slice, got %v", extraction.Fields["actresses"])
	}

	// Test genres extraction
	if genres, ok := extraction.Fields["genres"].([]string); ok {
		expected := []string{"Genre 1", "Genre 2"}
		if len(genres) != len(expected) {
			t.Errorf("Expected %d genres, got %d", len(expected), len(genres))
		}
	} else {
		t.Errorf("Expected genres slice, got %v", extraction.Fields["genres"])
	}

	// Test runtime extraction
	if runtime, ok := extraction.Fields["runtime"].(string); !ok || runtime != "120 minutes" {
		t.Errorf("Expected runtime '120 minutes', got %v", extraction.Fields["runtime"])
	}

	// Test release_date extraction
	if releaseDate, ok := extraction.Fields["release_date"].(string); !ok || releaseDate != "2023-12-01" {
		t.Errorf("Expected release_date '2023-12-01', got %v", extraction.Fields["release_date"])
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Normal text  ", "Normal text"},
		{"Text\n\nwith\t\tmultiple\n\nspaces", "Text with multiple spaces"},
		{"・Prefixed text", "Prefixed text"},
		{"Suffixed text・", "Suffixed text"},
		{"・Both sides・", "Both sides"},
		{"   ・Multi\n\nspaces・   ", "Multi spaces"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := CleanText(tt.input)
			if result != tt.expected {
				t.Errorf("CleanText(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123 minutes", 123},
		{"Runtime: 90min", 90},
		{"No numbers here", 0},
		{"Multiple 123 numbers 456", 123}, // Should return first number
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractNumber(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractNumber(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"Rating: 4.5", 4.5},
		{"9.2 out of 10", 9.2},
		{"100%", 100.0},
		{"No numbers", 0.0},
		{"", 0.0},
		{"3.14159", 3.14159},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractFloat(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractFloat(%q) = %f, expected %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		baseURL     string
		relativeURL string
		expected    string
	}{
		{"https://example.com", "http://full.url", "http://full.url"},
		{"https://example.com", "//cdn.example.com/image.jpg", "https://cdn.example.com/image.jpg"},
		{"https://example.com", "/path/to/resource", "https://example.com/path/to/resource"},
		{"https://example.com/", "/path/to/resource", "https://example.com/path/to/resource"},
		{"https://example.com", "relative/path", "https://example.com/relative/path"},
		{"https://example.com/", "relative/path", "https://example.com/relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.relativeURL, func(t *testing.T) {
			result := NormalizeURL(tt.baseURL, tt.relativeURL)
			if result != tt.expected {
				t.Errorf("NormalizeURL(%q, %q) = %q, expected %q", tt.baseURL, tt.relativeURL, result, tt.expected)
			}
		})
	}
}

func TestExtractDomainAndID(t *testing.T) {
	tests := []struct {
		url            string
		expectedDomain string
		expectedID     string
	}{
		{"https://www.javbus.com/STARS-123", "javbus", "STARS-123"},
		{"https://javdb.com/v/abc123", "javdb", "abc123"},
		{"https://www.javlib.com/en/?v=javli123", "javlib", "javli123"},
		{"https://www.dmm.co.jp/digital/videoa/-/detail/=/cid=abc00123/", "dmm", "abc00123"},
		{"https://unknown.com/movie/123", "", ""},
		{"invalid-url", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			domain, id := ExtractDomainAndID(tt.url)
			if domain != tt.expectedDomain || id != tt.expectedID {
				t.Errorf("ExtractDomainAndID(%q) = (%q, %q), expected (%q, %q)",
					tt.url, domain, id, tt.expectedDomain, tt.expectedID)
			}
		})
	}
}

func TestParserHTML(t *testing.T) {
	html := `<html><body><h1>Test</h1></body></html>`
	parser, err := NewParserFromString(html)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	resultHTML, err := parser.HTML()
	if err != nil {
		t.Fatalf("Failed to get HTML: %v", err)
	}

	if !strings.Contains(resultHTML, "<h1>Test</h1>") {
		t.Error("HTML should contain original content")
	}
}

func BenchmarkExtractText(b *testing.B) {
	html := `
	<html>
		<body>
			<div class="container">
				<h1>Title</h1>
				<p class="content">This is some content text</p>
				<div class="info">Additional information</div>
			</div>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ExtractText("h1")
	}
}

func BenchmarkExtractMovieInfo(b *testing.B) {
	html := `
	<html>
		<body>
			<h1>Movie Title</h1>
			<div class="cover"><img src="/cover.jpg"></div>
			<div class="actress"><a href="/actress/1">Actress 1</a><a href="/actress/2">Actress 2</a></div>
			<div class="genre"><a href="/genre/1">Genre 1</a><a href="/genre/2">Genre 2</a></div>
			<div class="runtime">120 minutes</div>
			<div class="release-date">2023-12-01</div>
		</body>
	</html>`

	parser, err := NewParserFromString(html)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ExtractMovieInfo()
	}
}