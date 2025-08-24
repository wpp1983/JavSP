package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MovieProgressDisplay manages the display of movie processing progress
type MovieProgressDisplay struct {
	totalMovies     int
	currentIndex    int
	successCount    int
	failedCount     int
	currentMovie    string
	currentStep     string
	crawlerStatus   map[string]CrawlerStatus
	startTime       time.Time
	mutex           sync.RWMutex
	lastUpdate      time.Time
	updateInterval  time.Duration
}

// CrawlerStatus represents the status of a single crawler
type CrawlerStatus struct {
	Name      string
	Status    string // "waiting", "connecting", "retrying", "success", "failed"
	Progress  float64
	Message   string
	Duration  time.Duration
	Attempt   int
	MaxAttempts int
}

// NewMovieProgressDisplay creates a new movie progress display
func NewMovieProgressDisplay(totalMovies int) *MovieProgressDisplay {
	return &MovieProgressDisplay{
		totalMovies:    totalMovies,
		currentIndex:   0,
		crawlerStatus:  make(map[string]CrawlerStatus),
		startTime:      time.Now(),
		updateInterval: 200 * time.Millisecond,
	}
}

// StartMovie begins processing a new movie
func (d *MovieProgressDisplay) StartMovie(index int, movieName string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.currentIndex = index
	d.currentMovie = movieName
	d.currentStep = "Initializing..."
	d.crawlerStatus = make(map[string]CrawlerStatus) // Clear previous status
}

// UpdateStep updates the current processing step
func (d *MovieProgressDisplay) UpdateStep(step string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.currentStep = step
}

// UpdateCrawler updates the status of a specific crawler
func (d *MovieProgressDisplay) UpdateCrawler(name, status, message string, progress float64, duration time.Duration, attempt, maxAttempts int) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.crawlerStatus[name] = CrawlerStatus{
		Name:        name,
		Status:      status,
		Progress:    progress,
		Message:     message,
		Duration:    duration,
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
	}
}

// FinishMovie marks a movie as completed (success or failure)
func (d *MovieProgressDisplay) FinishMovie(success bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if success {
		d.successCount++
	} else {
		d.failedCount++
	}
}

// Render renders the current progress display
func (d *MovieProgressDisplay) Render() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	var output strings.Builder
	
	// Overall progress
	overallProgress := float64(d.currentIndex-1+d.successCount+d.failedCount) / float64(d.totalMovies)
	if overallProgress > 1.0 {
		overallProgress = 1.0
	}
	
	progressBar := d.createColoredProgressBar(overallProgress, 20)
	output.WriteString(fmt.Sprintf("[%d/%d] %s %s\n", 
		d.currentIndex, d.totalMovies, progressBar, BoldText(d.currentMovie)))
	
	// Current step
	if d.currentStep != "" {
		output.WriteString(fmt.Sprintf("\n%s %s\n", 
			ProcessingIcon(), Highlight(d.currentStep)))
	}
	
	// Crawler statuses
	if len(d.crawlerStatus) > 0 {
		output.WriteString(d.renderCrawlerStatus())
	}
	
	// Statistics
	output.WriteString(d.renderStats())
	
	return output.String()
}

// createColoredProgressBar creates a colored progress bar
func (d *MovieProgressDisplay) createColoredProgressBar(progress float64, width int) string {
	if progress > 1.0 {
		progress = 1.0
	}
	if progress < 0 {
		progress = 0
	}
	
	filled := int(progress * float64(width))
	remaining := width - filled
	
	filledBar := ProgressBarFilled(strings.Repeat("█", filled))
	emptyBar := ProgressBarEmpty(strings.Repeat("░", remaining))
	
	percentage := BoldText(fmt.Sprintf("%.1f%%", progress*100))
	
	return fmt.Sprintf("%s%s %s", filledBar, emptyBar, percentage)
}

// renderCrawlerStatus renders the status of all crawlers
func (d *MovieProgressDisplay) renderCrawlerStatus() string {
	var output strings.Builder
	
	output.WriteString(Colorize("├─ Crawler Status\n", BrightBlue))
	
	for _, status := range d.crawlerStatus {
		icon := d.getStatusIcon(status.Status)
		
		var statusText string
		switch status.Status {
		case "waiting":
			statusText = DimText("Waiting...")
		case "connecting":
			statusText = fmt.Sprintf("Connecting... %s", 
				d.createMiniProgressBar(status.Progress, 10))
		case "retrying":
			statusText = fmt.Sprintf("Retrying (attempt %d/%d) %s", 
				status.Attempt, status.MaxAttempts,
				d.createMiniProgressBar(status.Progress, 10))
		case "success":
			statusText = Success(fmt.Sprintf("Success (%s)", 
				FormatDuration(status.Duration.Seconds())))
		case "failed":
			statusText = Error("Failed")
		default:
			statusText = status.Message
		}
		
		output.WriteString(fmt.Sprintf("├─ %-8s %s %s\n", 
			BoldText(status.Name), icon, statusText))
	}
	
	return output.String()
}

// renderStats renders session statistics
func (d *MovieProgressDisplay) renderStats() string {
	elapsed := time.Since(d.startTime)
	processed := d.successCount + d.failedCount
	
	var etaText string
	if processed > 0 && d.currentIndex < d.totalMovies {
		avgTimePerMovie := elapsed / time.Duration(processed)
		remaining := d.totalMovies - processed
		eta := avgTimePerMovie * time.Duration(remaining)
		etaText = fmt.Sprintf(" | ETA: %s", DimText(FormatDuration(eta.Seconds())))
	}
	
	successRate := 0.0
	if processed > 0 {
		successRate = float64(d.successCount) / float64(processed) * 100
	}
	
	stats := fmt.Sprintf("%s Stats: %s%d %s%d%s",
		Info("📊"), 
		SuccessIcon(), d.successCount,
		ErrorIcon(), d.failedCount,
		etaText)
	
	if processed > 0 {
		stats += fmt.Sprintf(" | Success Rate: %s%.1f%%",
			d.getSuccessRateColor(successRate), successRate)
	}
	
	return fmt.Sprintf("└─ %s\n", stats)
}

