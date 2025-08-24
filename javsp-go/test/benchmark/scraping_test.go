//go:build benchmark

package benchmark

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"javsp-go/internal/avid"
	"javsp-go/internal/config"
	"javsp-go/internal/crawler"
	"javsp-go/pkg/web"
	"javsp-go/test/testutils"
)

func BenchmarkAVIDRecognition(b *testing.B) {
	recognizer := avid.NewRecognizer()
	
	testFilenames := []string{
		"STARS-123.mp4",
		"SSIS-001.mkv",
		"IPX-177.avi",
		"FC2-PPV-1234567.mp4",
		"[JavBus] MIDE-456 女优名.mp4",
		"300MIUM-001.mp4",
		"GANA-2156.mp4",
		"259LUXU-1234.mp4",
		"HEYDOUGA-4017-123.mp4",
		"invalid_file.mp4",
	}
	
	b.ResetTimer()
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, filename := range testFilenames {
				recognizer.Recognize(filename)
			}
		}
	})
	
	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				filename := testFilenames[i%len(testFilenames)]
				recognizer.Recognize(filename)
				i++
			}
		})
	})
}

func BenchmarkAVIDRecognitionComplex(b *testing.B) {
	recognizer := avid.NewRecognizer()
	
	// Generate more complex test cases
	complexFilenames := []string{
		"[JavBus] STARS-123 桃乃木かな タイトル名前 1080p.mp4",
		"www.javbus.com_SSIS-001_深田えいみ_長いタイトル名前_HD.mkv",
		"【高清】IPX-177 天海つばさ スペシャル版 [中文字幕].avi",
		"FC2-PPV-1234567 素人美女 個人撮影 4K.mp4",
		"300MIUM-001 ナンパ企画 美女 ハメ撮り.mp4",
		"GANA-2156 マジ軟派、初撮。 渋谷編.mp4",
		"259LUXU-1234 ラグジュTV プレミアム版.mp4",
		"HEYDOUGA-4017-123 Hey動画オリジナル.mp4",
		"複雜的中文文件名無法識別.mp4",
		"very_long_filename_without_any_recognizable_pattern_that_should_fail.mp4",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, filename := range complexFilenames {
			recognizer.Recognize(filename)
		}
	}
}

func BenchmarkCIDGeneration(b *testing.B) {
	testDVDIDs := []string{
		"STARS-123", "SSIS-001", "IPX-177", "MIDE-456", "PRED-789",
		"ABP-001", "SSNI-123", "PPPD-456", "MEYD-789", "JUL-001",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			dvdid := testDVDIDs[i%len(testDVDIDs)]
			avid.GetCID(dvdid)
			i++
		}
	})
}

func BenchmarkAVTypeGuessing(b *testing.B) {
	testAVIDs := []string{
		"STARS-123", "FC2-1234567", "GANA-2156", "300MIUM-001",
		"GETCHU-123456", "GYUTTO-789", "HEYDOUGA-4017-123",
		"259LUXU-1234", "ARA-456", "SIRO-789",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			avid := testAVIDs[i%len(testAVIDs)]
			avid.GuessAVType(avid)
			i++
		}
	})
}

func BenchmarkMockCrawlerFetch(b *testing.B) {
	// Create mock server
	server := testutils.CreateJavBusTestServer()
	defer server.Close()
	
	// Create mock crawler
	mockCrawler := testutils.NewMockCrawler("benchmark", server.URL())
	generator := testutils.NewTestDataGenerator()
	
	// Generate test data
	testMovies := make([]string, 100)
	for i := 0; i < 100; i++ {
		movieID := fmt.Sprintf("TEST-%03d", i+1)
		testMovies[i] = movieID
		movieInfo := generator.GenerateMovieInfo(movieID)
		mockCrawler.SetMovieData(movieID, movieInfo)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			movieID := testMovies[i%len(testMovies)]
			_, err := mockCrawler.FetchMovieInfo(ctx, movieID)
			if err != nil {
				b.Fatalf("Failed to fetch movie info: %v", err)
			}
		}
	})
	
	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				movieID := testMovies[i%len(testMovies)]
				_, err := mockCrawler.FetchMovieInfo(ctx, movieID)
				if err != nil {
					b.Fatalf("Failed to fetch movie info: %v", err)
				}
				i++
			}
		})
	})
}

