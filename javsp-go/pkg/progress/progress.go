package progress

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProgressCallback is a function type for progress updates
type ProgressCallback func(message string, progress float64, remaining time.Duration)

// ProgressTracker tracks progress with timeout awareness
type ProgressTracker struct {
	startTime   time.Time
	timeout     time.Duration
	message     string
	callback    ProgressCallback
	mutex       sync.RWMutex
	done        bool
	ctx         context.Context
	cancel      context.CancelFunc
	lastUpdate  time.Time
	updateInterval time.Duration
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(message string, timeout time.Duration, callback ProgressCallback) *ProgressTracker {
	ctx, cancel := context.WithCancel(context.Background())
	
	tracker := &ProgressTracker{
		startTime:      time.Now(),
		timeout:        timeout,
		message:        message,
		callback:       callback,
		ctx:            ctx,
		cancel:         cancel,
		updateInterval: 500 * time.Millisecond, // Update every 500ms
	}
	
	// Start progress reporting goroutine
	go tracker.reportProgress()
	
	return tracker
}

// reportProgress runs in a separate goroutine to report progress
func (p *ProgressTracker) reportProgress() {
	ticker := time.NewTicker(p.updateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.updateProgress()
		}
	}
}

// updateProgress calculates and reports current progress
func (p *ProgressTracker) updateProgress() {
	p.mutex.RLock()
	if p.done {
		p.mutex.RUnlock()
		return
	}
	
	elapsed := time.Since(p.startTime)
	remaining := p.timeout - elapsed
	progress := float64(elapsed) / float64(p.timeout)
	
	if progress > 1.0 {
		progress = 1.0
		remaining = 0
	}
	
	p.mutex.RUnlock()
	
	if p.callback != nil && time.Since(p.lastUpdate) >= p.updateInterval {
		p.callback(p.message, progress, remaining)
		p.lastUpdate = time.Now()
	}
}

// UpdateMessage updates the progress message
func (p *ProgressTracker) UpdateMessage(message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.message = message
}

// Done marks the progress as complete
func (p *ProgressTracker) Done() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.done {
		p.done = true
		p.cancel()
		
		if p.callback != nil {
			p.callback(p.message, 1.0, 0)
		}
	}
}

// Fail marks the progress as failed
func (p *ProgressTracker) Fail(err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.done {
		p.done = true
		p.cancel()
		
		if p.callback != nil {
			failMsg := fmt.Sprintf("%s - Failed: %v", p.message, err)
			p.callback(failMsg, 1.0, 0)
		}
	}
}

// IsExpired returns true if the timeout has been exceeded
func (p *ProgressTracker) IsExpired() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	return !p.done && time.Since(p.startTime) > p.timeout
}

// GetElapsed returns the elapsed time
func (p *ProgressTracker) GetElapsed() time.Duration {
	return time.Since(p.startTime)
}

// GetRemaining returns the remaining time
func (p *ProgressTracker) GetRemaining() time.Duration {
	elapsed := time.Since(p.startTime)
	remaining := p.timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CountdownDisplay provides a countdown display utility
type CountdownDisplay struct {
	start    time.Time
	duration time.Duration
	message  string
	done     chan struct{}
	mu       sync.RWMutex
}

// NewCountdownDisplay creates a new countdown display
func NewCountdownDisplay(message string, duration time.Duration) *CountdownDisplay {
	return &CountdownDisplay{
		start:    time.Now(),
		duration: duration,
		message:  message,
		done:     make(chan struct{}),
	}
}

// Start begins the countdown display
func (c *CountdownDisplay) Start() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				c.updateDisplay()
			}
		}
	}()
}

// updateDisplay updates the countdown display
func (c *CountdownDisplay) updateDisplay() {
	c.mu.RLock()
	elapsed := time.Since(c.start)
	remaining := c.duration - elapsed
	
	if remaining <= 0 {
		c.mu.RUnlock()
		return
	}
	
	progress := float64(elapsed) / float64(c.duration)
	progressBar := createProgressBar(progress, 20)
	
	fmt.Printf("\r%s %s [%.1fs remaining]", c.message, progressBar, remaining.Seconds())
	c.mu.RUnlock()
}

// Stop stops the countdown display
func (c *CountdownDisplay) Stop() {
	close(c.done)
	fmt.Print("\r") // Clear the line
}

// createProgressBar creates a visual progress bar with enhanced characters
func createProgressBar(progress float64, width int) string {
	if progress > 1.0 {
		progress = 1.0
	}
	
	if progress < 0 {
		progress = 0
	}
	
	// More precise progress calculation
	totalChars := float64(width)
	filledFloat := progress * totalChars
	filledFull := int(filledFloat)
	remainder := filledFloat - float64(filledFull)
	
	// Enhanced progress bar characters
	fullChar := "█"
	partialChars := []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	emptyChar := "░"
	
	var bar strings.Builder
	
	// Add full characters
	for i := 0; i < filledFull; i++ {
		bar.WriteString(fullChar)
	}
	
	// Add partial character if needed
	if filledFull < width && remainder > 0 {
		partialIndex := int(remainder * float64(len(partialChars)))
		if partialIndex >= len(partialChars) {
			partialIndex = len(partialChars) - 1
		}
		bar.WriteString(partialChars[partialIndex])
		filledFull++
	}
	
	// Add empty characters
	for i := filledFull; i < width; i++ {
		bar.WriteString(emptyChar)
	}
	
	return fmt.Sprintf("[%s] %.1f%%", bar.String(), progress*100)
}

// FormatDuration formats a duration into a human-readable string
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
}

// SpinnerDisplay provides a spinning indicator
type SpinnerDisplay struct {
	message  string
	done     chan struct{}
	spinner  []string
	mu       sync.RWMutex
}

// NewSpinnerDisplay creates a new spinner display
func NewSpinnerDisplay(message string) *SpinnerDisplay {
	return &SpinnerDisplay{
		message: message,
		done:    make(chan struct{}),
		spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Start begins the spinner display
func (s *SpinnerDisplay) Start() {
	go func() {
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				s.mu.RLock()
				fmt.Printf("\r%s %s", s.spinner[i%len(s.spinner)], s.message)
				s.mu.RUnlock()
				i++
			}
		}
	}()
}

// UpdateMessage updates the spinner message
func (s *SpinnerDisplay) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

// Stop stops the spinner display
func (s *SpinnerDisplay) Stop() {
	close(s.done)
	fmt.Print("\r") // Clear the line
}