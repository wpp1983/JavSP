package errors

import (
	"fmt"
	"strings"
	"time"
)

// ErrorType represents the type of error
type ErrorType int

const (
	// ErrorTypeNetwork represents network-related errors
	ErrorTypeNetwork ErrorType = iota
	// ErrorTypeHTTP represents HTTP status errors
	ErrorTypeHTTP
	// ErrorTypeParsing represents HTML parsing errors
	ErrorTypeParsing
	// ErrorTypeValidation represents data validation errors
	ErrorTypeValidation
	// ErrorTypeTimeout represents timeout errors
	ErrorTypeTimeout
	// ErrorTypeNotFound represents resource not found errors
	ErrorTypeNotFound
	// ErrorTypeRateLimit represents rate limiting errors
	ErrorTypeRateLimit
	// ErrorTypeAuth represents authentication errors
	ErrorTypeAuth
	// ErrorTypeUnknown represents unknown errors
	ErrorTypeUnknown
)

// String returns the string representation of ErrorType
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeNetwork:
		return "NETWORK"
	case ErrorTypeHTTP:
		return "HTTP"
	case ErrorTypeParsing:
		return "PARSING"
	case ErrorTypeValidation:
		return "VALIDATION"
	case ErrorTypeTimeout:
		return "TIMEOUT"
	case ErrorTypeNotFound:
		return "NOT_FOUND"
	case ErrorTypeRateLimit:
		return "RATE_LIMIT"
	case ErrorTypeAuth:
		return "AUTH"
	default:
		return "UNKNOWN"
	}
}

// CrawlerError represents an error that occurred during crawling
type CrawlerError struct {
	Type      ErrorType `json:"type"`
	Source    string    `json:"source"`
	MovieID   string    `json:"movie_id,omitempty"`
	URL       string    `json:"url,omitempty"`
	Message   string    `json:"message"`
	Cause     error     `json:"cause,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Retryable bool      `json:"retryable"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *CrawlerError) Error() string {
	if e.MovieID != "" {
		return fmt.Sprintf("[%s] %s: %s (MovieID: %s)", e.Type, e.Source, e.Message, e.MovieID)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Source, e.Message)
}

// Unwrap returns the underlying error
func (e *CrawlerError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether this error is retryable
func (e *CrawlerError) IsRetryable() bool {
	return e.Retryable
}

// WithContext adds context information to the error
func (e *CrawlerError) WithContext(key string, value interface{}) *CrawlerError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewCrawlerError creates a new CrawlerError
func NewCrawlerError(errorType ErrorType, source, message string, cause error) *CrawlerError {
	return &CrawlerError{
		Type:      errorType,
		Source:    source,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryable(errorType, cause),
	}
}

// NewNetworkError creates a network-related error
func NewNetworkError(source, message string, cause error) *CrawlerError {
	return NewCrawlerError(ErrorTypeNetwork, source, message, cause)
}

// NewHTTPError creates an HTTP-related error
func NewHTTPError(source string, statusCode int, cause error) *CrawlerError {
	message := fmt.Sprintf("HTTP %d error", statusCode)
	err := NewCrawlerError(ErrorTypeHTTP, source, message, cause)
	err.WithContext("status_code", statusCode)
	return err
}

// NewParsingError creates a parsing-related error
func NewParsingError(source, message string, cause error) *CrawlerError {
	return NewCrawlerError(ErrorTypeParsing, source, message, cause)
}

// NewValidationError creates a validation-related error
func NewValidationError(source, message string, cause error) *CrawlerError {
	err := NewCrawlerError(ErrorTypeValidation, source, message, cause)
	err.Retryable = false // Validation errors are usually not retryable
	return err
}

// NewTimeoutError creates a timeout-related error
func NewTimeoutError(source, message string, cause error) *CrawlerError {
	return NewCrawlerError(ErrorTypeTimeout, source, message, cause)
}

// NewNotFoundError creates a not-found error
func NewNotFoundError(source, movieID string) *CrawlerError {
	err := NewCrawlerError(ErrorTypeNotFound, source, "Movie not found", nil)
	err.MovieID = movieID
	err.Retryable = false // Not found errors are not retryable
	return err
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(source, message string) *CrawlerError {
	err := NewCrawlerError(ErrorTypeRateLimit, source, message, nil)
	err.Retryable = true // Rate limit errors are retryable after delay
	return err
}

// isRetryable determines if an error type is generally retryable
func isRetryable(errorType ErrorType, cause error) bool {
	switch errorType {
	case ErrorTypeNetwork, ErrorTypeHTTP, ErrorTypeTimeout, ErrorTypeRateLimit:
		return true
	case ErrorTypeNotFound, ErrorTypeValidation, ErrorTypeAuth:
		return false
	case ErrorTypeParsing:
		// Parsing errors might be retryable if they're due to temporary issues
		return true
	case ErrorTypeUnknown:
		// Be conservative with unknown errors
		if cause != nil {
			return isRetryableFromCause(cause)
		}
		return false
	default:
		return false
	}
}

// isRetryableFromCause determines retryability based on the underlying error
func isRetryableFromCause(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	
	// Retryable network errors
	retryablePatterns := []string{
		"timeout", "connection refused", "connection reset",
		"temporary failure", "network unreachable", "host unreachable",
		"i/o timeout", "context deadline exceeded", "too many requests",
		"service unavailable", "bad gateway", "gateway timeout",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Non-retryable errors
	nonRetryablePatterns := []string{
		"not found", "unauthorized", "forbidden", "bad request",
		"validation failed", "invalid", "malformed",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	return false
}

// ClassifyError attempts to classify an unknown error into a CrawlerError
func ClassifyError(source string, err error) *CrawlerError {
	if err == nil {
		return nil
	}

	// If it's already a CrawlerError, return it
	if crawlerErr, ok := err.(*CrawlerError); ok {
		return crawlerErr
	}

	errStr := strings.ToLower(err.Error())

	// Classify by error message patterns
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return NewTimeoutError(source, "Operation timed out", err)
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "network"):
		return NewNetworkError(source, "Network connection failed", err)
	case strings.Contains(errStr, "not found") || strings.Contains(errStr, "404"):
		return NewCrawlerError(ErrorTypeNotFound, source, "Resource not found", err)
	case strings.Contains(errStr, "too many requests") || strings.Contains(errStr, "rate limit"):
		return NewRateLimitError(source, "Rate limit exceeded")
	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "forbidden"):
		return NewCrawlerError(ErrorTypeAuth, source, "Authentication failed", err)
	case strings.Contains(errStr, "parse") || strings.Contains(errStr, "invalid html"):
		return NewParsingError(source, "Failed to parse response", err)
	case strings.Contains(errStr, "validation"):
		return NewValidationError(source, "Data validation failed", err)
	default:
		return NewCrawlerError(ErrorTypeUnknown, source, "Unknown error occurred", err)
	}
}

// RetryConfig contains configuration for retry logic
type RetryConfig struct {
	MaxAttempts    int           `json:"max_attempts"`
	InitialDelay   time.Duration `json:"initial_delay"`
	MaxDelay       time.Duration `json:"max_delay"`
	BackoffFactor  float64       `json:"backoff_factor"`
	JitterPercent  float64       `json:"jitter_percent"`
	RetryableTypes []ErrorType   `json:"retryable_types"`
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterPercent: 0.1,
		RetryableTypes: []ErrorType{
			ErrorTypeNetwork,
			ErrorTypeHTTP,
			ErrorTypeTimeout,
			ErrorTypeRateLimit,
			ErrorTypeParsing,
		},
	}
}