func BenchmarkCrawlerRegistry(b *testing.B) {
	// Create multiple mock crawlers
	crawlers := make([]crawler.Crawler, 10)
	for i := 0; i < 10; i++ {
		crawlers[i] = testutils.NewMockCrawler(fmt.Sprintf("crawler-%d", i), fmt.Sprintf("http://test%d.com", i))
	}
	
	b.Run("Registration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			registry := crawler.NewCrawlerRegistry()
			for j, c := range crawlers {
				registry.Register(fmt.Sprintf("crawler-%d", j), c)
			}
		}
	})
	
	b.Run("Lookup", func(b *testing.B) {
		registry := crawler.NewCrawlerRegistry()
		for j, c := range crawlers {
			registry.Register(fmt.Sprintf("crawler-%d", j), c)
		}
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				crawlerName := fmt.Sprintf("crawler-%d", i%len(crawlers))
				registry.Get(crawlerName)
				i++
			}
		})
	})
	
	b.Run("StatsUpdate", func(b *testing.B) {
		registry := crawler.NewCrawlerRegistry()
		for j, c := range crawlers {
			registry.Register(fmt.Sprintf("crawler-%d", j), c)
		}
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				crawlerName := fmt.Sprintf("crawler-%d", i%len(crawlers))
				success := i%2 == 0
				duration := time.Duration(i%100) * time.Millisecond
				registry.UpdateStats(crawlerName, success, duration)
				i++
			}
		})
	})
}

func BenchmarkWebClient(b *testing.B) {
	// Create test server
	server := testutils.NewMockHTTPServer()
	defer server.Close()
	
	// Set up responses
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/test-%d", i)
		server.SetResponse(path, &testutils.MockResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "text/html"},
			Body:       fmt.Sprintf("<html><body>Test response %d</body></html>", i),
		})
	}
	
	client, err := web.NewClient(&web.ClientOptions{
		Timeout:   10 * time.Second,
		RateLimit: 0, // Disable rate limiting for benchmark
	})
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx := context.Background()
	
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			path := fmt.Sprintf("/test-%d", i%100)
			url := server.URL() + path
			
			resp, err := client.Get(ctx, url)
			if err != nil {
				b.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()
		}
	})
	
	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				path := fmt.Sprintf("/test-%d", i%100)
				url := server.URL() + path
				
				resp, err := client.Get(ctx, url)
				if err != nil {
					b.Fatalf("Request failed: %v", err)
				}
				resp.Body.Close()
				i++
			}
		})
	})
}

func BenchmarkHTMLParsing(b *testing.B) {
	// Create complex HTML document
	htmlTemplate := `
<html>
	<head><title>%s</title></head>
	<body>
		<div class="container">
			<h1 class="title">%s</h1>
			<div class="cover">
				<img src="/covers/%s.jpg" alt="Cover">
			</div>
			<div class="info">
				<div class="info-row">
					<span class="label">番号:</span>
					<span class="value">%s</span>
				</div>
				<div class="info-row">
					<span class="label">发行日期:</span>
					<span class="value">2023-12-01</span>
				</div>
				<div class="info-row">
					<span class="label">时长:</span>
					<span class="value">120分钟</span>
				</div>
			</div>
			<div class="actress">
				%s
			</div>
			<div class="genre">
				%s
			</div>
		</div>
	</body>
</html>`
	
	// Generate test HTMLs
	testHTMLs := make([]string, 10)
	for i := 0; i < 10; i++ {
		movieID := fmt.Sprintf("TEST-%03d", i+1)
		title := fmt.Sprintf("测试电影 %d", i+1)
		actresses := ""
		for j := 0; j < 3; j++ {
			actresses += fmt.Sprintf(`<a href="/actress/%d">女优%d</a>`, j+1, j+1)
		}
		genres := ""
		for j := 0; j < 5; j++ {
			genres += fmt.Sprintf(`<a href="/genre/%d">类型%d</a>`, j+1, j+1)
		}
		
		testHTMLs[i] = fmt.Sprintf(htmlTemplate, title, title, movieID, movieID, actresses, genres)
	}
	
	b.Run("ParserCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			html := testHTMLs[i%len(testHTMLs)]
			_, err := web.NewParserFromString(html)
			if err != nil {
				b.Fatalf("Failed to create parser: %v", err)
			}
		}
	})
	
	b.Run("BasicExtraction", func(b *testing.B) {
		html := testHTMLs[0]
		parser, err := web.NewParserFromString(html)
		if err != nil {
			b.Fatalf("Failed to create parser: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			parser.ExtractText(".title")
			parser.ExtractAttr(".cover img", "src")
			parser.ExtractTexts(".actress a")
			parser.ExtractTexts(".genre a")
		}
	})
	
	b.Run("MovieInfoExtraction", func(b *testing.B) {
		html := testHTMLs[0]
		parser, err := web.NewParserFromString(html)
		if err != nil {
			b.Fatalf("Failed to create parser: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			parser.ExtractMovieInfo()
		}
	})
	
	b.Run("ParallelParsing", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				html := testHTMLs[i%len(testHTMLs)]
				parser, err := web.NewParserFromString(html)
				if err != nil {
					b.Fatalf("Failed to create parser: %v", err)
				}
				
				parser.ExtractMovieInfo()
				i++
			}
		})
	})
}

