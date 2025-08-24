package image

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestNewImageProcessor(t *testing.T) {
	config := DefaultProcessorConfig()
	processor := NewImageProcessor(config)

	if processor.config != config {
		t.Error("Config not properly set")
	}

	if len(processor.tasks) != 0 {
		t.Error("Tasks map should be empty initially")
	}

	if processor.stats.TotalTasks != 0 {
		t.Error("Stats should be zero initially")
	}
}

func TestDefaultProcessorConfig(t *testing.T) {
	config := DefaultProcessorConfig()

	if config.OutputDir != "./processed" {
		t.Errorf("Expected default output dir './processed', got '%s'", config.OutputDir)
	}

	if config.Quality != 85 {
		t.Errorf("Expected default quality 85, got %d", config.Quality)
	}

	if config.MaxWidth != 1920 {
		t.Errorf("Expected default max width 1920, got %d", config.MaxWidth)
	}

	if config.MaxHeight != 1080 {
		t.Errorf("Expected default max height 1080, got %d", config.MaxHeight)
	}

	if config.Format != FormatJPEG {
		t.Errorf("Expected default format JPEG, got %s", config.Format)
	}

	if !config.PreserveAspect {
		t.Error("PreserveAspect should be true by default")
	}

	if config.MaxConcurrency != 3 {
		t.Errorf("Expected MaxConcurrency 3, got %d", config.MaxConcurrency)
	}
}

func TestImageFormat_String(t *testing.T) {
	tests := []struct {
		format   ImageFormat
		expected string
	}{
		{FormatJPEG, "jpeg"},
		{FormatPNG, "png"},
		{FormatWebP, "webp"},
		{FormatAuto, "auto"},
		{ImageFormat(999), "unknown"},
	}

	for _, test := range tests {
		result := test.format.String()
		if result != test.expected {
			t.Errorf("ImageFormat(%d).String(): expected '%s', got '%s'", int(test.format), test.expected, result)
		}
	}
}

func TestImageFormat_Extension(t *testing.T) {
	tests := []struct {
		format   ImageFormat
		expected string
	}{
		{FormatJPEG, ".jpg"},
		{FormatPNG, ".png"},
		{FormatWebP, ".webp"},
		{FormatAuto, ".jpg"},
		{ImageFormat(999), ".jpg"},
	}

	for _, test := range tests {
		result := test.format.Extension()
		if result != test.expected {
			t.Errorf("ImageFormat(%d).Extension(): expected '%s', got '%s'", int(test.format), test.expected, result)
		}
	}
}

func TestOperationType_String(t *testing.T) {
	tests := []struct {
		op       OperationType
		expected string
	}{
		{OpResize, "RESIZE"},
		{OpCompress, "COMPRESS"},
		{OpConvert, "CONVERT"},
		{OpCrop, "CROP"},
		{OpWatermark, "WATERMARK"},
		{OperationType(999), "UNKNOWN"},
	}

	for _, test := range tests {
		result := test.op.String()
		if result != test.expected {
			t.Errorf("OperationType(%d).String(): expected '%s', got '%s'", int(test.op), test.expected, result)
		}
	}
}

func TestTaskStatus_String(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskPending, "PENDING"},
		{TaskRunning, "RUNNING"},
		{TaskCompleted, "COMPLETED"},
		{TaskFailed, "FAILED"},
		{TaskStatus(999), "UNKNOWN"},
	}

	for _, test := range tests {
		result := test.status.String()
		if result != test.expected {
			t.Errorf("TaskStatus(%d).String(): expected '%s', got '%s'", int(test.status), test.expected, result)
		}
	}
}

func TestIsValidImageFile(t *testing.T) {
	processor := NewImageProcessor(DefaultProcessorConfig())

	tests := []struct {
		path     string
		expected bool
	}{
		{"test.jpg", true},
		{"test.jpeg", true},
		{"test.png", true},
		{"test.bmp", true},
		{"test.gif", true},
		{"test.webp", true},
		{"test.txt", false},
		{"test.mp4", false},
		{"test", false},
		{"", false},
	}

	for _, test := range tests {
		result := processor.isValidImageFile(test.path)
		if result != test.expected {
			t.Errorf("isValidImageFile(%q): expected %v, got %v", test.path, test.expected, result)
		}
	}
}

func TestGenerateOutputPath(t *testing.T) {
	config := DefaultProcessorConfig()
	config.OutputDir = "/test/output"
	config.Format = FormatPNG
	
	processor := NewImageProcessor(config)

	inputPath := "/input/test.jpg"
	outputPath := processor.generateOutputPath(inputPath)

	expectedPath := filepath.Join("/test/output", "test_processed.png")
	if outputPath != expectedPath {
		t.Errorf("Expected output path '%s', got '%s'", expectedPath, outputPath)
	}
}

