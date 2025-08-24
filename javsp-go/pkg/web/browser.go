package web

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Browser provides headless Chrome automation
type Browser struct {
	ctx       context.Context
	cancel    context.CancelFunc
	allocCtx  context.Context
	allocCancel context.CancelFunc
}

// BrowserOptions contains options for browser creation
type BrowserOptions struct {
	Headless      bool
	NoSandbox     bool
	DisableGPU    bool
	DisableImages bool
	WindowSize    [2]int
	UserAgent     string
	Timeout       time.Duration
}

// DefaultBrowserOptions returns default browser options
func DefaultBrowserOptions() *BrowserOptions {
	return &BrowserOptions{
		Headless:      true,
		NoSandbox:     true,
		DisableGPU:    true,
		DisableImages: true,
		WindowSize:    [2]int{1920, 1080},
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Timeout:       30 * time.Second,
	}
}

// NewBrowser creates a new browser instance
func NewBrowser(opts *BrowserOptions) (*Browser, error) {
	if opts == nil {
		opts = DefaultBrowserOptions()
	}

	// Chrome options
	chromeOpts := []chromedp.ExecAllocatorOption{
		chromedp.WindowSize(opts.WindowSize[0], opts.WindowSize[1]),
		chromedp.UserAgent(opts.UserAgent),
	}

	if opts.Headless {
		chromeOpts = append(chromeOpts, chromedp.Headless)
	}

	if opts.NoSandbox {
		chromeOpts = append(chromeOpts, chromedp.NoSandbox)
	}

	if opts.DisableGPU {
		chromeOpts = append(chromeOpts, chromedp.DisableGPU)
	}

	if opts.DisableImages {
		chromeOpts = append(chromeOpts,
			chromedp.Flag("blink-settings", "imagesEnabled=false"),
		)
	}

	// Additional flags for better compatibility
	chromeOpts = append(chromeOpts,
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)

	// Create allocator context
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), chromeOpts...)

	// Create browser context with timeout
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}

	browser := &Browser{
		ctx:         ctx,
		cancel:      cancel,
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
	}

	return browser, nil
}

// Navigate navigates to a URL and waits for the page to load
func (b *Browser) Navigate(url string) error {
	return chromedp.Run(b.ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)
}

// GetHTML returns the HTML content of the current page
func (b *Browser) GetHTML() (string, error) {
	var html string
	err := chromedp.Run(b.ctx,
		chromedp.OuterHTML("html", &html),
	)
	return html, err
}

// GetText returns text content from a selector
func (b *Browser) GetText(selector string) (string, error) {
	var text string
	err := chromedp.Run(b.ctx,
		chromedp.Text(selector, &text, chromedp.NodeVisible),
	)
	return text, err
}

// GetAttribute returns attribute value from an element
func (b *Browser) GetAttribute(selector, attribute string) (string, error) {
	var value string
	err := chromedp.Run(b.ctx,
		chromedp.AttributeValue(selector, attribute, &value, nil),
	)
	return value, err
}

// Click clicks on an element
func (b *Browser) Click(selector string) error {
	return chromedp.Run(b.ctx,
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	)
}

// WaitForElement waits for an element to be visible
func (b *Browser) WaitForElement(selector string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(b.ctx, timeout)
	defer cancel()

	return chromedp.Run(ctx,
		chromedp.WaitVisible(selector),
	)
}

// ExecuteScript executes JavaScript and returns the result
func (b *Browser) ExecuteScript(script string, result interface{}) error {
	return chromedp.Run(b.ctx,
		chromedp.Evaluate(script, result),
	)
}

// Screenshot takes a screenshot of the current page
func (b *Browser) Screenshot() ([]byte, error) {
	var buf []byte
	err := chromedp.Run(b.ctx,
		chromedp.FullScreenshot(&buf, 90),
	)
	return buf, err
}

// SetCookie sets a cookie
func (b *Browser) SetCookie(name, value, domain string) error {
	return chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetCookie(name, value).
				WithDomain(domain).
				Do(ctx)
		}),
	)
}

// GetCookies returns all cookies for the current domain
func (b *Browser) GetCookies() ([]*network.Cookie, error) {
	var cookies []*network.Cookie
	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, _ = network.GetCookies().Do(ctx)
			return nil
		}),
	)
	return cookies, err
}

// WaitForNavigation waits for navigation to complete
func (b *Browser) WaitForNavigation() error {
	return chromedp.Run(b.ctx,
		chromedp.WaitReady("body"),
	)
}

// HandleJavaScriptDialog handles JavaScript alerts/confirms
func (b *Browser) HandleJavaScriptDialog(accept bool, text string) error {
	return chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			chromedp.ListenTarget(ctx, func(ev interface{}) {
				switch ev.(type) {
				case *page.EventJavascriptDialogOpening:
					go func() {
						if accept {
							page.HandleJavaScriptDialog(true).Do(ctx)
						} else {
							page.HandleJavaScriptDialog(false).Do(ctx)
						}
					}()
				}
			})
			return nil
		}),
	)
}

// SolveCloudflare attempts to solve Cloudflare challenges
func (b *Browser) SolveCloudflare(url string, maxWait time.Duration) error {
	// Navigate to the page
	if err := b.Navigate(url); err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for Cloudflare challenge to complete
	ctx, cancel := context.WithTimeout(b.ctx, maxWait)
	defer cancel()

	// Check if we're on a Cloudflare challenge page
	var title string
	if err := chromedp.Run(ctx, chromedp.Title(&title)); err == nil {
		if strings.Contains(title, "Just a moment") || strings.Contains(title, "Checking your browser") {
			// Wait for challenge to complete (look for title change or specific elements)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(1 * time.Second):
					var newTitle string
					if err := chromedp.Run(ctx, chromedp.Title(&newTitle)); err != nil {
						return err
					}
					if newTitle != title && !strings.Contains(newTitle, "Just a moment") {
						return nil // Challenge completed
					}
				}
			}
		}
	}

	return nil
}

// Close closes the browser
func (b *Browser) Close() error {
	if b.cancel != nil {
		b.cancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
	return nil
}

// IsReady checks if the browser is ready
func (b *Browser) IsReady() bool {
	ctx, cancel := context.WithTimeout(b.ctx, 1*time.Second)
	defer cancel()

	var title string
	err := chromedp.Run(ctx, chromedp.Title(&title))
	return err == nil
}

// GetURL returns the current URL
func (b *Browser) GetURL() (string, error) {
	var url string
	err := chromedp.Run(b.ctx,
		chromedp.Location(&url),
	)
	return url, err
}

// Refresh refreshes the current page
func (b *Browser) Refresh() error {
	return chromedp.Run(b.ctx,
		chromedp.Reload(),
		chromedp.WaitReady("body"),
	)
}

// ScrollToBottom scrolls to the bottom of the page
func (b *Browser) ScrollToBottom() error {
	return chromedp.Run(b.ctx,
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil),
	)
}

// GetPageLoadTime returns page load time metrics
func (b *Browser) GetPageLoadTime() (map[string]float64, error) {
	var metrics map[string]float64
	script := `
		(function() {
			var t = performance.timing;
			return {
				'dns': t.domainLookupEnd - t.domainLookupStart,
				'connect': t.connectEnd - t.connectStart,
				'request': t.responseStart - t.requestStart,
				'response': t.responseEnd - t.responseStart,
				'dom': t.domContentLoadedEventEnd - t.responseEnd,
				'total': t.loadEventEnd - t.navigationStart
			};
		})()
	`
	
	err := chromedp.Run(b.ctx,
		chromedp.Evaluate(script, &metrics),
	)
	
	return metrics, err
}