package organizer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"javsp-go/internal/datatype"
)

func TestNewFileOrganizer(t *testing.T) {
	config := DefaultOrganizerConfig()
	organizer := NewFileOrganizer(config)

	if organizer.config != config {
		t.Error("Config not properly set")
	}

	if len(organizer.operations) != 0 {
		t.Error("Operations map should be empty initially")
	}

	if organizer.stats.TotalOperations != 0 {
		t.Error("Stats should be zero initially")
	}
}

func TestDefaultOrganizerConfig(t *testing.T) {
	config := DefaultOrganizerConfig()

	if config.OutputDir != "./organized" {
		t.Errorf("Expected default output dir './organized', got '%s'", config.OutputDir)
	}

	if config.Pattern != "{DVDID} - {Title}" {
		t.Errorf("Expected default pattern '{DVDID} - {Title}', got '%s'", config.Pattern)
	}

	if !config.CreateDirectories {
		t.Error("CreateDirectories should be true by default")
	}

	if config.OverwriteExisting {
		t.Error("OverwriteExisting should be false by default")
	}

	if !config.BackupOriginal {
		t.Error("BackupOriginal should be true by default")
	}

	if config.MaxConcurrency != 3 {
		t.Errorf("Expected MaxConcurrency 3, got %d", config.MaxConcurrency)
	}
}

func TestGenerateDestinationPath(t *testing.T) {
	config := DefaultOrganizerConfig()
	config.OutputDir = "/test/output"
	config.Pattern = "{DVDID} - {Title}"
	
	organizer := NewFileOrganizer(config)

	// Create test movie
	movie := &datatype.Movie{
		FilePath: "/test/source/movie.mp4",
		FileName: "movie.mp4",
		DVDID:    "TEST-123",
		Info: &datatype.MovieInfo{
			DVDID: "TEST-123",
			Title: "Test Movie",
			Year:  "2023",
		},
	}

	destPath, err := organizer.generateDestinationPath(movie)
	if err != nil {
		t.Fatalf("Failed to generate destination path: %v", err)
	}

	expected := filepath.Join("/test/output", "TEST-123 - Test Movie.mp4")
	if destPath != expected {
		t.Errorf("Expected destination path '%s', got '%s'", expected, destPath)
	}
}

func TestGenerateDestinationPath_SpecialCharacters(t *testing.T) {
	config := DefaultOrganizerConfig()
	config.OutputDir = "/test/output"
	config.Pattern = "{DVDID} - {Title}"
	
	organizer := NewFileOrganizer(config)

	// Create test movie with special characters
	movie := &datatype.Movie{
		FilePath: "/test/source/movie.mp4",
		FileName: "movie.mp4",
		DVDID:    "TEST-456",
		Info: &datatype.MovieInfo{
			DVDID: "TEST-456",
			Title: "Test/Movie: <Special>",
		},
	}

	destPath, err := organizer.generateDestinationPath(movie)
	if err != nil {
		t.Fatalf("Failed to generate destination path: %v", err)
	}

	// Special characters should be cleaned
	if strings.Contains(destPath, "/") && !strings.HasPrefix(destPath, "/test/output") {
		t.Error("Path should not contain invalid characters in filename")
	}

	if strings.Contains(filepath.Base(destPath), ":") {
		t.Error("Filename should not contain colon characters")
	}
}

func TestOrganizeMovie_DryRun(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.mp4")
	
	// Create source file
	if err := os.WriteFile(sourceFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure organizer for dry run
	config := DefaultOrganizerConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.DryRun = true
	
	organizer := NewFileOrganizer(config)

	// Create test movie
	movie := &datatype.Movie{
		FilePath: sourceFile,
		FileName: "source.mp4",
		DVDID:    "DRY-001",
		Info: &datatype.MovieInfo{
			DVDID: "DRY-001",
			Title: "Dry Run Test",
		},
	}

	// Execute organize operation
	op, err := organizer.OrganizeMovie(context.Background(), movie)
	if err != nil {
		t.Fatalf("OrganizeMovie failed: %v", err)
	}

	// Check operation status
	if op.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, op.Status)
	}

	// Source file should still exist (dry run)
	if !fileExists(sourceFile) {
		t.Error("Source file should still exist in dry run mode")
	}

	// Destination file should not exist (dry run)
	if fileExists(op.Destination) {
		t.Error("Destination file should not exist in dry run mode")
	}
}

