//go:build unit

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultBrowserOptions(t *testing.T) {
	opts := DefaultBrowserOptions()
	
	if opts == nil {
		t.Fatal("DefaultBrowserOptions should not return nil")
	}
	
	if !opts.Headless {
		t.Error("Expected headless to be true")
	}
	
	if !opts.NoSandbox {
		t.Error("Expected no-sandbox to be true")
	}
	
	if !opts.DisableGPU {
		t.Error("Expected disable-gpu to be true")
	}
	
	if !opts.DisableImages {
		t.Error("Expected disable-images to be true")
	}
	
	if opts.WindowSize[0] != 1920 || opts.WindowSize[1] != 1080 {
		t.Errorf("Expected window size [1920, 1080], got [%d, %d]", opts.WindowSize[0], opts.WindowSize[1])
	}
	
	if opts.UserAgent == "" {
		t.Error("Expected non-empty user agent")
	}
	
	if !strings.Contains(opts.UserAgent, "Chrome") {
		t.Error("Expected user agent to contain 'Chrome'")
	}
	
	if opts.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", opts.Timeout)
	}
}

func TestNewBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	tests := []struct {
		name string
		opts *BrowserOptions
	}{
		{
			name: "default options",
			opts: nil,
		},
		{
			name: "custom options",
			opts: &BrowserOptions{
				Headless:      true,
				NoSandbox:     true,
				DisableGPU:    true,
				DisableImages: true,
				WindowSize:    [2]int{800, 600},
				UserAgent:     "Test User Agent",
				Timeout:       10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser, err := NewBrowser(tt.opts)
			if err != nil {
				t.Fatalf("Failed to create browser: %v", err)
			}
			
			if browser == nil {
				t.Fatal("Browser should not be nil")
			}
			
			defer browser.Close()
			
			// Test that browser context is set up
			if browser.ctx == nil {
				t.Error("Browser context should not be nil")
			}
			
			if browser.cancel == nil {
				t.Error("Browser cancel function should not be nil")
			}
		})
	}
}

// TestBrowserNavigate tests basic navigation functionality
// This test requires Chrome/Chromium to be installed
func TestBrowserNavigate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser navigation test in short mode")
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
				<head><title>Test Page</title></head>
				<body>
					<h1 id="main-title">Welcome to Test Page</h1>
					<p class="content">This is test content.</p>
					<a href="/link" id="test-link">Test Link</a>
					<input type="text" id="test-input" value="test value">
				</body>
			</html>
		`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:      true,
		NoSandbox:     true,
		DisableGPU:    true,
		DisableImages: true,
		Timeout:       10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser (Chrome might not be installed): %v", err)
	}
	defer browser.Close()

	// Test navigation
	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Test getting URL
	currentURL, err := browser.GetURL()
	if err != nil {
		t.Fatalf("Failed to get URL: %v", err)
	}
	
	if !strings.Contains(currentURL, server.URL) {
		t.Errorf("Expected URL to contain %s, got %s", server.URL, currentURL)
	}

	// Test getting HTML
	html, err := browser.GetHTML()
	if err != nil {
		t.Fatalf("Failed to get HTML: %v", err)
	}
	
	if !strings.Contains(html, "Welcome to Test Page") {
		t.Error("HTML should contain page title")
	}

	// Test getting text
	title, err := browser.GetText("#main-title")
	if err != nil {
		t.Fatalf("Failed to get text: %v", err)
	}
	
	if title != "Welcome to Test Page" {
		t.Errorf("Expected title 'Welcome to Test Page', got '%s'", title)
	}

	// Test getting attribute
	value, err := browser.GetAttribute("#test-input", "value")
	if err != nil {
		t.Fatalf("Failed to get attribute: %v", err)
	}
	
	if value != "test value" {
		t.Errorf("Expected value 'test value', got '%s'", value)
	}
}

func TestBrowserIsReady(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser ready test in short mode")
	}

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// Browser should not be ready before navigation
	if browser.IsReady() {
		t.Error("Browser should not be ready before navigation")
	}

	// Create simple test page
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Test</title></head><body>Test</body></html>`))
	}))
	defer server.Close()

	// Navigate and check readiness
	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	if !browser.IsReady() {
		t.Error("Browser should be ready after navigation")
	}
}