func TestDetermineOutputFormat(t *testing.T) {
	tests := []struct {
		configFormat ImageFormat
		inputFormat  string
		expected     ImageFormat
	}{
		{FormatJPEG, "png", FormatJPEG},      // Config overrides
		{FormatPNG, "jpeg", FormatPNG},       // Config overrides
		{FormatAuto, "png", FormatPNG},       // Auto-detect
		{FormatAuto, "webp", FormatWebP},     // Auto-detect
		{FormatAuto, "jpeg", FormatJPEG},     // Auto-detect
		{FormatAuto, "unknown", FormatJPEG},  // Default to JPEG
	}

	for _, test := range tests {
		config := DefaultProcessorConfig()
		config.Format = test.configFormat
		processor := NewImageProcessor(config)

		result := processor.determineOutputFormat(test.inputFormat)
		if result != test.expected {
			t.Errorf("determineOutputFormat(config=%s, input=%s): expected %s, got %s",
				test.configFormat, test.inputFormat, test.expected, result)
		}
	}
}

func TestProcessImage_InvalidFile(t *testing.T) {
	processor := NewImageProcessor(DefaultProcessorConfig())

	// Test with non-existent file
	_, err := processor.ProcessImage(context.Background(), "/nonexistent/file.jpg", nil)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test with invalid file extension
	_, err = processor.ProcessImage(context.Background(), "/test/file.txt", nil)
	if err == nil {
		t.Error("Expected error for invalid file extension")
	}
}

func TestProcessImage_ValidImage(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create a simple test image
	inputPath := filepath.Join(tempDir, "test.png")
	testImg := createTestImage(100, 100)
	
	inputFile, err := os.Create(inputPath)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	defer inputFile.Close()

	if err := png.Encode(inputFile, testImg); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
	inputFile.Close()

	// Configure processor
	config := DefaultProcessorConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	processor := NewImageProcessor(config)

	// Create processing operations
	operations := []ProcessingOperation{
		CreateResizeOperation(200, 200),
		CreateCompressOperation(90),
	}

	// Process image
	task, err := processor.ProcessImage(context.Background(), inputPath, operations)
	if err != nil {
		t.Fatalf("ProcessImage failed: %v", err)
	}

	// Check task status
	if task.Status != TaskCompleted {
		t.Errorf("Expected task status %s, got %s", TaskCompleted, task.Status)
	}

	// Check that output file exists
	if !fileExists(task.OutputPath) {
		t.Error("Output file should exist after processing")
	}

	// Check task has input/output sizes
	if task.InputSize == 0 {
		t.Error("Task should have input size recorded")
	}

	if task.OutputSize == 0 {
		t.Error("Task should have output size recorded")
	}
}

func TestProcessBatch(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create multiple test images
	var inputPaths []string
	for i := 1; i <= 3; i++ {
		inputPath := filepath.Join(tempDir, fmt.Sprintf("test%d.png", i))
		testImg := createTestImage(50+i*10, 50+i*10)
		
		inputFile, err := os.Create(inputPath)
		if err != nil {
			t.Fatalf("Failed to create test image file %d: %v", i, err)
		}
		
		if err := png.Encode(inputFile, testImg); err != nil {
			inputFile.Close()
			t.Fatalf("Failed to encode test image %d: %v", i, err)
		}
		inputFile.Close()
		
		inputPaths = append(inputPaths, inputPath)
	}

	// Configure processor
	config := DefaultProcessorConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	config.MaxConcurrency = 2
	processor := NewImageProcessor(config)

	// Create processing operations
	operations := []ProcessingOperation{
		CreateResizeOperation(100, 100),
	}

	// Process batch
	tasks, err := processor.ProcessBatch(context.Background(), inputPaths, operations)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Check results
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	completedCount := 0
	for _, task := range tasks {
		if task.Status == TaskCompleted {
			completedCount++
		}
	}

	if completedCount != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", completedCount)
	}
}

