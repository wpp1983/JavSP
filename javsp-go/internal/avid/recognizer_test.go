package avid

import (
	"testing"

	"javsp-go/internal/config"
)

func TestRecognizer_Recognize(t *testing.T) {
	r := NewRecognizer()

	tests := []struct {
		name     string
		filename string
		expected string
	}{
		// Standard JAV codes
		{"Standard code", "ABC-123.mp4", "ABC-123"},
		{"Standard with dash", "STAR-999.mkv", "STAR-999"},
		{"Standard with underscore", "SSIS_001.avi", "SSIS-001"},
		{"Standard with brackets", "[JavBus] STARS-123 女优名.mp4", "STARS-123"},
		
		// FC2 patterns
		{"FC2 standard", "FC2-PPV-1234567.mp4", "FC2-1234567"},
		{"FC2 without PPV", "FC2-1234567.mp4", "FC2-1234567"},
		{"FC2 with spaces", "FC2 PPV 1234567.mp4", "FC2-1234567"},
		{"FC2 messy format", "[FC2-PPV] 1234567 title here.mp4", "FC2-1234567"},
		
		// HEYDOUGA patterns
		{"HEYDOUGA standard", "HEYDOUGA-4017-123.mp4", "HEYDOUGA-4017-123"},
		{"HEYDOUGA with underscore", "HEYDOUGA_4017_123.mp4", "HEYDOUGA-4017-123"},
		
		// GETCHU patterns
		{"GETCHU standard", "GETCHU-1234567.mp4", "GETCHU-1234567"},
		{"GETCHU with underscore", "GETCHU_1234567.mp4", "GETCHU-1234567"},
		
		// GYUTTO patterns
		{"GYUTTO standard", "GYUTTO-123456.mp4", "GYUTTO-123456"},
		
		// Special series
		{"259LUXU", "259LUXU-1234.mp4", "259LUXU-1234"},
		{"Numbered series", "300MIUM-001.mp4", "300MIUM-001"},
		
		// Amateur series
		{"GANA series", "GANA-2156.mp4", "GANA-2156"},
		{"SIRO series", "SIRO-4718.mp4", "SIRO-4718"},
		{"ARA series", "261ARA-123.mp4", "261ARA-123"},
		
		// Edge cases
		{"With quality indicator", "ABC-123 1080p.mp4", "ABC-123"},
		{"With brackets and info", "[HD] STAR-123 女優名 Title.mkv", "STAR-123"},
		{"Complex filename", "www.domain.com_STARS-123_女優名_タイトル_1080p.mp4", "STARS-123"},
		
		// Should return empty for invalid cases
		{"Pure Chinese", "纯中文文件名.mp4", ""},
		{"Pure Japanese", "純日本語ファイル名.mp4", ""},
		{"No valid code", "random_video_file.mp4", ""},
		{"Only numbers", "123456789.mp4", ""},
		{"Only letters", "abcdefg.mp4", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Recognize(tt.filename)
			if result != tt.expected {
				t.Errorf("Recognize(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestRecognizer_WithConfig(t *testing.T) {
	cfg := config.GetDefaultConfig()
	cfg.Scanner.IgnoredIDPattern = append(cfg.Scanner.IgnoredIDPattern, `TEST`)
	
	r := NewRecognizerWithConfig(cfg)
	
	// Should ignore TEST pattern
	result := r.Recognize("TEST-ABC-123.mp4")
	expected := "ABC-123"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestGetCID(t *testing.T) {
	tests := []struct {
		dvdid    string
		expected string
	}{
		{"ABC-123", "abc00123"},
		{"STAR-999", "star00999"},
		{"SSIS-001", "ssis00001"},
		{"IPX-177", "ipx00177"},
		{"FC2-1234567", ""}, // FC2 doesn't have CID
		{"HEYDOUGA-4017-123", ""}, // HEYDOUGA doesn't have CID  
		{"", ""},
		{"INVALID", ""},
	}

	for _, tt := range tests {
		t.Run(tt.dvdid, func(t *testing.T) {
			result := GetCID(tt.dvdid)
			if result != tt.expected {
				t.Errorf("GetCID(%q) = %q, expected %q", tt.dvdid, result, tt.expected)
			}
		})
	}
}

func TestGuessAVType(t *testing.T) {
	tests := []struct {
		id       string
		expected string
	}{
		{"FC2-1234567", "fc2"},
		{"GETCHU-123456", "getchu"},
		{"GYUTTO-123456", "gyutto"},
		{"HEYDOUGA-4017-123", "normal"},
		{"GANA-2156", "amateur"},
		{"SIRO-4718", "amateur"},
		{"259LUXU-1234", "amateur"},
		{"STARS-123", "normal"},
		{"ABC-123", "normal"},
		{"", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := GuessAVType(tt.id)
			if result != tt.expected {
				t.Errorf("GuessAVType(%q) = %q, expected %q", tt.id, result, tt.expected)
			}
		})
	}
}

func TestRecognizer_EdgeCases(t *testing.T) {
	r := NewRecognizer()

	// Test files that should not be recognized
	invalidFiles := []string{
		"",
		".mp4",
		"123.mp4",
		"abc.mp4",
		"纯中文.mp4",
		"純日本語.mp4",
		"random_file.mp4",
		"movie.mp4",
		"video.mp4",
	}

	for _, filename := range invalidFiles {
		t.Run("Invalid: "+filename, func(t *testing.T) {
			result := r.Recognize(filename)
			if result != "" {
				t.Errorf("Expected empty result for %q, got %q", filename, result)
			}
		})
	}

	// Test case sensitivity
	caseSensitiveTests := []struct {
		filename string
		expected string
	}{
		{"abc-123.mp4", "ABC-123"},
		{"Star-999.mp4", "STAR-999"},
		{"fc2-ppv-1234567.mp4", "FC2-1234567"},
		{"HEYDOUGA-4017-123.mp4", "HEYDOUGA-4017-123"},
	}

	for _, tt := range caseSensitiveTests {
		t.Run("Case: "+tt.filename, func(t *testing.T) {
			result := r.Recognize(tt.filename)
			if result != tt.expected {
				t.Errorf("Recognize(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkRecognizer_Recognize(b *testing.B) {
	r := NewRecognizer()
	filenames := []string{
		"STARS-123.mp4",
		"FC2-PPV-1234567.mp4",
		"300MIUM-001.mp4",
		"GANA-2156.mp4",
		"[JavBus] SSIS-001 女优名 标题.mkv",
		"www.site.com_IPX-177_actress_title_1080p.mp4",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, filename := range filenames {
			r.Recognize(filename)
		}
	}
}

func BenchmarkRecognizer_SingleFile(b *testing.B) {
	r := NewRecognizer()
	filename := "[JavBus] STARS-123 女优名 标题 1080p.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Recognize(filename)
	}
}