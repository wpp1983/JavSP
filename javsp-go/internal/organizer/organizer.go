package organizer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"javsp-go/internal/datatype"
)

// OrganizerConfig contains configuration for the file organizer
type OrganizerConfig struct {
	OutputDir         string            `json:"output_dir"`
	Pattern           string            `json:"pattern"`           // Naming pattern with placeholders
	CreateDirectories bool              `json:"create_directories"`
	OverwriteExisting bool              `json:"overwrite_existing"`
	BackupOriginal    bool              `json:"backup_original"`
	DryRun            bool              `json:"dry_run"`
	MaxConcurrency    int               `json:"max_concurrency"`
	CustomFields      map[string]string `json:"custom_fields"`
	FileExtensions    []string          `json:"file_extensions"`
}

// DefaultOrganizerConfig returns default configuration
func DefaultOrganizerConfig() *OrganizerConfig {
	return &OrganizerConfig{
		OutputDir:         "./organized",
		Pattern:           "{DVDID} - {Title}",
		CreateDirectories: true,
		OverwriteExisting: false,
		BackupOriginal:    true,
		DryRun:            false,
		MaxConcurrency:    3,
		CustomFields:      make(map[string]string),
		FileExtensions:    []string{".mp4", ".mkv", ".avi", ".wmv", ".mov", ".m4v"},
	}
}

// Operation represents a single file operation
type Operation struct {
	ID          string                `json:"id"`
	Type        OperationType         `json:"type"`
	Source      string                `json:"source"`
	Destination string                `json:"destination"`
	Movie       *datatype.MovieInfo   `json:"movie,omitempty"`
	Status      OperationStatus       `json:"status"`
	Error       error                 `json:"error,omitempty"`
	StartTime   time.Time             `json:"start_time"`
	EndTime     time.Time             `json:"end_time"`
	BackupPath  string                `json:"backup_path,omitempty"`
}

// OperationType represents different operation types
type OperationType int

const (
	OpMove OperationType = iota
	OpCopy
	OpRename
	OpDelete
)

func (o OperationType) String() string {
	switch o {
	case OpMove:
		return "MOVE"
	case OpCopy:
		return "COPY"
	case OpRename:
		return "RENAME"
	case OpDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// OperationStatus represents operation status
type OperationStatus int

const (
	StatusPending OperationStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusRolledBack
)

func (s OperationStatus) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusRunning:
		return "RUNNING"
	case StatusCompleted:
		return "COMPLETED"
	case StatusFailed:
		return "FAILED"
	case StatusRolledBack:
		return "ROLLED_BACK"
	default:
		return "UNKNOWN"
	}
}

// FileOrganizer handles file organization operations
type FileOrganizer struct {
	config     *OrganizerConfig
	operations map[string]*Operation
	mutex      sync.RWMutex
	stats      *OrganizerStats
}

// OrganizerStats tracks organizer statistics
type OrganizerStats struct {
	TotalOperations    int           `json:"total_operations"`
	CompletedOperations int           `json:"completed_operations"`
	FailedOperations   int           `json:"failed_operations"`
	BytesProcessed     int64         `json:"bytes_processed"`
	TotalDuration      time.Duration `json:"total_duration"`
}

// NewFileOrganizer creates a new file organizer
func NewFileOrganizer(config *OrganizerConfig) *FileOrganizer {
	if config == nil {
		config = DefaultOrganizerConfig()
	}

	return &FileOrganizer{
		config:     config,
		operations: make(map[string]*Operation),
		stats:      &OrganizerStats{},
	}
}

// OrganizeMovie organizes a single movie file based on metadata
func (o *FileOrganizer) OrganizeMovie(ctx context.Context, movie *datatype.Movie) (*Operation, error) {
	if movie == nil || !movie.IsValid() {
		return nil, fmt.Errorf("invalid movie data")
	}

	// Generate destination path
	destPath, err := o.generateDestinationPath(movie)
	if err != nil {
		return nil, fmt.Errorf("failed to generate destination path: %w", err)
	}

	// Create operation
	op := &Operation{
		ID:          generateOperationID(),
		Type:        OpMove,
		Source:      movie.FilePath,
		Destination: destPath,
		Movie:       movie.Info,
		Status:      StatusPending,
		StartTime:   time.Now(),
	}

	// Store operation
	o.mutex.Lock()
	o.operations[op.ID] = op
	o.stats.TotalOperations++
	o.mutex.Unlock()

	// Execute operation
	if !o.config.DryRun {
		err = o.executeOperation(ctx, op)
		if err != nil {
			op.Status = StatusFailed
			op.Error = err
			op.EndTime = time.Now()
			o.stats.FailedOperations++
			return op, err
		}
	}

	op.Status = StatusCompleted
	op.EndTime = time.Now()
	o.stats.CompletedOperations++
	o.stats.TotalDuration += op.EndTime.Sub(op.StartTime)

	return op, nil
}

