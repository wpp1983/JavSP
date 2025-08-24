package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"javsp-go/internal/config"
)

// Client provides an advanced HTTP client with retry, proxy, and rate limiting
type Client struct {
	httpClient       *http.Client
	config           *config.Config
	userAgents       []string
	uaIndex          int
	uaMutex          sync.RWMutex
	rateLimiter      *RateLimiter
	cookieJar        http.CookieJar
	lastRequest      time.Time
	requestMutex     sync.Mutex
	progressCallback ProgressCallback
}

// ClientOptions contains options for creating a new client
type ClientOptions struct {
	Timeout        time.Duration
	MaxIdleConns   int
	MaxConnsPerHost int
	ProxyURL       string
	UserAgents     []string
	EnableCookies  bool
	SkipTLSVerify  bool
	RateLimit      time.Duration
}

// ProgressCallback is called during operations with progress updates
type ProgressCallback func(message string, elapsed time.Duration, remaining time.Duration)

// RateLimiter controls request frequency
type RateLimiter struct {
	minInterval      time.Duration
	lastRequest      time.Time
	mutex           sync.Mutex
	progressCallback ProgressCallback
}

// NewClient creates a new HTTP client with advanced features
func NewClient(opts *ClientOptions) (*Client, error) {
	if opts == nil {
		opts = DefaultClientOptions()
	}

	// Create transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          opts.MaxIdleConns,
		MaxIdleConnsPerHost:   opts.MaxConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.SkipTLSVerify,
		},
	}

	// Setup proxy if specified
	if opts.ProxyURL != "" {
		if err := setupProxy(transport, opts.ProxyURL); err != nil {
			return nil, fmt.Errorf("failed to setup proxy: %w", err)
		}
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
	}

	// Setup cookie jar if enabled
	var jar http.CookieJar
	if opts.EnableCookies {
		var err error
		jar, err = cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create cookie jar: %w", err)
		}
		httpClient.Jar = jar
	}

	client := &Client{
		httpClient:  httpClient,
		userAgents:  opts.UserAgents,
		cookieJar:   jar,
		rateLimiter: &RateLimiter{minInterval: opts.RateLimit},
	}

	return client, nil
}

// NewClientWithConfig creates a client from JavSP config
func NewClientWithConfig(cfg *config.Config) (*Client, error) {
	opts := &ClientOptions{
		Timeout:        cfg.Network.Timeout,
		MaxIdleConns:   100,
		MaxConnsPerHost: 10,
		EnableCookies:  true,
		SkipTLSVerify:  false,
		RateLimit:      cfg.Crawler.SleepAfterScraping,
		UserAgents:     DefaultUserAgents(),
	}

	if cfg.Network.ProxyServer != nil && *cfg.Network.ProxyServer != "" {
		opts.ProxyURL = *cfg.Network.ProxyServer
	}

	client, err := NewClient(opts)
	if err != nil {
		return nil, err
	}

	client.config = cfg
	return client, nil
}

// DefaultClientOptions returns default client options
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		Timeout:        30 * time.Second,
		MaxIdleConns:   100,
		MaxConnsPerHost: 10,
		UserAgents:     DefaultUserAgents(),
		EnableCookies:  true,
		SkipTLSVerify:  false,
		RateLimit:      1 * time.Second,
	}
}

// DefaultUserAgents returns a list of common user agents
func DefaultUserAgents() []string {
	return []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// Get performs a GET request with retry logic
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.DoWithRetry(req)
}

// Post performs a POST request with retry logic
func (c *Client) Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return c.DoWithRetry(req)
}