func TestOrganizeMovie_ActualMove(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.mp4")
	
	// Create source file with content
	testContent := []byte("test movie content")
	if err := os.WriteFile(sourceFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure organizer
	config := DefaultOrganizerConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.DryRun = false
	
	organizer := NewFileOrganizer(config)

	// Create test movie
	movie := &datatype.Movie{
		FilePath: sourceFile,
		FileName: "source.mp4",
		DVDID:    "MOVE-001",
		Info: &datatype.MovieInfo{
			DVDID: "MOVE-001",
			Title: "Move Test",
		},
	}

	// Execute organize operation
	op, err := organizer.OrganizeMovie(context.Background(), movie)
	if err != nil {
		t.Fatalf("OrganizeMovie failed: %v", err)
	}

	// Check operation status
	if op.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, op.Status)
	}

	// Source file should not exist (moved)
	if fileExists(sourceFile) {
		t.Error("Source file should not exist after move")
	}

	// Destination file should exist
	if !fileExists(op.Destination) {
		t.Error("Destination file should exist after move")
	}

	// Verify content
	destContent, err := os.ReadFile(op.Destination)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(destContent) != string(testContent) {
		t.Error("Destination file content does not match source")
	}
}

func TestOrganizeMovie_OverwriteProtection(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.mp4")
	
	// Create source file
	if err := os.WriteFile(sourceFile, []byte("source content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure organizer
	config := DefaultOrganizerConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.DryRun = false
	config.OverwriteExisting = false
	
	organizer := NewFileOrganizer(config)

	// Create test movie
	movie := &datatype.Movie{
		FilePath: sourceFile,
		FileName: "source.mp4",
		DVDID:    "OVERWRITE-001",
		Info: &datatype.MovieInfo{
			DVDID: "OVERWRITE-001",
			Title: "Overwrite Test",
		},
	}

	// Pre-create destination file
	destPath, _ := organizer.generateDestinationPath(movie)
	destDir := filepath.Dir(destPath)
	os.MkdirAll(destDir, 0755)
	if err := os.WriteFile(destPath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("Failed to create destination file: %v", err)
	}

	// Execute organize operation (should fail)
	_, err := organizer.OrganizeMovie(context.Background(), movie)
	if err == nil {
		t.Error("Expected operation to fail due to existing destination file")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

func TestOrganizeBatch(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	
	// Create multiple source files
	var movies []*datatype.Movie
	for i := 1; i <= 5; i++ {
		sourceFile := filepath.Join(tempDir, fmt.Sprintf("source%d.mp4", i))
		content := []byte(fmt.Sprintf("test content %d", i))
		if err := os.WriteFile(sourceFile, content, 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}

		movie := &datatype.Movie{
			FilePath: sourceFile,
			FileName: fmt.Sprintf("source%d.mp4", i),
			DVDID:    fmt.Sprintf("BATCH-%03d", i),
			Info: &datatype.MovieInfo{
				DVDID: fmt.Sprintf("BATCH-%03d", i),
				Title: fmt.Sprintf("Batch Test %d", i),
			},
		}
		movies = append(movies, movie)
	}

	// Configure organizer
	config := DefaultOrganizerConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.DryRun = false
	config.MaxConcurrency = 2
	
	organizer := NewFileOrganizer(config)

	// Execute batch organize
	operations, err := organizer.OrganizeBatch(context.Background(), movies)
	if err != nil {
		t.Fatalf("OrganizeBatch failed: %v", err)
	}

	// Check results
	if len(operations) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(operations))
	}

	completedCount := 0
	for _, op := range operations {
		if op.Status == StatusCompleted {
			completedCount++
		}
	}

	if completedCount != 5 {
		t.Errorf("Expected 5 completed operations, got %d", completedCount)
	}

	// Verify all files were moved
	for i := 1; i <= 5; i++ {
		sourceFile := filepath.Join(tempDir, fmt.Sprintf("source%d.mp4", i))
		if fileExists(sourceFile) {
			t.Errorf("Source file %d should not exist after batch move", i)
		}
	}
}

func TestRollbackOperation(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "source.mp4")
	
	// Create source file
	testContent := []byte("test content for rollback")
	if err := os.WriteFile(sourceFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure organizer
	config := DefaultOrganizerConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.DryRun = false
	
	organizer := NewFileOrganizer(config)

	// Create test movie
	movie := &datatype.Movie{
		FilePath: sourceFile,
		FileName: "source.mp4",
		DVDID:    "ROLLBACK-001",
		Info: &datatype.MovieInfo{
			DVDID: "ROLLBACK-001",
			Title: "Rollback Test",
		},
	}

	// Execute organize operation
	op, err := organizer.OrganizeMovie(context.Background(), movie)
	if err != nil {
		t.Fatalf("OrganizeMovie failed: %v", err)
	}

	// Verify move completed
	if !fileExists(op.Destination) {
		t.Fatal("Destination file should exist after move")
	}
	if fileExists(sourceFile) {
		t.Fatal("Source file should not exist after move")
	}

	// Execute rollback
	err = organizer.RollbackOperation(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("RollbackOperation failed: %v", err)
	}

	// Verify rollback
	if !fileExists(sourceFile) {
		t.Error("Source file should exist after rollback")
	}
	if fileExists(op.Destination) {
		t.Error("Destination file should not exist after rollback")
	}

	// Verify content
	rolledBackContent, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read rolled back file: %v", err)
	}

	if string(rolledBackContent) != string(testContent) {
		t.Error("Rolled back file content does not match original")
	}

	// Check operation status
	rolledBackOp, _ := organizer.GetOperation(op.ID)
	if rolledBackOp.Status != StatusRolledBack {
		t.Errorf("Expected status %s, got %s", StatusRolledBack, rolledBackOp.Status)
	}
}

func TestCleanFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Title", "Normal Title"},
		{"Title/With\\Slashes", "Title_With_Slashes"},
		{"Title:With:Colons", "Title-With-Colons"},
		{"Title*With?Invalid\"Chars", "Title_With_Invalid'Chars"},
		{"Title<With>Brackets", "Title(With)Brackets"},
		{"Title|With|Pipes", "Title_With_Pipes"},
		{"  Leading and Trailing  ", "Leading and Trailing"},
	}

	for _, test := range tests {
		result := cleanFilename(test.input)
		if result != test.expected {
			t.Errorf("cleanFilename(%q): expected %q, got %q", test.input, test.expected, result)
		}
	}
}