// getStatusIcon returns the appropriate icon for a crawler status
func (d *MovieProgressDisplay) getStatusIcon(status string) string {
	switch status {
	case "waiting":
		return WaitingIcon()
	case "connecting":
		return ProcessingIcon()
	case "retrying":
		return RetryIcon()
	case "success":
		return SuccessIcon()
	case "failed":
		return ErrorIcon()
	default:
		return InfoIcon()
	}
}

// createMiniProgressBar creates a small progress bar for individual crawlers
func (d *MovieProgressDisplay) createMiniProgressBar(progress float64, width int) string {
	if width < 5 {
		width = 10
	}
	
	if progress > 1.0 {
		progress = 1.0
	}
	if progress < 0 {
		progress = 0
	}
	
	filled := int(progress * float64(width))
	remaining := width - filled
	
	filledPart := strings.Repeat("█", filled)
	emptyPart := strings.Repeat("░", remaining)
	
	return fmt.Sprintf("[%s%s]", 
		ProgressBarFilled(filledPart), 
		ProgressBarEmpty(emptyPart))
}

// getSuccessRateColor returns appropriate color for success rate
func (d *MovieProgressDisplay) getSuccessRateColor(rate float64) string {
	if rate >= 80 {
		return Green
	} else if rate >= 60 {
		return Yellow
	} else {
		return Red
	}
}

// Clear clears the display area
func (d *MovieProgressDisplay) Clear() {
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top
}

// MoveCursorUp moves cursor up by specified lines
func (d *MovieProgressDisplay) MoveCursorUp(lines int) {
	fmt.Printf("\033[%dA", lines)
}

// ClearLine clears the current line
func (d *MovieProgressDisplay) ClearLine() {
	fmt.Print("\033[2K\r")
}

// FinalSummary creates a beautiful final summary
func (d *MovieProgressDisplay) FinalSummary(crawlerStats, downloadStats, organizerStats map[string]interface{}) string {
	totalTime := time.Since(d.startTime)
	totalProcessed := d.successCount + d.failedCount
	
	// Create summary content
	content := fmt.Sprintf("Total Movies: %d\n", d.totalMovies)
	content += fmt.Sprintf("%s Success: %d (%.1f%%)\n", 
		SuccessIcon(), d.successCount, 
		float64(d.successCount)/float64(d.totalMovies)*100)
	content += fmt.Sprintf("%s Failed: %d (%.1f%%)\n", 
		ErrorIcon(), d.failedCount,
		float64(d.failedCount)/float64(d.totalMovies)*100)
	content += fmt.Sprintf("⏱️ Total Time: %s\n", FormatDuration(totalTime.Seconds()))
	
	if totalProcessed > 0 {
		avgTime := totalTime / time.Duration(totalProcessed)
		content += fmt.Sprintf("📊 Avg per Movie: %s", FormatDuration(avgTime.Seconds()))
	}
	
	summary := CreateBox(Success("🎉 Processing Complete!"), content, 35)
	
	// Add detailed stats
	var statsContent strings.Builder
	
	if crawlerStats != nil {
		statsContent.WriteString(fmt.Sprintf("🌐 Network Activity\n"))
		if total, ok := crawlerStats["total_requests"].(int64); ok {
			success, _ := crawlerStats["successful_crawls"].(int64)
			failed, _ := crawlerStats["failed_crawls"].(int64)
			statsContent.WriteString(fmt.Sprintf("├─ Requests: %d (success: %d, failed: %d)\n", 
				total, success, failed))
			if total > 0 {
				successRate := float64(success) / float64(total) * 100
				statsContent.WriteString(fmt.Sprintf("└─ Success Rate: %.1f%%\n\n", successRate))
			}
		}
	}
	
	if downloadStats != nil {
		statsContent.WriteString(fmt.Sprintf("📥 Image Downloads\n"))
		if total, ok := downloadStats["total_downloads"].(int); ok {
			success, _ := downloadStats["successful_downloads"].(int)
			failed, _ := downloadStats["failed_downloads"].(int)
			skipped, _ := downloadStats["skipped_downloads"].(int)
			statsContent.WriteString(fmt.Sprintf("├─ Total: %d (success: %d, failed: %d, skipped: %d)\n", 
				total, success, failed, skipped))
			if bytes, ok := downloadStats["bytes_downloaded"].(int64); ok {
				statsContent.WriteString(fmt.Sprintf("└─ Downloaded: %s\n\n", FormatFileSize(bytes)))
			}
		}
	}
	
	if organizerStats != nil {
		statsContent.WriteString(fmt.Sprintf("📁 File Operations\n"))
		if total, ok := organizerStats["total_operations"].(int); ok {
			completed, _ := organizerStats["completed_operations"].(int)
			failed, _ := organizerStats["failed_operations"].(int)
			statsContent.WriteString(fmt.Sprintf("└─ Total: %d (completed: %d, failed: %d)", 
				total, completed, failed))
		}
	}
	
	if statsContent.Len() > 0 {
		summary += "\n\n" + CreateBox("📈 Detailed Statistics", statsContent.String(), 50)
	}
	
	return summary
}