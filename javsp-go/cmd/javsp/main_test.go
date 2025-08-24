//go:build unit

package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainVersionVariables(t *testing.T) {
	// Test that version variables are set to default values
	if version == "" {
		t.Error("version should not be empty")
	}
	
	if commit == "" {
		t.Error("commit should not be empty")
	}
	
	if date == "" {
		t.Error("date should not be empty")
	}
	
	// Test default values
	if version != "v1.0.0" {
		t.Errorf("Expected default version 'v1.0.0', got '%s'", version)
	}
	
	if commit != "unknown" {
		t.Errorf("Expected default commit 'unknown', got '%s'", commit)
	}
	
	if date != "unknown" {
		t.Errorf("Expected default date 'unknown', got '%s'", date)
	}
}

func TestMainFunction(t *testing.T) {
	// Since main() calls config.Execute() which is a CLI command,
	// we can't easily test it directly without affecting the test environment.
	// Instead, we test that the main function compiles correctly.
	// The main function is tested indirectly through successful compilation.
	
	// This test verifies the main package compiles correctly
	t.Log("Main package compiles correctly")
}

// TestBuildInfo tests that build information can be set at compile time
func TestBuildInfo(t *testing.T) {
	tests := []struct {
		name     string
		variable *string
		expected string
	}{
		{"version", &version, "v1.0.0"},
		{"commit", &commit, "unknown"},
		{"date", &date, "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if *tt.variable != tt.expected {
				t.Errorf("Expected %s to be '%s', got '%s'", tt.name, tt.expected, *tt.variable)
			}
		})
	}
}

// TestMainPackageImports tests that necessary packages are imported
func TestMainPackageImports(t *testing.T) {
	// This is more of a compile-time test, but we can verify
	// that the config package is accessible
	
	// Read the main.go file to check imports
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Skipf("Cannot read main.go: %v", err)
	}
	
	mainGoContent := string(content)
	
	// Check that config package is imported
	if !strings.Contains(mainGoContent, `"javsp-go/internal/config"`) {
		t.Error("main.go should import javsp-go/internal/config")
	}
	
	// Check that config.Execute() is called
	if !strings.Contains(mainGoContent, "config.Execute()") {
		t.Error("main.go should call config.Execute()")
	}
}

// TestMainGlobalVariables tests the global variables structure
func TestMainGlobalVariables(t *testing.T) {
	// Test that global variables are properly declared and accessible
	
	// These should be string pointers that can be set by linker flags
	versionPtr := &version
	commitPtr := &commit
	datePtr := &date
	
	if versionPtr == nil {
		t.Error("version variable should be addressable")
	}
	
	if commitPtr == nil {
		t.Error("commit variable should be addressable")
	}
	
	if datePtr == nil {
		t.Error("date variable should be addressable")
	}
	
	// Test that they contain some value
	if *versionPtr == "" {
		t.Error("version should have a default value")
	}
	
	if *commitPtr == "" {
		t.Error("commit should have a default value")
	}
	
	if *datePtr == "" {
		t.Error("date should have a default value")
	}
}