func TestCreateOperations(t *testing.T) {
	// Test CreateResizeOperation
	resizeOp := CreateResizeOperation(800, 600)
	if resizeOp.Type != OpResize {
		t.Error("CreateResizeOperation should create resize operation")
	}
	
	width, ok := resizeOp.Parameters["width"].(int)
	if !ok || width != 800 {
		t.Error("Resize operation should have correct width parameter")
	}

	height, ok := resizeOp.Parameters["height"].(int)
	if !ok || height != 600 {
		t.Error("Resize operation should have correct height parameter")
	}

	// Test CreateCompressOperation
	compressOp := CreateCompressOperation(85)
	if compressOp.Type != OpCompress {
		t.Error("CreateCompressOperation should create compress operation")
	}

	quality, ok := compressOp.Parameters["quality"].(int)
	if !ok || quality != 85 {
		t.Error("Compress operation should have correct quality parameter")
	}

	// Test CreateConvertOperation
	convertOp := CreateConvertOperation(FormatPNG)
	if convertOp.Type != OpConvert {
		t.Error("CreateConvertOperation should create convert operation")
	}

	format, ok := convertOp.Parameters["format"].(ImageFormat)
	if !ok || format != FormatPNG {
		t.Error("Convert operation should have correct format parameter")
	}

	// Test CreateCropOperation
	cropOp := CreateCropOperation(10, 20, 300, 400)
	if cropOp.Type != OpCrop {
		t.Error("CreateCropOperation should create crop operation")
	}

	x, ok := cropOp.Parameters["x"].(int)
	if !ok || x != 10 {
		t.Error("Crop operation should have correct x parameter")
	}
}

func TestGetStats(t *testing.T) {
	processor := NewImageProcessor(DefaultProcessorConfig())

	// Initial stats should be zero
	stats := processor.GetStats()
	if stats.TotalTasks != 0 {
		t.Error("Initial total tasks should be 0")
	}
	if stats.CompletedTasks != 0 {
		t.Error("Initial completed tasks should be 0")
	}
	if stats.FailedTasks != 0 {
		t.Error("Initial failed tasks should be 0")
	}
}

func TestGetTask(t *testing.T) {
	processor := NewImageProcessor(DefaultProcessorConfig())

	// Test non-existent task
	_, exists := processor.GetTask("nonexistent")
	if exists {
		t.Error("Non-existent task should not be found")
	}

	// Add a task manually for testing
	testTask := &ProcessingTask{
		ID:     "test-task-123",
		Status: TaskPending,
	}

	processor.mutex.Lock()
	processor.tasks[testTask.ID] = testTask
	processor.mutex.Unlock()

	// Test existing task
	retrievedTask, exists := processor.GetTask("test-task-123")
	if !exists {
		t.Error("Task should be found")
	}

	if retrievedTask.ID != testTask.ID {
		t.Error("Retrieved task should have correct ID")
	}
}

func TestGetTasks(t *testing.T) {
	processor := NewImageProcessor(DefaultProcessorConfig())

	// Initially should be empty
	tasks := processor.GetTasks()
	if len(tasks) != 0 {
		t.Error("Initially should have no tasks")
	}

	// Add some tasks
	for i := 1; i <= 3; i++ {
		task := &ProcessingTask{
			ID:     fmt.Sprintf("task-%d", i),
			Status: TaskPending,
		}
		processor.mutex.Lock()
		processor.tasks[task.ID] = task
		processor.mutex.Unlock()
	}

	// Check tasks are returned
	tasks = processor.GetTasks()
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
}

func TestResetStats(t *testing.T) {
	processor := NewImageProcessor(DefaultProcessorConfig())

	// Set some stats
	processor.stats.TotalTasks = 10
	processor.stats.CompletedTasks = 8
	processor.stats.FailedTasks = 2

	// Reset stats
	processor.ResetStats()

	// Check stats are reset
	stats := processor.GetStats()
	if stats.TotalTasks != 0 {
		t.Error("Total tasks should be reset to 0")
	}
	if stats.CompletedTasks != 0 {
		t.Error("Completed tasks should be reset to 0")
	}
	if stats.FailedTasks != 0 {
		t.Error("Failed tasks should be reset to 0")
	}
}

// Helper function to create a simple test image
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Fill with a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a simple gradient pattern
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	
	return img
}

func BenchmarkProcessImage(b *testing.B) {
	// Create temp directory
	tempDir := b.TempDir()

	// Create a test image
	inputPath := filepath.Join(tempDir, "benchmark.png")
	testImg := createTestImage(200, 200)
	
	inputFile, err := os.Create(inputPath)
	if err != nil {
		b.Fatalf("Failed to create test image file: %v", err)
	}
	defer inputFile.Close()

	if err := png.Encode(inputFile, testImg); err != nil {
		b.Fatalf("Failed to encode test image: %v", err)
	}
	inputFile.Close()

	// Configure processor
	config := DefaultProcessorConfig()
	config.OutputDir = filepath.Join(tempDir, "output")
	processor := NewImageProcessor(config)

	// Create processing operations
	operations := []ProcessingOperation{
		CreateResizeOperation(150, 150),
		CreateCompressOperation(85),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessImage(context.Background(), inputPath, operations)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}