// OrganizeBatch organizes multiple movies concurrently
func (o *FileOrganizer) OrganizeBatch(ctx context.Context, movies []*datatype.Movie) ([]*Operation, error) {
	if len(movies) == 0 {
		return []*Operation{}, nil
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, o.config.MaxConcurrency)
	results := make(chan *Operation, len(movies))
	var wg sync.WaitGroup

	// Start workers
	for _, movie := range movies {
		wg.Add(1)
		go func(m *datatype.Movie) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			op, _ := o.OrganizeMovie(ctx, m)
			results <- op
		}(movie)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var operations []*Operation
	for op := range results {
		if op != nil {
			operations = append(operations, op)
		}
	}

	return operations, nil
}

// executeOperation executes a single operation
func (o *FileOrganizer) executeOperation(ctx context.Context, op *Operation) error {
	op.Status = StatusRunning

	// Check if source exists
	if !fileExists(op.Source) {
		return fmt.Errorf("source file does not exist: %s", op.Source)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(op.Destination)
	if o.config.CreateDirectories {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// Check if destination already exists
	if fileExists(op.Destination) {
		if !o.config.OverwriteExisting {
			return fmt.Errorf("destination file already exists: %s", op.Destination)
		}

		// Create backup if configured
		if o.config.BackupOriginal {
			backupPath := op.Destination + ".backup"
			if err := copyFile(op.Destination, backupPath); err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}
			op.BackupPath = backupPath
		}
	}

	// Execute the operation based on type
	switch op.Type {
	case OpMove:
		return o.moveFile(ctx, op)
	case OpCopy:
		return o.copyFileOp(ctx, op)
	case OpRename:
		return o.renameFile(ctx, op)
	case OpDelete:
		return o.deleteFile(ctx, op)
	default:
		return fmt.Errorf("unsupported operation type: %s", op.Type)
	}
}

// moveFile moves a file from source to destination
func (o *FileOrganizer) moveFile(ctx context.Context, op *Operation) error {
	// Try atomic rename first (works if source and destination are on same filesystem)
	if err := os.Rename(op.Source, op.Destination); err == nil {
		return nil
	}

	// Fall back to copy + delete for cross-filesystem moves
	if err := o.copyFileOp(ctx, op); err != nil {
		return fmt.Errorf("copy failed during move: %w", err)
	}

	// Delete source after successful copy
	if err := os.Remove(op.Source); err != nil {
		// Try to clean up destination file on failure
		os.Remove(op.Destination)
		return fmt.Errorf("failed to remove source after copy: %w", err)
	}

	return nil
}

// copyFileOp copies a file with progress tracking
func (o *FileOrganizer) copyFileOp(ctx context.Context, op *Operation) error {
	source, err := os.Open(op.Source)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer source.Close()

	dest, err := os.Create(op.Destination)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dest.Close()

	// Copy with context cancellation support
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			os.Remove(op.Destination) // Clean up on cancellation
			return ctx.Err()
		default:
		}

		n, err := source.Read(buffer)
		if n > 0 {
			if _, writeErr := dest.Write(buffer[:n]); writeErr != nil {
				os.Remove(op.Destination) // Clean up on error
				return fmt.Errorf("write failed: %w", writeErr)
			}
			o.stats.BytesProcessed += int64(n)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(op.Destination) // Clean up on error
			return fmt.Errorf("read failed: %w", err)
		}
	}

	// Sync to ensure data is written
	return dest.Sync()
}

// renameFile renames a file
func (o *FileOrganizer) renameFile(ctx context.Context, op *Operation) error {
	return os.Rename(op.Source, op.Destination)
}

// deleteFile deletes a file
func (o *FileOrganizer) deleteFile(ctx context.Context, op *Operation) error {
	if o.config.BackupOriginal {
		backupPath := op.Source + ".deleted." + time.Now().Format("20060102_150405")
		if err := os.Rename(op.Source, backupPath); err != nil {
			return fmt.Errorf("failed to move to backup before delete: %w", err)
		}
		op.BackupPath = backupPath
	} else {
		if err := os.Remove(op.Source); err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
	}
	return nil
}

