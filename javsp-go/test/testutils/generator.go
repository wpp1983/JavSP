package testutils

import (
	"fmt"
	"math/rand"
	"time"

	"javsp-go/internal/datatype"
)

// TestDataGenerator generates test data for various scenarios
type TestDataGenerator struct {
	rand *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateMovieInfo generates a mock MovieInfo with random data
func (g *TestDataGenerator) GenerateMovieInfo(dvdid string) *datatype.MovieInfo {
	actresses := [][]string{
		{"桃乃木かな", "深田えいみ"},
		{"三上悠亜", "明里つむぎ"},
		{"辻本杏", "白石茉莉奈"},
		{"JULIA", "蓮実クレア"},
	}

	genres := [][]string{
		{"巨乳", "单体作品", "中出", "乳交"},
		{"熟女", "人妻", "中出", "口交"},
		{"美少女", "学生", "制服", "口交"},
		{"OL", "丝袜", "中出", "3P"},
	}

	publishers := []string{
		"S1 NO.1 STYLE",
		"MOODYZ",
		"IdeaPocket",
		"Premium",
		"アタッカーズ",
	}

	actressIdx := g.rand.Intn(len(actresses))
	genreIdx := g.rand.Intn(len(genres))
	publisherIdx := g.rand.Intn(len(publishers))

	releaseDate := time.Now().AddDate(0, -g.rand.Intn(36), -g.rand.Intn(30))

	return &datatype.MovieInfo{
		DVDID:       dvdid,
		Title:       fmt.Sprintf("测试电影 %s", dvdid),
		Plot:        fmt.Sprintf("这是电影 %s 的剧情简介。", dvdid),
		ReleaseDate: releaseDate.Format("2006-01-02"),
		Year:        releaseDate.Format("2006"),
		Runtime:     fmt.Sprintf("%d", 100+g.rand.Intn(80)),
		Director:    "测试导演",
		Publisher:   publishers[publisherIdx],
		Actress:     actresses[actressIdx],
		Genre:       genres[genreIdx],
		Rating:      float64(g.rand.Intn(50)+50) / 10.0,
		Votes:       g.rand.Intn(1000) + 100,
		Cover:       fmt.Sprintf("https://example.com/covers/%s.jpg", dvdid),
		Fanart:      fmt.Sprintf("https://example.com/fanart/%s.jpg", dvdid),
		ExtraFanarts: []string{
			fmt.Sprintf("https://example.com/fanart/%s-1.jpg", dvdid),
			fmt.Sprintf("https://example.com/fanart/%s-2.jpg", dvdid),
		},
		CreatedAt:   releaseDate,
		UpdatedAt:   releaseDate,
	}
}

// GenerateMovie generates a mock Movie with file information
func (g *TestDataGenerator) GenerateMovie(filePath string, dvdid string) *datatype.Movie {
	return &datatype.Movie{
		FilePath: filePath,
		FileName: extractFileName(filePath),
		FileSize: int64(g.rand.Intn(5000)+1000) * 1024 * 1024, // 1GB - 6GB
		DVDID:    dvdid,
		Info:     g.GenerateMovieInfo(dvdid),
	}
}

// GenerateDVDIDs generates a list of realistic DVD IDs
func (g *TestDataGenerator) GenerateDVDIDs(count int) []string {
	prefixes := []string{
		"STARS", "SSIS", "IPX", "MIDE", "MIAD", "PRED", "ABP",
		"SSNI", "PPPD", "MEYD", "JUL", "CAWD", "MIFD", "WANZ",
	}

	dvdids := make([]string, count)
	for i := 0; i < count; i++ {
		prefix := prefixes[g.rand.Intn(len(prefixes))]
		number := g.rand.Intn(999) + 1
		dvdids[i] = fmt.Sprintf("%s-%03d", prefix, number)
	}
	return dvdids
}

// GenerateFC2IDs generates FC2 style IDs
func (g *TestDataGenerator) GenerateFC2IDs(count int) []string {
	fc2ids := make([]string, count)
	for i := 0; i < count; i++ {
		number := g.rand.Intn(9000000) + 1000000
		fc2ids[i] = fmt.Sprintf("FC2-%d", number)
	}
	return fc2ids
}

// GenerateFilePaths generates realistic file paths
func (g *TestDataGenerator) GenerateFilePaths(dvdids []string) []string {
	extensions := []string{".mp4", ".mkv", ".avi", ".wmv"}
	paths := make([]string, len(dvdids))

	for i, dvdid := range dvdids {
		ext := extensions[g.rand.Intn(len(extensions))]
		
		// Generate different path formats
		formats := []string{
			"/test/videos/%s%s",
			"/test/videos/[JavBus] %s 女优名%s",
			"/test/videos/%s 1080p%s",
			"/test/videos/www.site.com_%s_title%s",
		}
		
		format := formats[g.rand.Intn(len(formats))]
		paths[i] = fmt.Sprintf(format, dvdid, ext)
	}
	return paths
}

// CreateTestMovies creates a batch of test movies
func (g *TestDataGenerator) CreateTestMovies(count int) []*datatype.Movie {
	dvdids := g.GenerateDVDIDs(count)
	paths := g.GenerateFilePaths(dvdids)
	
	movies := make([]*datatype.Movie, count)
	for i := 0; i < count; i++ {
		movies[i] = g.GenerateMovie(paths[i], dvdids[i])
	}
	return movies
}

// Helper function to extract file name from path
func extractFileName(filePath string) string {
	// Simple extraction for testing
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' || filePath[i] == '\\' {
			return filePath[i+1:]
		}
	}
	return filePath
}

// GenerateConfig generates test configuration
func (g *TestDataGenerator) GenerateConfig() map[string]interface{} {
	return map[string]interface{}{
		"scanner": map[string]interface{}{
			"input_directory":     "/test/input",
			"minimum_size":        "100MiB",
			"filename_extensions": []string{".mp4", ".mkv", ".avi"},
		},
		"network": map[string]interface{}{
			"retry":   3,
			"timeout": "30s",
		},
		"crawler": map[string]interface{}{
			"hardworking": true,
			"selection": map[string]interface{}{
				"normal": []string{"javbus", "avwiki"},
			},
		},
		"other": map[string]interface{}{
			"log_level":   "DEBUG",
			"interactive": false,
		},
	}
}