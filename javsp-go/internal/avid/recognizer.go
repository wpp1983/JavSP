package avid

import (
	"path/filepath"
	"regexp"
	"strings"

	"javsp-go/internal/config"
)

var (
	// Pre-compile commonly used patterns - simplified CJK detection
	cjkPattern    = regexp.MustCompile(`^[\p{Han}\p{Hiragana}\p{Katakana}\s　！-～]+$`)
	domainPattern = regexp.MustCompile(`(?i)\w{3,10}\.(COM|NET|APP|XYZ)`)
)

// Recognizer handles movie ID recognition from filenames
type Recognizer struct {
	ignoredPatterns []*regexp.Regexp
	patterns        map[string]*regexp.Regexp
}

// NewRecognizer creates a new ID recognizer
func NewRecognizer() *Recognizer {
	r := &Recognizer{
		patterns: make(map[string]*regexp.Regexp),
	}
	r.compilePatterns()
	return r
}

// NewRecognizerWithConfig creates a new recognizer with custom config
func NewRecognizerWithConfig(cfg *config.Config) *Recognizer {
	r := &Recognizer{
		patterns: make(map[string]*regexp.Regexp),
	}
	
	// Compile ignored patterns from config
	for _, pattern := range cfg.Scanner.IgnoredIDPattern {
		if compiled, err := regexp.Compile(pattern); err == nil {
			r.ignoredPatterns = append(r.ignoredPatterns, compiled)
		}
	}
	
	r.compilePatterns()
	return r
}

// compilePatterns compiles all recognition patterns
func (r *Recognizer) compilePatterns() {
	// FC2 pattern: FC2-PPV-1234567 or FC2-1234567
	r.patterns["FC2"] = regexp.MustCompile(`(?i)FC2[^A-Z\d]{0,5}(PPV[^A-Z\d]{0,5})?(\d{5,7})`)
	
	// HEYDOUGA pattern: HEYDOUGA-4017-123 or similar
	r.patterns["HEYDOUGA"] = regexp.MustCompile(`(?i)(HEYDOUGA)[-_]*(\d{4})[-_]0?(\d{3,5})`)
	
	// GETCHU pattern: GETCHU-1234567
	r.patterns["GETCHU"] = regexp.MustCompile(`(?i)GETCHU[-_]*(\d+)`)
	
	// GYUTTO pattern: GYUTTO-123456
	r.patterns["GYUTTO"] = regexp.MustCompile(`(?i)GYUTTO-(\d+)`)
	
	// Special case: 259LUXU (has form of '259luxu')
	r.patterns["259LUXU"] = regexp.MustCompile(`(?i)259LUXU-(\d+)`)
	
	// General JAV patterns
	// Pattern for standard JAV codes like ABC-123, ABCD-123, etc.
	r.patterns["STANDARD"] = regexp.MustCompile(`(?i)([A-Z]{2,10})[-_]?(\d{2,5})`)
	
	// Pattern for numbered series like 300MIUM-001, 123ABC-456
	r.patterns["NUMBERED"] = regexp.MustCompile(`(?i)(\d{3}[A-Z]{2,8})[-_]?(\d{3,5})`)
	
	// Pattern for codes with additional letters like SSIS-123A
	r.patterns["EXTENDED"] = regexp.MustCompile(`(?i)([A-Z]{2,10})[-_]?(\d{2,5})[A-Z]?`)
}

// Recognize extracts movie ID from filename
func (r *Recognizer) Recognize(filename string) string {
	// Get base filename without extension
	base := filepath.Base(filename)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	
	// Apply ignored patterns to clean the filename
	cleaned := r.applyIgnoredPatterns(stem)
	normalized := strings.ToUpper(cleaned)
	
	// Check if filename is pure Chinese/Japanese (should return empty)
	if r.isPureCJK(stem) {
		return ""
	}
	
	// Try special patterns first
	if id := r.trySpecialPatterns(normalized); id != "" {
		return id
	}
	
	// Try domain removal and re-recognition
	if noDomain := r.removeDomains(normalized); noDomain != normalized {
		if id := r.Recognize(noDomain); id != "" {
			return id
		}
	}
	
	// Try general patterns
	return r.tryGeneralPatterns(normalized)
}

// applyIgnoredPatterns removes ignored patterns from filename
func (r *Recognizer) applyIgnoredPatterns(filename string) string {
	result := filename
	for _, pattern := range r.ignoredPatterns {
		result = pattern.ReplaceAllString(result, "")
	}
	return result
}

// isPureCJK checks if the filename contains only Chinese/Japanese characters
func (r *Recognizer) isPureCJK(filename string) bool {
	return cjkPattern.MatchString(filename)
}

