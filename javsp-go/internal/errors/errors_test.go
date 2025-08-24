package errors

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeNetwork, "NETWORK"},
		{ErrorTypeHTTP, "HTTP"},
		{ErrorTypeParsing, "PARSING"},
		{ErrorTypeValidation, "VALIDATION"},
		{ErrorTypeTimeout, "TIMEOUT"},
		{ErrorTypeNotFound, "NOT_FOUND"},
		{ErrorTypeRateLimit, "RATE_LIMIT"},
		{ErrorTypeAuth, "AUTH"},
		{ErrorTypeUnknown, "UNKNOWN"},
		{ErrorType(999), "UNKNOWN"}, // Test unknown type
	}

	for _, test := range tests {
		result := test.errorType.String()
		if result != test.expected {
			t.Errorf("ErrorType(%d).String(): expected '%s', got '%s'", 
				int(test.errorType), test.expected, result)
		}
	}
}

func TestNewCrawlerError(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := NewCrawlerError(ErrorTypeNetwork, "test-source", "test message", cause)

	if err.Type != ErrorTypeNetwork {
		t.Errorf("Expected type %v, got %v", ErrorTypeNetwork, err.Type)
	}

	if err.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got '%s'", err.Source)
	}

	if err.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause to be set")
	}

	if err.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Network errors should be retryable by default
	if !err.IsRetryable() {
		t.Error("Network errors should be retryable")
	}
}

func TestCrawlerError_Error(t *testing.T) {
	// Test without movie ID
	err1 := NewCrawlerError(ErrorTypeNetwork, "test-source", "test message", nil)
	expected1 := "[NETWORK] test-source: test message"
	if err1.Error() != expected1 {
		t.Errorf("Expected '%s', got '%s'", expected1, err1.Error())
	}

	// Test with movie ID
	err2 := NewCrawlerError(ErrorTypeNotFound, "test-source", "not found", nil)
	err2.MovieID = "TEST-123"
	expected2 := "[NOT_FOUND] test-source: not found (MovieID: TEST-123)"
	if err2.Error() != expected2 {
		t.Errorf("Expected '%s', got '%s'", expected2, err2.Error())
	}
}

func TestCrawlerError_WithContext(t *testing.T) {
	err := NewCrawlerError(ErrorTypeHTTP, "test-source", "HTTP error", nil)
	err.WithContext("status_code", 404)
	err.WithContext("url", "http://example.com")

	if len(err.Context) != 2 {
		t.Errorf("Expected 2 context items, got %d", len(err.Context))
	}

	if err.Context["status_code"] != 404 {
		t.Errorf("Expected status_code 404, got %v", err.Context["status_code"])
	}

	if err.Context["url"] != "http://example.com" {
		t.Errorf("Expected url 'http://example.com', got %v", err.Context["url"])
	}
}

