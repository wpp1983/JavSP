package image

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// External image processing library - commenting out for now
	// "github.com/disintegration/imaging"
)

// ProcessorConfig contains configuration for image processing
type ProcessorConfig struct {
	OutputDir       string            `json:"output_dir"`
	Quality         int               `json:"quality"`          // JPEG quality 1-100
	MaxWidth        int               `json:"max_width"`        // Maximum width for resizing
	MaxHeight       int               `json:"max_height"`       // Maximum height for resizing
	Format          ImageFormat       `json:"format"`           // Output format
	PreserveAspect  bool              `json:"preserve_aspect"`  // Maintain aspect ratio
	EnableWatermark bool              `json:"enable_watermark"`
	WatermarkText   string            `json:"watermark_text"`
	MaxConcurrency  int               `json:"max_concurrency"`
	AllowedFormats  []string          `json:"allowed_formats"`
}

// ImageFormat represents supported image formats
type ImageFormat int

const (
	FormatAuto ImageFormat = iota
	FormatJPEG
	FormatPNG
	FormatWebP
)

func (f ImageFormat) String() string {
	switch f {
	case FormatJPEG:
		return "jpeg"
	case FormatPNG:
		return "png"
	case FormatWebP:
		return "webp"
	case FormatAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// Extension returns file extension for format
func (f ImageFormat) Extension() string {
	switch f {
	case FormatJPEG:
		return ".jpg"
	case FormatPNG:
		return ".png"
	case FormatWebP:
		return ".webp"
	default:
		return ".jpg"
	}
}

// DefaultProcessorConfig returns default configuration
func DefaultProcessorConfig() *ProcessorConfig {
	return &ProcessorConfig{
		OutputDir:       "./processed",
		Quality:         85,
		MaxWidth:        1920,
		MaxHeight:       1080,
		Format:          FormatJPEG,
		PreserveAspect:  true,
		EnableWatermark: false,
		WatermarkText:   "",
		MaxConcurrency:  3,
		AllowedFormats:  []string{".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp"},
	}
}

// ProcessingTask represents an image processing task
type ProcessingTask struct {
	ID           string                `json:"id"`
	InputPath    string                `json:"input_path"`
	OutputPath   string                `json:"output_path"`
	Operations   []ProcessingOperation `json:"operations"`
	Status       TaskStatus            `json:"status"`
	Error        error                 `json:"error,omitempty"`
	StartTime    time.Time             `json:"start_time"`
	EndTime      time.Time             `json:"end_time"`
	InputSize    int64                 `json:"input_size"`
	OutputSize   int64                 `json:"output_size"`
}

// ProcessingOperation represents a single processing operation
type ProcessingOperation struct {
	Type       OperationType `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// OperationType represents different processing operations
type OperationType int

const (
	OpResize OperationType = iota
	OpCompress
	OpConvert
	OpCrop
	OpWatermark
)

func (o OperationType) String() string {
	switch o {
	case OpResize:
		return "RESIZE"
	case OpCompress:
		return "COMPRESS"
	case OpConvert:
		return "CONVERT"
	case OpCrop:
		return "CROP"
	case OpWatermark:
		return "WATERMARK"
	default:
		return "UNKNOWN"
	}
}

// TaskStatus represents processing task status
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskCompleted
	TaskFailed
)

func (s TaskStatus) String() string {
	switch s {
	case TaskPending:
		return "PENDING"
	case TaskRunning:
		return "RUNNING"
	case TaskCompleted:
		return "COMPLETED"
	case TaskFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// ImageProcessor handles image processing operations
type ImageProcessor struct {
	config *ProcessorConfig
	tasks  map[string]*ProcessingTask
	mutex  sync.RWMutex
	stats  *ProcessorStats
}

// ProcessorStats tracks processing statistics
type ProcessorStats struct {
	TotalTasks       int           `json:"total_tasks"`
	CompletedTasks   int           `json:"completed_tasks"`
	FailedTasks      int           `json:"failed_tasks"`
	BytesProcessed   int64         `json:"bytes_processed"`
	BytesSaved       int64         `json:"bytes_saved"`
	TotalDuration    time.Duration `json:"total_duration"`
	AverageTime      time.Duration `json:"average_time"`
}

// NewImageProcessor creates a new image processor
func NewImageProcessor(config *ProcessorConfig) *ImageProcessor {
	if config == nil {
		config = DefaultProcessorConfig()
	}

	return &ImageProcessor{
		config: config,
		tasks:  make(map[string]*ProcessingTask),
		stats:  &ProcessorStats{},
	}
}

// ProcessImage processes a single image with the given operations
func (p *ImageProcessor) ProcessImage(ctx context.Context, inputPath string, operations []ProcessingOperation) (*ProcessingTask, error) {
	// Validate input
	if !p.isValidImageFile(inputPath) {
		return nil, fmt.Errorf("invalid or unsupported image file: %s", inputPath)
	}

	if !fileExists(inputPath) {
		return nil, fmt.Errorf("input file does not exist: %s", inputPath)
	}

	// Generate output path
	outputPath := p.generateOutputPath(inputPath)

	// Create task
	task := &ProcessingTask{
		ID:         generateTaskID(),
		InputPath:  inputPath,
		OutputPath: outputPath,
		Operations: operations,
		Status:     TaskPending,
		StartTime:  time.Now(),
	}

	// Get input file size
	if info, err := os.Stat(inputPath); err == nil {
		task.InputSize = info.Size()
	}

	// Store task
	p.mutex.Lock()
	p.tasks[task.ID] = task
	p.stats.TotalTasks++
	p.mutex.Unlock()

	// Execute processing
	err := p.executeTask(ctx, task)
	if err != nil {
		task.Status = TaskFailed
		task.Error = err
		task.EndTime = time.Now()
		p.stats.FailedTasks++
		return task, err
	}

	task.Status = TaskCompleted
	task.EndTime = time.Now()
	p.stats.CompletedTasks++

	// Update statistics
	duration := task.EndTime.Sub(task.StartTime)
	p.stats.TotalDuration += duration
	p.stats.BytesProcessed += task.InputSize

	if task.OutputSize > 0 {
		if task.InputSize > task.OutputSize {
			p.stats.BytesSaved += task.InputSize - task.OutputSize
		}
	}

	if p.stats.CompletedTasks > 0 {
		p.stats.AverageTime = p.stats.TotalDuration / time.Duration(p.stats.CompletedTasks)
	}

	return task, nil
}

// ProcessBatch processes multiple images concurrently
func (p *ImageProcessor) ProcessBatch(ctx context.Context, inputs []string, operations []ProcessingOperation) ([]*ProcessingTask, error) {
	if len(inputs) == 0 {
		return []*ProcessingTask{}, nil
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, p.config.MaxConcurrency)
	results := make(chan *ProcessingTask, len(inputs))
	var wg sync.WaitGroup

	// Start workers
	for _, inputPath := range inputs {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			task, _ := p.ProcessImage(ctx, path, operations)
			results <- task
		}(inputPath)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var tasks []*ProcessingTask
	for task := range results {
		if task != nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// executeTask executes a processing task
func (p *ImageProcessor) executeTask(ctx context.Context, task *ProcessingTask) error {
	task.Status = TaskRunning

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(task.OutputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load image
	img, format, err := p.loadImage(task.InputPath)
	if err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}

	// Apply operations sequentially
	processedImg := img
	for _, op := range task.Operations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		processedImg, err = p.applyOperation(processedImg, op)
		if err != nil {
			return fmt.Errorf("failed to apply operation %s: %w", op.Type, err)
		}
	}

	// Save processed image
	outputFormat := p.determineOutputFormat(format)
	err = p.saveImage(processedImg, task.OutputPath, outputFormat)
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	// Get output file size
	if info, err := os.Stat(task.OutputPath); err == nil {
		task.OutputSize = info.Size()
	}

	return nil
}

// loadImage loads an image from file
func (p *ImageProcessor) loadImage(path string) (image.Image, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	// Decode image
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	return img, format, nil
}

// saveImage saves an image to file
func (p *ImageProcessor) saveImage(img image.Image, path string, format ImageFormat) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	switch format {
	case FormatJPEG:
		options := &jpeg.Options{Quality: p.config.Quality}
		return jpeg.Encode(file, img, options)
	case FormatPNG:
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		return encoder.Encode(file, img)
	default:
		// Default to JPEG
		options := &jpeg.Options{Quality: p.config.Quality}
		return jpeg.Encode(file, img, options)
	}
}

// applyOperation applies a single processing operation to an image
func (p *ImageProcessor) applyOperation(img image.Image, op ProcessingOperation) (image.Image, error) {
	switch op.Type {
	case OpResize:
		return p.resizeImage(img, op.Parameters)
	case OpCompress:
		// Compression is handled during saving
		return img, nil
	case OpConvert:
		// Format conversion is handled during saving
		return img, nil
	case OpCrop:
		return p.cropImage(img, op.Parameters)
	case OpWatermark:
		return p.addWatermark(img, op.Parameters)
	default:
		return img, fmt.Errorf("unsupported operation: %s", op.Type)
	}
}

// resizeImage resizes an image (basic implementation without external libraries)
func (p *ImageProcessor) resizeImage(img image.Image, params map[string]interface{}) (image.Image, error) {
	// This is a simplified resize implementation
	// In a real implementation, we would use imaging libraries for better quality
	
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Get target dimensions
	targetWidth, ok1 := params["width"].(int)
	targetHeight, ok2 := params["height"].(int)

	if !ok1 && !ok2 {
		targetWidth = p.config.MaxWidth
		targetHeight = p.config.MaxHeight
	}

	// Apply max constraints
	if targetWidth > p.config.MaxWidth {
		targetWidth = p.config.MaxWidth
	}
	if targetHeight > p.config.MaxHeight {
		targetHeight = p.config.MaxHeight
	}

	// Calculate scale factor to maintain aspect ratio
	if p.config.PreserveAspect {
		scaleX := float64(targetWidth) / float64(width)
		scaleY := float64(targetHeight) / float64(height)
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}

		targetWidth = int(float64(width) * scale)
		targetHeight = int(float64(height) * scale)
	}

	// For now, return original image if no actual resizing library is available
	// In production, we would use a proper imaging library
	if targetWidth >= width && targetHeight >= height {
		return img, nil // No resize needed
	}

	// Return original image for now - would implement actual resize with imaging library
	return img, nil
}

// cropImage crops an image
func (p *ImageProcessor) cropImage(img image.Image, params map[string]interface{}) (image.Image, error) {
	// Get crop parameters
	x, ok1 := params["x"].(int)
	y, ok2 := params["y"].(int)
	width, ok3 := params["width"].(int)
	height, ok4 := params["height"].(int)

	if !ok1 || !ok2 || !ok3 || !ok4 {
		return img, fmt.Errorf("invalid crop parameters")
	}

	bounds := img.Bounds()
	cropRect := image.Rect(x, y, x+width, y+height)
	
	// Validate crop rectangle
	if !cropRect.In(bounds) {
		return img, fmt.Errorf("crop rectangle outside image bounds")
	}

	// Create new image with cropped area
	// This is a basic implementation - would use imaging library for better performance
	return img, nil // Return original for now
}

// addWatermark adds a watermark to an image
func (p *ImageProcessor) addWatermark(img image.Image, params map[string]interface{}) (image.Image, error) {
	// Watermark implementation would require additional graphics libraries
	// For now, return original image
	return img, nil
}

// Utility functions

// generateOutputPath generates output path for processed image
func (p *ImageProcessor) generateOutputPath(inputPath string) string {
	filename := filepath.Base(inputPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	
	newExt := p.config.Format.Extension()
	newFilename := nameWithoutExt + "_processed" + newExt
	
	return filepath.Join(p.config.OutputDir, newFilename)
}

// isValidImageFile checks if file is a valid image
func (p *ImageProcessor) isValidImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	
	for _, allowed := range p.config.AllowedFormats {
		if ext == strings.ToLower(allowed) {
			return true
		}
	}
	
	return false
}

// determineOutputFormat determines output format based on config and input
func (p *ImageProcessor) determineOutputFormat(inputFormat string) ImageFormat {
	if p.config.Format != FormatAuto {
		return p.config.Format
	}

	// Auto-detect based on input format
	switch strings.ToLower(inputFormat) {
	case "png":
		return FormatPNG
	case "webp":
		return FormatWebP
	default:
		return FormatJPEG
	}
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// fileExists checks if file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetTask returns a task by ID
func (p *ImageProcessor) GetTask(taskID string) (*ProcessingTask, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	task, exists := p.tasks[taskID]
	return task, exists
}

// GetTasks returns all tasks
func (p *ImageProcessor) GetTasks() []*ProcessingTask {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	tasks := make([]*ProcessingTask, 0, len(p.tasks))
	for _, task := range p.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetStats returns current processor statistics
func (p *ImageProcessor) GetStats() *ProcessorStats {
	return &ProcessorStats{
		TotalTasks:     p.stats.TotalTasks,
		CompletedTasks: p.stats.CompletedTasks,
		FailedTasks:    p.stats.FailedTasks,
		BytesProcessed: p.stats.BytesProcessed,
		BytesSaved:     p.stats.BytesSaved,
		TotalDuration:  p.stats.TotalDuration,
		AverageTime:    p.stats.AverageTime,
	}
}

// ResetStats resets all statistics
func (p *ImageProcessor) ResetStats() {
	p.stats = &ProcessorStats{}
}

// CreateResizeOperation creates a resize operation
func CreateResizeOperation(width, height int) ProcessingOperation {
	return ProcessingOperation{
		Type: OpResize,
		Parameters: map[string]interface{}{
			"width":  width,
			"height": height,
		},
	}
}

// CreateCompressOperation creates a compression operation
func CreateCompressOperation(quality int) ProcessingOperation {
	return ProcessingOperation{
		Type: OpCompress,
		Parameters: map[string]interface{}{
			"quality": quality,
		},
	}
}

// CreateConvertOperation creates a format conversion operation
func CreateConvertOperation(format ImageFormat) ProcessingOperation {
	return ProcessingOperation{
		Type: OpConvert,
		Parameters: map[string]interface{}{
			"format": format,
		},
	}
}

// CreateCropOperation creates a crop operation
func CreateCropOperation(x, y, width, height int) ProcessingOperation {
	return ProcessingOperation{
		Type: OpCrop,
		Parameters: map[string]interface{}{
			"x":      x,
			"y":      y,
			"width":  width,
			"height": height,
		},
	}
}