// RollbackOperation rolls back a completed operation
func (o *FileOrganizer) RollbackOperation(ctx context.Context, operationID string) error {
	o.mutex.Lock()
	op, exists := o.operations[operationID]
	o.mutex.Unlock()

	if !exists {
		return fmt.Errorf("operation not found: %s", operationID)
	}

	if op.Status != StatusCompleted {
		return fmt.Errorf("operation is not completed, cannot rollback: %s", op.Status)
	}

	// Execute rollback based on operation type
	switch op.Type {
	case OpMove:
		// Move back from destination to source
		if err := os.Rename(op.Destination, op.Source); err != nil {
			return fmt.Errorf("failed to rollback move: %w", err)
		}
	case OpCopy:
		// Remove the copied file
		if err := os.Remove(op.Destination); err != nil {
			return fmt.Errorf("failed to rollback copy: %w", err)
		}
	case OpDelete:
		// Restore from backup if available
		if op.BackupPath != "" && fileExists(op.BackupPath) {
			if err := os.Rename(op.BackupPath, op.Source); err != nil {
				return fmt.Errorf("failed to rollback delete: %w", err)
			}
		} else {
			return fmt.Errorf("cannot rollback delete: no backup available")
		}
	}

	// Restore backup if it was created during overwrite
	if op.BackupPath != "" && strings.HasSuffix(op.BackupPath, ".backup") {
		if err := os.Rename(op.BackupPath, op.Destination); err != nil {
			return fmt.Errorf("failed to restore backup: %w", err)
		}
	}

	op.Status = StatusRolledBack
	return nil
}

// generateDestinationPath generates the destination path based on config pattern
func (o *FileOrganizer) generateDestinationPath(movie *datatype.Movie) (string, error) {
	if movie.Info == nil {
		return "", fmt.Errorf("movie info is required for path generation")
	}

	// Start with the pattern
	pattern := o.config.Pattern

	// Replace placeholders
	replacements := map[string]string{
		"{DVDID}":         movie.Info.DVDID,
		"{CID}":           movie.Info.CID,
		"{Title}":         movie.Info.Title,
		"{OriginalTitle}": movie.Info.OriginalTitle,
		"{Year}":          movie.Info.GetYear(),
		"{Studio}":        movie.Info.Publisher,
		"{Director}":      movie.Info.Director,
		"{Actress}":       movie.Info.GetActressString(),
		"{Genre}":         movie.Info.GetGenreString(),
		"{Series}":        movie.Info.Series,
	}

	// Add custom fields
	for key, value := range o.config.CustomFields {
		replacements["{"+key+"}"] = value
	}

	// Perform replacements
	result := pattern
	for placeholder, value := range replacements {
		// Clean the value for use in filename
		cleanValue := cleanFilename(value)
		result = strings.ReplaceAll(result, placeholder, cleanValue)
	}

	// Add file extension
	ext := filepath.Ext(movie.FilePath)
	if ext == "" {
		ext = ".mp4" // Default extension
	}

	// Combine with output directory
	fullPath := filepath.Join(o.config.OutputDir, result+ext)

	return fullPath, nil
}

// GetOperation returns an operation by ID
func (o *FileOrganizer) GetOperation(operationID string) (*Operation, bool) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	op, exists := o.operations[operationID]
	return op, exists
}

// GetOperations returns all operations
func (o *FileOrganizer) GetOperations() []*Operation {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	operations := make([]*Operation, 0, len(o.operations))
	for _, op := range o.operations {
		operations = append(operations, op)
	}
	return operations
}

// GetStats returns current organizer statistics
func (o *FileOrganizer) GetStats() *OrganizerStats {
	return &OrganizerStats{
		TotalOperations:     o.stats.TotalOperations,
		CompletedOperations: o.stats.CompletedOperations,
		FailedOperations:    o.stats.FailedOperations,
		BytesProcessed:      o.stats.BytesProcessed,
		TotalDuration:       o.stats.TotalDuration,
	}
}

// ResetStats resets all statistics
func (o *FileOrganizer) ResetStats() {
	o.stats = &OrganizerStats{}
}

// Utility functions

// generateOperationID generates a unique operation ID
func generateOperationID() string {
	return fmt.Sprintf("op_%d", time.Now().UnixNano())
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}

// cleanFilename removes invalid characters from filename
func cleanFilename(filename string) string {
	// Replace invalid characters with safe alternatives
	replacements := map[string]string{
		"/":  "_",
		"\\": "_",
		":":  "-",
		"*":  "_",
		"?":  "_",
		"\"": "'",
		"<":  "(",
		">":  ")",
		"|":  "_",
	}

	result := filename
	for invalid, replacement := range replacements {
		result = strings.ReplaceAll(result, invalid, replacement)
	}

	// Remove leading/trailing whitespace
	result = strings.TrimSpace(result)

	// Limit length to reasonable size
	if len(result) > 200 {
		result = result[:200]
	}

	return result
}