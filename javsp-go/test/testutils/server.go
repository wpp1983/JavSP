package testutils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// MockHTTPServer provides a mock HTTP server for testing crawlers
type MockHTTPServer struct {
	server    *httptest.Server
	responses map[string]*MockResponse
	requests  []MockRequest
	mu        sync.RWMutex
}

// MockResponse represents a mock HTTP response
type MockResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	Delay      int // milliseconds
}

// MockRequest represents a captured HTTP request
type MockRequest struct {
	Method string
	URL    string
	Headers map[string]string
	Body   string
}

// NewMockHTTPServer creates a new mock HTTP server
func NewMockHTTPServer() *MockHTTPServer {
	mock := &MockHTTPServer{
		responses: make(map[string]*MockResponse),
		requests:  make([]MockRequest, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// URL returns the server's URL
func (m *MockHTTPServer) URL() string {
	return m.server.URL
}

// Close closes the mock server
func (m *MockHTTPServer) Close() {
	m.server.Close()
}

// SetResponse sets a mock response for a specific path
func (m *MockHTTPServer) SetResponse(path string, response *MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[path] = response
}

// SetJavBusResponse sets a typical JavBus movie page response
func (m *MockHTTPServer) SetJavBusResponse(movieID string) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
</head>
<body>
    <div class="container">
        <h3>测试电影标题 %s</h3>
        <div class="info">
            <p><span class="header">識別碼:</span> %s</p>
            <p><span class="header">發行日期:</span> 2023-12-01</p>
            <p><span class="header">長度:</span> 120分鐘</p>
            <p><span class="header">導演:</span> 测试导演</p>
            <p><span class="header">製作商:</span> 测试制作商</p>
            <p><span class="header">發行商:</span> 测试发行商</p>
            <p><span class="header">系列:</span> 测试系列</p>
        </div>
        <div class="star-name">
            <a href="/actress/1">测试女优1</a>
            <a href="/actress/2">测试女优2</a>
        </div>
        <div class="genre">
            <a href="/genre/1">巨乳</a>
            <a href="/genre/2">单体作品</a>
            <a href="/genre/3">中出</a>
        </div>
        <div class="bigImage">
            <a href="https://example.com/covers/%s.jpg">
                <img src="https://example.com/covers/%s.jpg" alt="cover">
            </a>
        </div>
    </div>
</body>
</html>`, movieID, movieID, movieID, movieID, movieID)

	m.SetResponse("/"+movieID, &MockResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
		Body:       html,
	})
}

// SetAVWikiResponse sets a typical AVWiki movie page response
func (m *MockHTTPServer) SetAVWikiResponse(movieID string) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s - AVWiki</title>
</head>
<body>
    <div id="mw-content-text">
        <table class="infobox">
            <tr>
                <th>品番</th>
                <td>%s</td>
            </tr>
            <tr>
                <th>发售日</th>
                <td>2023年12月1日</td>
            </tr>
            <tr>
                <th>时长</th>
                <td>120分钟</td>
            </tr>
            <tr>
                <th>导演</th>
                <td>测试导演</td>
            </tr>
            <tr>
                <th>制作商</th>
                <td>测试制作商</td>
            </tr>
            <tr>
                <th>发行商</th>
                <td>测试发行商</td>
            </tr>
        </table>
        <div class="actress-info">
            <a href="/wiki/测试女优1">测试女优1</a>
            <a href="/wiki/测试女优2">测试女优2</a>
        </div>
        <div class="categories">
            <a href="/category/巨乳">巨乳</a>
            <a href="/category/单体作品">单体作品</a>
        </div>
    </div>
</body>
</html>`, movieID, movieID)

	m.SetResponse("/wiki/"+movieID, &MockResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
		Body:       html,
	})
}

// Set404Response sets a 404 response for a path
func (m *MockHTTPServer) Set404Response(path string) {
	m.SetResponse(path, &MockResponse{
		StatusCode: 404,
		Headers:    map[string]string{"Content-Type": "text/html"},
		Body:       "<html><body><h1>404 Not Found</h1></body></html>",
	})
}

// SetTimeoutResponse sets a response that will timeout
func (m *MockHTTPServer) SetTimeoutResponse(path string) {
	m.SetResponse(path, &MockResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/html"},
		Body:       "<html><body>This will timeout</body></html>",
		Delay:      60000, // 60 seconds
	})
}

// GetRequests returns all captured requests
func (m *MockHTTPServer) GetRequests() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	requests := make([]MockRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

// ClearRequests clears all captured requests
func (m *MockHTTPServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}

// RequestCount returns the number of requests made
func (m *MockHTTPServer) RequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.requests)
}

// handleRequest handles incoming HTTP requests
func (m *MockHTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Capture the request
	m.captureRequest(r)

	// Find matching response
	m.mu.RLock()
	response, exists := m.responses[r.URL.Path]
	m.mu.RUnlock()

	if !exists {
		// Default 404 response
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
		return
	}

	// Apply delay if specified
	if response.Delay > 0 {
		// In real implementation, you might want to use time.Sleep(time.Duration(response.Delay) * time.Millisecond)
		// For testing purposes, we'll skip the delay to keep tests fast
	}

	// Set headers
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(response.StatusCode)

	// Write body
	w.Write([]byte(response.Body))
}

// captureRequest captures request details
func (m *MockHTTPServer) captureRequest(r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	headers := make(map[string]string)
	for key, values := range r.Header {
		headers[key] = strings.Join(values, ", ")
	}

	// For simplicity, we're not reading the body here
	// In a more complete implementation, you'd want to read and store it
	
	request := MockRequest{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: headers,
		Body:    "",
	}

	m.requests = append(m.requests, request)
}

// CreateJavBusTestServer creates a mock JavBus server with common responses
func CreateJavBusTestServer() *MockHTTPServer {
	server := NewMockHTTPServer()
	
	// Set up common JavBus responses
	testMovies := []string{"STARS-123", "SSIS-001", "IPX-177", "MIDE-456"}
	for _, movieID := range testMovies {
		server.SetJavBusResponse(movieID)
	}
	
	// Set up 404 for unknown movies
	server.Set404Response("/UNKNOWN-123")
	
	return server
}

// CreateAVWikiTestServer creates a mock AVWiki server with common responses  
func CreateAVWikiTestServer() *MockHTTPServer {
	server := NewMockHTTPServer()
	
	// Set up common AVWiki responses
	testMovies := []string{"STARS-123", "SSIS-001", "IPX-177", "MIDE-456"}
	for _, movieID := range testMovies {
		server.SetAVWikiResponse(movieID)
	}
	
	// Set up 404 for unknown movies
	server.Set404Response("/wiki/UNKNOWN-123")
	
	return server
}