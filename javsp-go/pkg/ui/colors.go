package ui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// ANSI color codes
const (
	// Reset
	Reset = "\033[0m"
	
	// Regular colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	
	// Bright colors
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
	
	// Text formatting
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"
)

// ColorConfig holds color configuration
type ColorConfig struct {
	Enabled bool
}

var globalColorConfig = &ColorConfig{
	Enabled: supportsColor(),
}

// SetColorEnabled enables or disables color output
func SetColorEnabled(enabled bool) {
	globalColorConfig.Enabled = enabled
}

// IsColorEnabled returns whether color output is enabled
func IsColorEnabled() bool {
	return globalColorConfig.Enabled
}

// supportsColor detects if the terminal supports color
func supportsColor() bool {
	// Check for explicit disable
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	
	// Check for explicit enable
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	
	// Windows detection
	if runtime.GOOS == "windows" {
		// Check Windows version and terminal
		return os.Getenv("WT_SESSION") != "" || 
			   os.Getenv("ConEmuANSI") == "ON" ||
			   os.Getenv("ANSICON") != ""
	}
	
	// Unix-like systems
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}

// Colorize applies color to text if colors are enabled
func Colorize(text, color string) string {
	if !globalColorConfig.Enabled {
		return text
	}
	return color + text + Reset
}

// Success returns green text
func Success(text string) string {
	return Colorize(text, Green)
}

// Error returns red text
func Error(text string) string {
	return Colorize(text, Red)
}

// Warning returns yellow text
func Warning(text string) string {
	return Colorize(text, Yellow)
}

// Info returns blue text
func Info(text string) string {
	return Colorize(text, Blue)
}

// Highlight returns cyan text
func Highlight(text string) string {
	return Colorize(text, Cyan)
}

// Dim returns dimmed text
func DimText(text string) string {
	return Colorize(text, BrightBlack)
}

// Bold returns bold text
func BoldText(text string) string {
	if !globalColorConfig.Enabled {
		return text
	}
	return Bold + text + Reset
}

// Status icons with colors
func SuccessIcon() string {
	return Success("✅")
}

func ErrorIcon() string {
	return Error("❌")
}

func WarningIcon() string {
	return Warning("⚠️")
}

func InfoIcon() string {
	return Info("ℹ️")
}

func ProcessingIcon() string {
	return Highlight("🔄")
}

func WaitingIcon() string {
	return DimText("⏸️")
}

func RetryIcon() string {
	return Warning("⏳")
}

// Progress bar colors
func ProgressBarFilled(text string) string {
	return Colorize(text, Green)
}

func ProgressBarEmpty(text string) string {
	return Colorize(text, BrightBlack)
}

// Box drawing characters for better visuals
const (
	BoxHorizontal     = "─"
	BoxVertical       = "│"
	BoxTopLeft        = "╭"
	BoxTopRight       = "╮"
	BoxBottomLeft     = "╰"
	BoxBottomRight    = "╯"
	BoxCross          = "┼"
	BoxTeeDown        = "┬"
	BoxTeeUp          = "┴"
	BoxTeeRight       = "├"
	BoxTeeLeft        = "┤"
)

// CreateBox creates a bordered text box
func CreateBox(title, content string, width int) string {
	if width < 10 {
		width = 40
	}
	
	lines := strings.Split(content, "\n")
	var result strings.Builder
	
	// Top border
	result.WriteString(Colorize(BoxTopLeft, BrightBlue))
	if title != "" {
		titleLen := len(stripAnsiCodes(title))
		padding := (width - titleLen - 2) / 2
		result.WriteString(strings.Repeat(BoxHorizontal, padding))
		result.WriteString(Colorize(" "+title+" ", BrightWhite))
		result.WriteString(strings.Repeat(BoxHorizontal, width-padding-titleLen-2))
	} else {
		result.WriteString(Colorize(strings.Repeat(BoxHorizontal, width-2), BrightBlue))
	}
	result.WriteString(Colorize(BoxTopRight, BrightBlue))
	result.WriteString("\n")
	
	// Content lines
	for _, line := range lines {
		result.WriteString(Colorize(BoxVertical, BrightBlue))
		lineLen := len(stripAnsiCodes(line))
		result.WriteString(" " + line)
		if lineLen < width-3 {
			result.WriteString(strings.Repeat(" ", width-3-lineLen))
		}
		result.WriteString(Colorize(BoxVertical, BrightBlue))
		result.WriteString("\n")
	}
	
	// Bottom border
	result.WriteString(Colorize(BoxBottomLeft, BrightBlue))
	result.WriteString(Colorize(strings.Repeat(BoxHorizontal, width-2), BrightBlue))
	result.WriteString(Colorize(BoxBottomRight, BrightBlue))
	
	return result.String()
}

// stripAnsiCodes removes ANSI color codes from text for length calculation
func stripAnsiCodes(text string) string {
	// Simple regex to remove ANSI escape sequences
	result := text
	for strings.Contains(result, "\033[") {
		start := strings.Index(result, "\033[")
		end := strings.Index(result[start:], "m")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	return result
}

// FormatFileSize formats byte size in human readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration in human readable format
func FormatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	} else if seconds < 3600 {
		minutes := int(seconds / 60)
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm%ds", minutes, secs)
	} else {
		hours := int(seconds / 3600)
		minutes := int(seconds/60) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}