func TestNewHTTPError(t *testing.T) {
	cause := fmt.Errorf("HTTP request failed")
	err := NewHTTPError("test-source", 503, cause)

	if err.Type != ErrorTypeHTTP {
		t.Errorf("Expected HTTP error type, got %v", err.Type)
	}

	if !strings.Contains(err.Message, "503") {
		t.Errorf("Expected message to contain status code, got '%s'", err.Message)
	}

	if err.Context["status_code"] != 503 {
		t.Errorf("Expected status_code 503 in context, got %v", err.Context["status_code"])
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("test-source", "TEST-123")

	if err.Type != ErrorTypeNotFound {
		t.Errorf("Expected NOT_FOUND error type, got %v", err.Type)
	}

	if err.MovieID != "TEST-123" {
		t.Errorf("Expected movie ID 'TEST-123', got '%s'", err.MovieID)
	}

	if err.IsRetryable() {
		t.Error("Not found errors should not be retryable")
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		inputError    error
		expectedType  ErrorType
		shouldRetry   bool
	}{
		{fmt.Errorf("connection timeout"), ErrorTypeTimeout, true},
		{fmt.Errorf("network unreachable"), ErrorTypeNetwork, true},
		{fmt.Errorf("404 not found"), ErrorTypeNotFound, false},
		{fmt.Errorf("too many requests"), ErrorTypeRateLimit, true},
		{fmt.Errorf("unauthorized access"), ErrorTypeAuth, false},
		{fmt.Errorf("failed to parse HTML"), ErrorTypeParsing, true},
		{fmt.Errorf("validation failed"), ErrorTypeValidation, false},
		{fmt.Errorf("some unknown error"), ErrorTypeUnknown, false},
	}

	for _, test := range tests {
		result := ClassifyError("test-source", test.inputError)
		
		if result.Type != test.expectedType {
			t.Errorf("ClassifyError(%v): expected type %v, got %v", 
				test.inputError, test.expectedType, result.Type)
		}

		if result.IsRetryable() != test.shouldRetry {
			t.Errorf("ClassifyError(%v): expected retryable %v, got %v", 
				test.inputError, test.shouldRetry, result.IsRetryable())
		}
	}
}

func TestClassifyError_CrawlerError(t *testing.T) {
	// Test that existing CrawlerError is returned as-is
	original := NewNetworkError("source", "message", nil)
	result := ClassifyError("different-source", original)

	if result != original {
		t.Error("Expected same CrawlerError instance to be returned")
	}
}

func TestClassifyError_Nil(t *testing.T) {
	result := ClassifyError("source", nil)
	if result != nil {
		t.Error("Expected nil result for nil error")
	}
}

func TestRetryConfig_ShouldRetry(t *testing.T) {
	config := DefaultRetryConfig()

	tests := []struct {
		err         error
		attempt     int
		shouldRetry bool
	}{
		{fmt.Errorf("network timeout"), 1, true},
		{fmt.Errorf("not found"), 1, false},
		{fmt.Errorf("network timeout"), config.MaxAttempts, false}, // Max attempts reached
		{fmt.Errorf("validation failed"), 1, false},
		{fmt.Errorf("connection refused"), 2, true},
	}

	for _, test := range tests {
		result := config.ShouldRetry(test.err, test.attempt)
		if result != test.shouldRetry {
			t.Errorf("ShouldRetry(%v, %d): expected %v, got %v", 
				test.err, test.attempt, test.shouldRetry, result)
		}
	}
}

func TestRetryConfig_CalculateDelay(t *testing.T) {
	config := &RetryConfig{
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		JitterPercent: 0.0, // No jitter for predictable testing
	}

	// Test initial delay
	delay0 := config.CalculateDelay(0)
	if delay0 != 1*time.Second {
		t.Errorf("Expected initial delay 1s, got %v", delay0)
	}

	// Test exponential backoff
	delay1 := config.CalculateDelay(1)
	expected1 := 2 * time.Second
	if delay1 != expected1 {
		t.Errorf("Expected delay %v, got %v", expected1, delay1)
	}

	delay2 := config.CalculateDelay(2)
	expected2 := 4 * time.Second
	if delay2 != expected2 {
		t.Errorf("Expected delay %v, got %v", expected2, delay2)
	}

	// Test max delay cap
	delay5 := config.CalculateDelay(5) // Would be 32s without cap
	if delay5 != 10*time.Second {
		t.Errorf("Expected max delay 10s, got %v", delay5)
	}
}

func TestRetryConfig_CalculateDelay_WithJitter(t *testing.T) {
	config := &RetryConfig{
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		JitterPercent: 0.1,
	}

	// Test that jitter produces different results
	delay1a := config.CalculateDelay(1)
	delay1b := config.CalculateDelay(1)

	// Due to jitter, delays should be close but potentially different
	baseDuration := 2 * time.Second
	tolerance := time.Duration(float64(baseDuration) * 0.2) // Allow 20% variance

	if delay1a < baseDuration-tolerance || delay1a > baseDuration+tolerance {
		t.Errorf("Delay with jitter outside expected range: %v", delay1a)
	}

	if delay1b < baseDuration-tolerance || delay1b > baseDuration+tolerance {
		t.Errorf("Delay with jitter outside expected range: %v", delay1b)
	}
}

func TestErrorStats(t *testing.T) {
	stats := NewErrorStats()

	// Test initial state
	if stats.TotalErrors != 0 {
		t.Errorf("Expected 0 total errors, got %d", stats.TotalErrors)
	}

	if stats.GetRetryableRate() != 0.0 {
		t.Errorf("Expected 0.0 retryable rate, got %f", stats.GetRetryableRate())
	}

	// Record some errors
	err1 := NewNetworkError("source1", "network error", nil)
	err2 := NewNotFoundError("source2", "TEST-123")
	err3 := NewHTTPError("source1", 500, nil)

	stats.RecordError(err1)
	stats.RecordError(err2)
	stats.RecordError(err3)

	// Test statistics
	if stats.TotalErrors != 3 {
		t.Errorf("Expected 3 total errors, got %d", stats.TotalErrors)
	}

	if stats.ErrorsByType[ErrorTypeNetwork] != 1 {
		t.Errorf("Expected 1 network error, got %d", stats.ErrorsByType[ErrorTypeNetwork])
	}

	if stats.ErrorsBySource["source1"] != 2 {
		t.Errorf("Expected 2 errors from source1, got %d", stats.ErrorsBySource["source1"])
	}

	// Test error rates
	networkRate := stats.GetErrorRate(ErrorTypeNetwork)
	expectedNetworkRate := 1.0 / 3.0
	if networkRate != expectedNetworkRate {
		t.Errorf("Expected network error rate %f, got %f", expectedNetworkRate, networkRate)
	}

	retryableRate := stats.GetRetryableRate()
	expectedRetryableRate := 2.0 / 3.0 // Network and HTTP errors are retryable
	if retryableRate != expectedRetryableRate {
		t.Errorf("Expected retryable rate %f, got %f", expectedRetryableRate, retryableRate)
	}

	// Test reset
	stats.Reset()
	if stats.TotalErrors != 0 {
		t.Errorf("Expected 0 total errors after reset, got %d", stats.TotalErrors)
	}
}

func TestIsRetryableFromCause(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{fmt.Errorf("timeout occurred"), true},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("service unavailable"), true},
		{fmt.Errorf("not found"), false},
		{fmt.Errorf("unauthorized"), false},
		{fmt.Errorf("validation failed"), false},
		{fmt.Errorf("some random error"), false},
	}

	for _, test := range tests {
		result := isRetryableFromCause(test.err)
		if result != test.expected {
			t.Errorf("isRetryableFromCause(%v): expected %v, got %v", 
				test.err, test.expected, result)
		}
	}
}

func BenchmarkClassifyError(b *testing.B) {
	testError := fmt.Errorf("network timeout occurred")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyError("test-source", testError)
	}
}

func BenchmarkCalculateDelay(b *testing.B) {
	config := DefaultRetryConfig()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.CalculateDelay(i % 10)
	}
}