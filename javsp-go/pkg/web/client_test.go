//go:build unit

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"net/url"

	"javsp-go/internal/config"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		opts    *ClientOptions
		wantErr bool
	}{
		{
			name: "default options",
			opts: nil,
			wantErr: false,
		},
		{
			name: "custom options",
			opts: &ClientOptions{
				Timeout:        10 * time.Second,
				MaxIdleConns:   50,
				MaxConnsPerHost: 5,
				EnableCookies:  true,
				SkipTLSVerify:  false,
				RateLimit:      2 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "with invalid proxy",
			opts: &ClientOptions{
				ProxyURL: "invalid-url",
			},
			wantErr: true,
		},
		{
			name: "with valid HTTP proxy",
			opts: &ClientOptions{
				ProxyURL: "http://proxy.example.com:8080",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.opts)
			
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if client == nil {
				t.Error("Expected client, got nil")
				return
			}
			
			// Verify client properties
			if tt.opts == nil {
				tt.opts = DefaultClientOptions()
			}
			
			if client.httpClient.Timeout != tt.opts.Timeout {
				t.Errorf("Expected timeout %v, got %v", tt.opts.Timeout, client.httpClient.Timeout)
			}
			
			if tt.opts.EnableCookies && client.cookieJar == nil {
				t.Error("Expected cookie jar to be enabled")
			}
			
			if len(client.userAgents) == 0 {
				t.Error("Expected user agents to be set")
			}
		})
	}
}

func TestNewClientWithConfig(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Network.Timeout = 15 * time.Second
	proxyURL := "http://proxy.test:8080"
	cfg.Network.ProxyServer = &proxyURL

	client, err := NewClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create client with config: %v", err)
	}

	if client.config != cfg {
		t.Error("Config not set correctly")
	}

	if client.httpClient.Timeout != cfg.Network.Timeout {
		t.Errorf("Expected timeout %v, got %v", cfg.Network.Timeout, client.httpClient.Timeout)
	}
}

func TestDefaultUserAgents(t *testing.T) {
	userAgents := DefaultUserAgents()
	
	if len(userAgents) == 0 {
		t.Error("Expected non-empty user agents list")
	}
	
	for _, ua := range userAgents {
		if ua == "" {
			t.Error("Found empty user agent")
		}
		if !strings.Contains(ua, "Mozilla") {
			t.Error("User agent should contain 'Mozilla'")
		}
	}
}

func TestClientGet(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		
		// Check headers
		if r.Header.Get("User-Agent") == "" {
			t.Error("Missing User-Agent header")
		}
		
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Test Response</body></html>"))
	}))
	defer server.Close()

	client, err := NewClient(nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if contentType := resp.Header.Get("Content-Type"); contentType != "text/html" {
		t.Errorf("Expected content type 'text/html', got '%s'", contentType)
	}
}

func TestClientPost(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}
		
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status": "created"}`))
	}))
	defer server.Close()

	client, err := NewClient(nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	body := strings.NewReader(`{"test": "data"}`)
	resp, err := client.Post(ctx, server.URL, "application/json", body)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

func TestClientRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	}))
	defer server.Close()

	client, err := NewClient(&ClientOptions{
		Timeout:   10 * time.Second,
		RateLimit: 10 * time.Millisecond, // Speed up test
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	resp, err := client.Get(ctx, server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClientMaxRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(&ClientOptions{
		Timeout:   5 * time.Second,
		RateLimit: 10 * time.Millisecond, // Speed up test
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	_, err = client.Get(ctx, server.URL)
	if err == nil {
		t.Fatal("Expected error due to max retries exceeded")
	}

	expectedAttempts := 4 // 1 initial + 3 retries
	if attempts != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, attempts)
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{http.StatusOK, false},
		{http.StatusNotFound, false},
		{http.StatusBadRequest, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			result := shouldRetry(tt.statusCode)
			if result != tt.expected {
				t.Errorf("shouldRetry(%d) = %v, expected %v", tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestUserAgentRotation(t *testing.T) {
	userAgents := []string{"UA1", "UA2", "UA3"}
	client, err := NewClient(&ClientOptions{UserAgents: userAgents})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test user agent rotation
	for i := 0; i < len(userAgents)*2; i++ {
		ua := client.getNextUserAgent()
		expected := userAgents[i%len(userAgents)]
		if ua != expected {
			t.Errorf("Iteration %d: expected %s, got %s", i, expected, ua)
		}
	}
}

func TestClientCookies(t *testing.T) {
	client, err := NewClient(&ClientOptions{EnableCookies: true})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	testURL, _ := url.Parse("http://example.com")
	cookie := &http.Cookie{
		Name:  "test",
		Value: "value",
	}

	// Set cookie
	client.SetCookie(testURL, cookie)

	// Get cookies
	cookies := client.GetCookies(testURL)
	if len(cookies) == 0 {
		t.Error("Expected at least one cookie")
		return
	}

	found := false
	for _, c := range cookies {
		if c.Name == "test" && c.Value == "value" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Cookie not found")
	}
}

func TestRateLimiter(t *testing.T) {
	rateLimiter := &RateLimiter{
		minInterval: 100 * time.Millisecond,
	}

	start := time.Now()
	
	// First call should not wait
	rateLimiter.Wait()
	elapsed1 := time.Since(start)
	
	// Second call should wait
	rateLimiter.Wait()
	elapsed2 := time.Since(start)

	if elapsed1 > 10*time.Millisecond {
		t.Errorf("First call should not wait, took %v", elapsed1)
	}

	if elapsed2 < 100*time.Millisecond {
		t.Errorf("Second call should wait at least 100ms, took %v", elapsed2)
	}
}

func TestClientStats(t *testing.T) {
	userAgents := []string{"UA1", "UA2"}
	client, err := NewClient(&ClientOptions{
		UserAgents:    userAgents,
		EnableCookies: true,
		RateLimit:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	stats := client.GetStats()

	if stats["user_agents_count"] != len(userAgents) {
		t.Errorf("Expected user_agents_count %d, got %v", len(userAgents), stats["user_agents_count"])
	}

	if stats["current_user_agent"] != userAgents[0] {
		t.Errorf("Expected current_user_agent %s, got %v", userAgents[0], stats["current_user_agent"])
	}

	if stats["has_cookies"] != true {
		t.Error("Expected has_cookies to be true")
	}

	if stats["rate_limit"] != 1*time.Second {
		t.Errorf("Expected rate_limit 1s, got %v", stats["rate_limit"])
	}
}

func TestContextCancellation(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.Get(ctx, server.URL)
	if err == nil {
		t.Error("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func BenchmarkClientGet(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	client, err := NewClient(&ClientOptions{RateLimit: 0}) // Disable rate limiting for benchmark
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(ctx, server.URL)
			if err != nil {
				b.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()
		}
	})
}