func BenchmarkConcurrentCrawling(b *testing.B) {
	// Create mock server
	server := testutils.CreateJavBusTestServer()
	defer server.Close()
	
	// Create crawler registry
	registry := crawler.NewCrawlerRegistry()
	defer registry.Close()
	
	// Create multiple crawlers
	numCrawlers := 5
	for i := 0; i < numCrawlers; i++ {
		mockCrawler := testutils.NewMockCrawler(fmt.Sprintf("crawler-%d", i), server.URL())
		generator := testutils.NewTestDataGenerator()
		
		// Set up test data
		for j := 0; j < 20; j++ {
			movieID := fmt.Sprintf("TEST-%d-%03d", i, j+1)
			movieInfo := generator.GenerateMovieInfo(movieID)
			mockCrawler.SetMovieData(movieID, movieInfo)
		}
		
		registry.Register(fmt.Sprintf("crawler-%d", i), mockCrawler)
	}
	
	b.Run("ConcurrentFetching", func(b *testing.B) {
		ctx := context.Background()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				crawlerIdx := i % numCrawlers
				movieIdx := (i / numCrawlers) % 20
				
				crawlerName := fmt.Sprintf("crawler-%d", crawlerIdx)
				movieID := fmt.Sprintf("TEST-%d-%03d", crawlerIdx, movieIdx+1)
				
				if crawler, exists := registry.Get(crawlerName); exists {
					_, err := crawler.FetchMovieInfo(ctx, movieID)
					if err != nil {
						b.Fatalf("Failed to fetch movie info: %v", err)
					}
				}
				i++
			}
		})
	})
	
	b.Run("StressTest", func(b *testing.B) {
		ctx := context.Background()
		var wg sync.WaitGroup
		errorChan := make(chan error, b.N)
		
		b.ResetTimer()
		
		// Launch many goroutines
		for i := 0; i < b.N; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				crawlerIdx := idx % numCrawlers
				movieIdx := (idx / numCrawlers) % 20
				
				crawlerName := fmt.Sprintf("crawler-%d", crawlerIdx)
				movieID := fmt.Sprintf("TEST-%d-%03d", crawlerIdx, movieIdx+1)
				
				if crawler, exists := registry.Get(crawlerName); exists {
					_, err := crawler.FetchMovieInfo(ctx, movieID)
					if err != nil {
						errorChan <- err
						return
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errorChan)
		
		// Check for errors
		for err := range errorChan {
			if err != nil {
				b.Fatalf("Stress test failed: %v", err)
			}
		}
	})
}

func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("LargeDataStructures", func(b *testing.B) {
		generator := testutils.NewTestDataGenerator()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create large datasets
			movies := generator.CreateTestMovies(1000)
			
			// Process them
			for _, movie := range movies {
				_ = movie.DVDID
				_ = movie.Info.Title
				_ = movie.Info.Actress
				_ = movie.Info.Genre
			}
			
			// Let GC clean up
			movies = nil
		}
	})
	
	b.Run("CrawlerRegistry", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry := crawler.NewCrawlerRegistry()
			
			// Register many crawlers
			for j := 0; j < 100; j++ {
				crawler := testutils.NewMockCrawler(fmt.Sprintf("crawler-%d", j), fmt.Sprintf("http://test%d.com", j))
				registry.Register(fmt.Sprintf("crawler-%d", j), crawler)
			}
			
			// Use the registry
			for j := 0; j < 100; j++ {
				registry.Get(fmt.Sprintf("crawler-%d", j))
				registry.UpdateStats(fmt.Sprintf("crawler-%d", j), true, time.Millisecond)
			}
			
			registry.Close()
		}
	})
}