func TestOperationTypeString(t *testing.T) {
	tests := []struct {
		op       OperationType
		expected string
	}{
		{OpMove, "MOVE"},
		{OpCopy, "COPY"},
		{OpRename, "RENAME"},
		{OpDelete, "DELETE"},
		{OperationType(999), "UNKNOWN"},
	}

	for _, test := range tests {
		result := test.op.String()
		if result != test.expected {
			t.Errorf("OperationType(%d).String(): expected %q, got %q", int(test.op), test.expected, result)
		}
	}
}

func TestOperationStatusString(t *testing.T) {
	tests := []struct {
		status   OperationStatus
		expected string
	}{
		{StatusPending, "PENDING"},
		{StatusRunning, "RUNNING"},
		{StatusCompleted, "COMPLETED"},
		{StatusFailed, "FAILED"},
		{StatusRolledBack, "ROLLED_BACK"},
		{OperationStatus(999), "UNKNOWN"},
	}

	for _, test := range tests {
		result := test.status.String()
		if result != test.expected {
			t.Errorf("OperationStatus(%d).String(): expected %q, got %q", int(test.status), test.expected, result)
		}
	}
}

func TestGetStats(t *testing.T) {
	organizer := NewFileOrganizer(DefaultOrganizerConfig())
	
	// Initial stats should be zero
	stats := organizer.GetStats()
	if stats.TotalOperations != 0 {
		t.Error("Initial total operations should be 0")
	}
	if stats.CompletedOperations != 0 {
		t.Error("Initial completed operations should be 0")
	}
	if stats.FailedOperations != 0 {
		t.Error("Initial failed operations should be 0")
	}
}

func TestInvalidMovieData(t *testing.T) {
	organizer := NewFileOrganizer(DefaultOrganizerConfig())

	// Test with nil movie
	_, err := organizer.OrganizeMovie(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil movie")
	}

	// Test with invalid movie (no ID)
	invalidMovie := &datatype.Movie{
		FilePath: "/test/path.mp4",
	}

	_, err = organizer.OrganizeMovie(context.Background(), invalidMovie)
	if err == nil {
		t.Error("Expected error for invalid movie")
	}
}

func BenchmarkOrganizeMovie(b *testing.B) {
	// Create temp directory
	tempDir := b.TempDir()
	
	// Configure organizer for dry run (no actual I/O)
	config := DefaultOrganizerConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.DryRun = true
	
	organizer := NewFileOrganizer(config)

	// Create test movie
	movie := &datatype.Movie{
		FilePath: filepath.Join(tempDir, "benchmark.mp4"),
		FileName: "benchmark.mp4",
		DVDID:    "BENCH-001",
		Info: &datatype.MovieInfo{
			DVDID: "BENCH-001",
			Title: "Benchmark Movie",
			Year:  "2023",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := organizer.OrganizeMovie(context.Background(), movie)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}