// trySpecialPatterns tries special recognition patterns
func (r *Recognizer) trySpecialPatterns(filename string) string {
	// FC2 pattern
	if strings.Contains(filename, "FC2") {
		if match := r.patterns["FC2"].FindStringSubmatch(filename); len(match) >= 3 {
			return "FC2-" + match[2]
		}
	}
	
	// HEYDOUGA pattern
	if strings.Contains(filename, "HEYDOUGA") {
		if match := r.patterns["HEYDOUGA"].FindStringSubmatch(filename); len(match) >= 4 {
			return strings.Join(match[1:], "-")
		}
	}
	
	// GETCHU pattern
	if strings.Contains(filename, "GETCHU") {
		if match := r.patterns["GETCHU"].FindStringSubmatch(filename); len(match) >= 2 {
			return "GETCHU-" + match[1]
		}
	}
	
	// GYUTTO pattern
	if strings.Contains(filename, "GYUTTO") {
		if match := r.patterns["GYUTTO"].FindStringSubmatch(filename); len(match) >= 2 {
			return "GYUTTO-" + match[1]
		}
	}
	
	// 259LUXU special case
	if strings.Contains(filename, "259LUXU") {
		if match := r.patterns["259LUXU"].FindStringSubmatch(filename); len(match) >= 2 {
			return "259LUXU-" + match[1]
		}
	}
	
	return ""
}

// removeDomains removes suspected domain names from filename
func (r *Recognizer) removeDomains(filename string) string {
	return domainPattern.ReplaceAllString(filename, "")
}

// tryGeneralPatterns tries general JAV recognition patterns
func (r *Recognizer) tryGeneralPatterns(filename string) string {
	// Try numbered series pattern first (e.g., 300MIUM-001)
	if match := r.patterns["NUMBERED"].FindStringSubmatch(filename); len(match) >= 3 {
		return match[1] + "-" + match[2]
	}
	
	// Try standard pattern (e.g., ABC-123)
	if match := r.patterns["STANDARD"].FindStringSubmatch(filename); len(match) >= 3 {
		prefix := match[1]
		number := match[2]
		
		// Filter out common false positives
		if r.isValidPrefix(prefix) && r.isValidNumber(number) {
			return prefix + "-" + number
		}
	}
	
	return ""
}

// isValidPrefix checks if the prefix is a valid JAV studio code
func (r *Recognizer) isValidPrefix(prefix string) bool {
	prefix = strings.ToUpper(prefix)
	
	// Common false positives to exclude
	invalidPrefixes := []string{
		"HTTP", "HTTPS", "WWW", "COM", "NET", "ORG", "TV", "JP",
		"DVD", "BD", "CD", "MP", "AVI", "MKV", "MOV", "WMV",
		"X264", "X265", "H264", "H265", "HEVC",
		"HD", "FHD", "UHD", "SD", "HQ", "LQ",
	}
	
	for _, invalid := range invalidPrefixes {
		if prefix == invalid {
			return false
		}
	}
	
	// Must be 2-10 characters and contain only letters
	return len(prefix) >= 2 && len(prefix) <= 10 && regexp.MustCompile(`^[A-Z]+$`).MatchString(prefix)
}

// isValidNumber checks if the number part is reasonable
func (r *Recognizer) isValidNumber(number string) bool {
	// Must be 2-5 digits
	if len(number) < 2 || len(number) > 5 {
		return false
	}
	
	// Must contain only digits
	return regexp.MustCompile(`^\d+$`).MatchString(number)
}

// GetCID converts DVDID to CID format (DMM Content ID)
func GetCID(dvdid string) string {
	if dvdid == "" {
		return ""
	}
	
	// Special cases that don't have CID
	specialPrefixes := []string{"FC2-", "HEYDOUGA-", "GETCHU-", "GYUTTO-"}
	for _, prefix := range specialPrefixes {
		if strings.HasPrefix(dvdid, prefix) {
			return ""
		}
	}
	
	// Convert standard DVDID to CID
	// Example: ABC-123 -> abc00123
	parts := strings.Split(dvdid, "-")
	if len(parts) != 2 {
		return ""
	}
	
	prefix := strings.ToLower(parts[0])
	number := parts[1]
	
	// Pad number to 5 digits
	for len(number) < 5 {
		number = "0" + number
	}
	
	return prefix + number
}

// GuessAVType determines the type of AV based on the ID
func GuessAVType(id string) string {
	if id == "" {
		return "normal"
	}
	
	id = strings.ToUpper(id)
	
	if strings.HasPrefix(id, "FC2-") {
		return "fc2"
	}
	
	if strings.HasPrefix(id, "GETCHU-") {
		return "getchu"
	}
	
	if strings.HasPrefix(id, "GYUTTO-") {
		return "gyutto"
	}
	
	if strings.HasPrefix(id, "HEYDOUGA-") {
		return "normal"
	}
	
	// Check for amateur/素人 series by prefix
	amateurPrefixes := []string{
		"GANA-", "SIRO-", "ARA-", "MIUM-", "HHL-", "DCV-", "LUXU-", "259LUXU-",
		"SWEET-", "CUTE-", "ORE-", "MGS-", "BEAV-", "JKSR-",
	}
	
	for _, prefix := range amateurPrefixes {
		if strings.HasPrefix(id, prefix) {
			return "amateur"
		}
	}
	
	return "normal"
}