// ShouldRetry determines if an error should be retried based on the config
func (rc *RetryConfig) ShouldRetry(err error, attempt int) bool {
	if attempt >= rc.MaxAttempts {
		return false
	}

	crawlerErr := ClassifyError("unknown", err)
	if crawlerErr == nil {
		return false
	}

	// Check if the error type is in the retryable list
	for _, retryableType := range rc.RetryableTypes {
		if crawlerErr.Type == retryableType {
			return crawlerErr.IsRetryable()
		}
	}

	return false
}

// CalculateDelay calculates the delay for the next retry attempt
func (rc *RetryConfig) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.InitialDelay
	}

	// Calculate exponential backoff
	delay := float64(rc.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= rc.BackoffFactor
	}

	// Apply maximum delay limit
	if delay > float64(rc.MaxDelay) {
		delay = float64(rc.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if rc.JitterPercent > 0 {
		jitter := delay * rc.JitterPercent * (0.5 - (float64(time.Now().UnixNano()%1000000) / 1000000))
		delay += jitter
	}

	return time.Duration(delay)
}

// ErrorStats tracks error statistics
type ErrorStats struct {
	TotalErrors     int64                   `json:"total_errors"`
	ErrorsByType    map[ErrorType]int64     `json:"errors_by_type"`
	ErrorsBySource  map[string]int64        `json:"errors_by_source"`
	RetryableErrors int64                   `json:"retryable_errors"`
	LastError       *CrawlerError           `json:"last_error,omitempty"`
	LastUpdated     time.Time               `json:"last_updated"`
}

// NewErrorStats creates a new ErrorStats instance
func NewErrorStats() *ErrorStats {
	return &ErrorStats{
		ErrorsByType:   make(map[ErrorType]int64),
		ErrorsBySource: make(map[string]int64),
		LastUpdated:    time.Now(),
	}
}

// RecordError records an error in the statistics
func (es *ErrorStats) RecordError(err *CrawlerError) {
	if err == nil {
		return
	}

	es.TotalErrors++
	es.ErrorsByType[err.Type]++
	es.ErrorsBySource[err.Source]++
	
	if err.IsRetryable() {
		es.RetryableErrors++
	}
	
	es.LastError = err
	es.LastUpdated = time.Now()
}

// GetErrorRate returns the error rate for a specific type
func (es *ErrorStats) GetErrorRate(errorType ErrorType) float64 {
	if es.TotalErrors == 0 {
		return 0.0
	}
	return float64(es.ErrorsByType[errorType]) / float64(es.TotalErrors)
}

// GetRetryableRate returns the percentage of errors that are retryable
func (es *ErrorStats) GetRetryableRate() float64 {
	if es.TotalErrors == 0 {
		return 0.0
	}
	return float64(es.RetryableErrors) / float64(es.TotalErrors)
}

// Reset resets all error statistics
func (es *ErrorStats) Reset() {
	es.TotalErrors = 0
	es.ErrorsByType = make(map[ErrorType]int64)
	es.ErrorsBySource = make(map[string]int64)
	es.RetryableErrors = 0
	es.LastError = nil
	es.LastUpdated = time.Now()
}