// DoWithRetry performs the request with retry logic
func (c *Client) DoWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := 3
	if c.config != nil {
		maxRetries = c.config.Network.Retry
	}

	// Add default headers
	c.addHeaders(req)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			retryDelay := backoff + jitter
			
			// Show retry progress if callback is set
			if c.progressCallback != nil {
				start := time.Now()
				message := fmt.Sprintf("Retrying (attempt %d/%d)", attempt+1, maxRetries+1)
				for {
					elapsed := time.Since(start)
					remaining := retryDelay - elapsed
					
					if remaining <= 0 {
						break
					}
					
					c.progressCallback(message, elapsed, remaining)
					time.Sleep(100 * time.Millisecond)
				}
			} else {
				time.Sleep(retryDelay)
			}
		}

		// Rate limiting
		c.rateLimiter.Wait()

		// Perform request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if we should retry based on status code
		if shouldRetry(resp.StatusCode) && attempt < maxRetries-1 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

// addHeaders adds common headers to the request
func (c *Client) addHeaders(req *http.Request) {
	// Set User-Agent
	req.Header.Set("User-Agent", c.getNextUserAgent())

	// Set common headers
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Add some randomness to headers
	if rand.Float32() < 0.5 {
		req.Header.Set("Cache-Control", "no-cache")
	}
}

// getNextUserAgent returns the next user agent in rotation
func (c *Client) getNextUserAgent() string {
	c.uaMutex.Lock()
	defer c.uaMutex.Unlock()

	if len(c.userAgents) == 0 {
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}

	ua := c.userAgents[c.uaIndex]
	c.uaIndex = (c.uaIndex + 1) % len(c.userAgents)
	return ua
}

// shouldRetry determines if a request should be retried based on status code
func shouldRetry(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// setupProxy configures proxy for the transport
func setupProxy(transport *http.Transport, proxyURL string) error {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp", u.Host, nil, proxy.Direct)
		if err != nil {
			return fmt.Errorf("failed to create SOCKS5 proxy: %w", err)
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	default:
		return fmt.Errorf("unsupported proxy scheme: %s", u.Scheme)
	}

	return nil
}

// Wait implements rate limiting with progress callback
func (rl *RateLimiter) Wait() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	if rl.minInterval <= 0 {
		return
	}

	elapsed := time.Since(rl.lastRequest)
	if elapsed < rl.minInterval {
		waitTime := rl.minInterval - elapsed
		
		// Show progress if callback is set and wait time is significant
		if rl.progressCallback != nil && waitTime > 100*time.Millisecond {
			start := time.Now()
			for {
				waitElapsed := time.Since(start)
				remaining := waitTime - waitElapsed
				
				if remaining <= 0 {
					break
				}
				
				rl.progressCallback("Rate limiting", waitElapsed, remaining)
				time.Sleep(50 * time.Millisecond)
			}
		} else {
			time.Sleep(waitTime)
		}
	}

	rl.lastRequest = time.Now()
}

// SetCookie adds a cookie to the jar
func (c *Client) SetCookie(u *url.URL, cookie *http.Cookie) {
	if c.cookieJar != nil {
		c.cookieJar.SetCookies(u, []*http.Cookie{cookie})
	}
}

// GetCookies returns cookies for a URL
func (c *Client) GetCookies(u *url.URL) []*http.Cookie {
	if c.cookieJar != nil {
		return c.cookieJar.Cookies(u)
	}
	return nil
}

// SetProgressCallback sets the progress callback for the client
func (c *Client) SetProgressCallback(callback ProgressCallback) {
	c.progressCallback = callback
	if c.rateLimiter != nil {
		c.rateLimiter.progressCallback = callback
	}
}

// Close cleans up client resources
func (c *Client) Close() error {
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}

// GetStats returns client statistics
func (c *Client) GetStats() map[string]interface{} {
	c.uaMutex.RLock()
	currentUA := ""
	if len(c.userAgents) > 0 {
		currentUA = c.userAgents[c.uaIndex]
	}
	c.uaMutex.RUnlock()

	c.requestMutex.Lock()
	lastRequest := c.lastRequest
	c.requestMutex.Unlock()

	return map[string]interface{}{
		"user_agents_count": len(c.userAgents),
		"current_user_agent": currentUA,
		"last_request_time":  lastRequest,
		"has_cookies":        c.cookieJar != nil,
		"rate_limit":         c.rateLimiter.minInterval,
	}
}