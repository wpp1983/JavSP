package testutils

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTimeout is the default timeout for tests
const TestTimeout = 30 * time.Second

// CreateTempDir creates a temporary directory for testing
func CreateTempDir(t *testing.T) string {
	t.Helper()
	
	tmpDir, err := os.MkdirTemp("", "javsp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	
	return tmpDir
}

// CreateTestFiles creates test video files in the given directory
func CreateTestFiles(t *testing.T, dir string, filenames []string) []string {
	t.Helper()
	
	paths := make([]string, len(filenames))
	for i, filename := range filenames {
		path := filepath.Join(dir, filename)
		
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		
		// Create empty file
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
		file.Close()
		
		paths[i] = path
	}
	
	return paths
}

// WriteTestFile writes content to a test file
func WriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file %s: %v", path, err)
	}
}

// ReadTestFile reads content from a test file
func ReadTestFile(t *testing.T, path string) string {
	t.Helper()
	
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test file %s: %v", path, err)
	}
	
	return string(content)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// AssertFileExists asserts that a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	
	if !FileExists(path) {
		t.Errorf("File %s does not exist", path)
	}
}

// AssertFileNotExists asserts that a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	
	if FileExists(path) {
		t.Errorf("File %s should not exist", path)
	}
}

// CreateTestContext creates a test context with timeout
func CreateTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), TestTimeout)
}

// AssertNoError asserts that an error is nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError asserts that an error is not nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// AssertEqual asserts that two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual asserts that two values are not equal
func AssertNotEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	
	if expected == actual {
		t.Errorf("Expected %v to not equal %v", actual, expected)
	}
}

// AssertContains asserts that a string contains a substring
func AssertContains(t *testing.T, str, substr string) {
	t.Helper()
	
	if !contains(str, substr) {
		t.Errorf("Expected %q to contain %q", str, substr)
	}
}

// AssertNotContains asserts that a string does not contain a substring
func AssertNotContains(t *testing.T, str, substr string) {
	t.Helper()
	
	if contains(str, substr) {
		t.Errorf("Expected %q to not contain %q", str, substr)
	}
}

// AssertSliceEqual asserts that two slices are equal
func AssertSliceEqual[T comparable](t *testing.T, expected, actual []T) {
	t.Helper()
	
	if len(expected) != len(actual) {
		t.Errorf("Expected slice length %d, got %d", len(expected), len(actual))
		return
	}
	
	for i, exp := range expected {
		if exp != actual[i] {
			t.Errorf("Expected slice[%d] = %v, got %v", i, exp, actual[i])
		}
	}
}

// AssertSliceContains asserts that a slice contains a value
func AssertSliceContains[T comparable](t *testing.T, slice []T, value T) {
	t.Helper()
	
	for _, item := range slice {
		if item == value {
			return
		}
	}
	
	t.Errorf("Expected slice to contain %v", value)
}

// AssertTrue asserts that a condition is true
func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	
	if !condition {
		t.Error("Expected condition to be true")
	}
}

// AssertFalse asserts that a condition is false
func AssertFalse(t *testing.T, condition bool) {
	t.Helper()
	
	if condition {
		t.Error("Expected condition to be false")
	}
}

// Helper function to check if string contains substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || 
		(len(substr) > 0 && containsRune(str, substr)))
}

func containsRune(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	t.Helper()
	
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// ExpectPanic expects a function to panic
func ExpectPanic(t *testing.T, f func()) {
	t.Helper()
	
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected function to panic")
		}
	}()
	
	f()
}

// Retry retries a function until it succeeds or times out
func Retry(t *testing.T, maxAttempts int, delay time.Duration, fn func() error) {
	t.Helper()
	
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return
		} else {
			lastErr = err
		}
		
		if i < maxAttempts-1 {
			time.Sleep(delay)
		}
	}
	
	t.Fatalf("Function failed after %d attempts. Last error: %v", maxAttempts, lastErr)
}