func TestBrowserWaitForElement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser wait test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body>
					<div id="immediate">Immediate element</div>
					<script>
						setTimeout(function() {
							var elem = document.createElement('div');
							elem.id = 'delayed';
							elem.textContent = 'Delayed element';
							document.body.appendChild(elem);
						}, 100);
					</script>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Wait for immediate element (should succeed quickly)
	if err := browser.WaitForElement("#immediate", 1*time.Second); err != nil {
		t.Errorf("Failed to find immediate element: %v", err)
	}

	// Wait for delayed element (should succeed after delay)
	if err := browser.WaitForElement("#delayed", 2*time.Second); err != nil {
		t.Errorf("Failed to find delayed element: %v", err)
	}

	// Wait for non-existent element (should timeout)
	err = browser.WaitForElement("#nonexistent", 100*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout when waiting for non-existent element")
	}
}

func TestBrowserExecuteScript(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser script test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><div id="test">Original</div></body></html>`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Execute script that returns a value
	var result string
	script := `document.getElementById('test').textContent`
	if err := browser.ExecuteScript(script, &result); err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}
	
	if result != "Original" {
		t.Errorf("Expected 'Original', got '%s'", result)
	}

	// Execute script that modifies the page
	script = `
		document.getElementById('test').textContent = 'Modified';
		return document.getElementById('test').textContent;
	`
	if err := browser.ExecuteScript(script, &result); err != nil {
		t.Fatalf("Failed to execute modification script: %v", err)
	}
	
	if result != "Modified" {
		t.Errorf("Expected 'Modified', got '%s'", result)
	}
}

func TestBrowserCookies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser cookie test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set a cookie from server side
		http.SetCookie(w, &http.Cookie{
			Name:  "server_cookie",
			Value: "server_value",
		})
		w.Write([]byte(`<html><body>Cookie test page</body></html>`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Set a cookie via browser
	if err := browser.SetCookie("test_cookie", "test_value", "127.0.0.1"); err != nil {
		t.Fatalf("Failed to set cookie: %v", err)
	}

	// Get cookies
	cookies, err := browser.GetCookies()
	if err != nil {
		t.Fatalf("Failed to get cookies: %v", err)
	}

	// Check that we have cookies
	if len(cookies) == 0 {
		t.Error("Expected at least one cookie")
	}

	// Look for our test cookie
	found := false
	for _, cookie := range cookies {
		if cookie.Name == "test_cookie" && cookie.Value == "test_value" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Test cookie not found")
	}
}

func TestBrowserScreenshot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser screenshot test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body style="background-color: red;">
					<h1>Screenshot Test</h1>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		WindowSize: [2]int{800, 600},
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Take screenshot
	screenshot, err := browser.Screenshot()
	if err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}

	if len(screenshot) == 0 {
		t.Error("Screenshot should not be empty")
	}

	// Basic check that it's a valid image (PNG header)
	if len(screenshot) < 8 || screenshot[0] != 0x89 || screenshot[1] != 0x50 || screenshot[2] != 0x4E || screenshot[3] != 0x47 {
		t.Error("Screenshot doesn't appear to be a valid PNG")
	}
}

func TestBrowserRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser refresh test in short mode")
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Write([]byte(`<html><body><div id="counter">` + string(rune('0'+requestCount)) + `</div></body></html>`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// Initial navigation
	if err := browser.Navigate(server.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Check initial content
	content1, err := browser.GetText("#counter")
	if err != nil {
		t.Fatalf("Failed to get initial content: %v", err)
	}

	// Refresh page
	if err := browser.Refresh(); err != nil {
		t.Fatalf("Failed to refresh: %v", err)
	}

	// Check content after refresh
	content2, err := browser.GetText("#counter")
	if err != nil {
		t.Fatalf("Failed to get content after refresh: %v", err)
	}

	if content1 == content2 {
		t.Error("Content should be different after refresh")
	}

	if requestCount < 2 {
		t.Errorf("Expected at least 2 requests, got %d", requestCount)
	}
}

// Mock test for Cloudflare solving (cannot test real Cloudflare challenges in unit tests)
func TestBrowserSolveCloudflare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser Cloudflare test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head><title>Normal Page</title></head>
				<body>
					<h1>Not a Cloudflare challenge</h1>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// This should succeed quickly since it's not a real Cloudflare page
	err = browser.SolveCloudflare(server.URL, 2*time.Second)
	if err != nil {
		t.Errorf("SolveCloudflare failed on normal page: %v", err)
	}
}

func TestBrowserClose(t *testing.T) {
	browser, err := NewBrowser(&BrowserOptions{
		Headless:   true,
		NoSandbox:  true,
		DisableGPU: true,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Skipf("Failed to create browser: %v", err)
	}

	// Test that Close doesn't panic
	err = browser.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Test multiple close calls don't panic
	err = browser.Close()
	if err != nil {
		t.Errorf("Multiple Close calls should not return error: %v", err)
	}
}